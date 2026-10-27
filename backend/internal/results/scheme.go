// Package results is marks, approval, release and transcripts (brief 7,
// Stage 13a, AC-09). Each class (offering) has a result sheet:
//
//	draft --submit (trainer)--> submitted --approve (Head of School)--> approved --release (registrar)--> released
//	submitted --return with a reason--> draft
//	approved or released --correction by the trainer, with a reason--> draft
//
// Students see only released results, as frozen at each release; a
// correction is invisible to them until it is approved and released again.
// Marks, sheet events and releases are append-only.
package results

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
)

// Component is one weighted part of a unit's assessment.
type Component struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// Band is the lowest total that earns a grade.
type Band struct {
	Grade string `json:"grade"`
	Min   int    `json:"min"`
}

// Scheme is how a programme assesses its units (DQ-07).
type Scheme struct {
	Kind       string      `json:"kind"` // marks or competency
	Components []Component `json:"components"`
	PassMark   int         `json:"pass_mark"`
	Bands      []Band      `json:"grade_bands"`
	// IsDefault: the college has not set this programme's scheme yet.
	IsDefault bool `json:"is_default"`
}

// CompetencyComponent is the single "component" of a competency unit.
const CompetencyComponent = "competency"

// Default applies until a programme's scheme is set: marks, CAT 30% and
// exam 70%, pass mark 40, bands A 70, B 60, C 50, D 40, E below. These are
// placeholders for the college to confirm (A-35), not college policy.
func Default() Scheme {
	return Scheme{
		Kind: "marks",
		Components: []Component{
			{Key: "cat", Name: "Continuous assessment", Weight: 30},
			{Key: "exam", Name: "Final exam", Weight: 70},
		},
		PassMark:  40,
		Bands:     []Band{{"A", 70}, {"B", 60}, {"C", 50}, {"D", 40}, {"E", 0}},
		IsDefault: true,
	}
}

var (
	componentKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,29}$`)
	notKeyChars  = regexp.MustCompile(`[^a-z0-9]+`)
)

// Normalise checks a scheme and fills what follows from its kind.
func (s *Scheme) Normalise() error {
	var p content.Problems
	s.IsDefault = false
	switch s.Kind {
	case "competency":
		s.Components = []Component{{Key: CompetencyComponent, Name: "Competency", Weight: 0}}
		s.PassMark, s.Bands = 0, []Band{}
		return nil
	case "marks":
	default:
		return content.Problems{{Field: "kind", Message: "Choose marks or competency."}}.Err()
	}
	if len(s.Components) == 0 || len(s.Components) > 6 {
		p.Add("components", "Use 1 to 6 parts, for example continuous assessment and a final exam.")
	}
	total, seen := 0, map[string]bool{}
	for i := range s.Components {
		c := &s.Components[i]
		c.Name = strings.TrimSpace(c.Name)
		if c.Key == "" {
			c.Key = strings.Trim(notKeyChars.ReplaceAllString(strings.ToLower(c.Name), "_"), "_")
		}
		if !componentKey.MatchString(c.Key) || seen[c.Key] || c.Key == CompetencyComponent {
			p.Add("components", fmt.Sprintf("Give each part a different name (%q is used twice or not allowed).", c.Name))
			break
		}
		seen[c.Key] = true
		if c.Name == "" || len([]rune(c.Name)) > 60 {
			p.Add("components", "Name each part in 1 to 60 characters.")
			break
		}
		if c.Weight < 1 || c.Weight > 100 {
			p.Add("components", "Each part's weight is a whole number from 1 to 100.")
			break
		}
		total += c.Weight
	}
	if total != 100 && len(p) == 0 {
		p.Add("components", fmt.Sprintf("The weights add up to %d. They must add up to 100.", total))
	}
	if s.PassMark < 0 || s.PassMark > 100 {
		p.Add("pass_mark", "The pass mark is a whole number from 0 to 100.")
	}
	if len(s.Bands) == 0 || len(s.Bands) > 12 {
		p.Add("grade_bands", "Use 1 to 12 grades.")
	} else {
		for i, b := range s.Bands {
			s.Bands[i].Grade = strings.TrimSpace(b.Grade)
			if s.Bands[i].Grade == "" || len([]rune(s.Bands[i].Grade)) > 3 {
				p.Add("grade_bands", "Each grade is 1 to 3 characters, like A or B+.")
				break
			}
			if i > 0 && b.Min >= s.Bands[i-1].Min {
				p.Add("grade_bands", "List grades from highest to lowest, each starting lower than the one before.")
				break
			}
		}
		if last := s.Bands[len(s.Bands)-1]; last.Min != 0 && len(p) == 0 {
			p.Add("grade_bands", "The lowest grade must start at 0, so every total has a grade.")
		}
	}
	return p.Err()
}

// Mark is one current entry.
type Mark struct {
	Score   *int    `json:"score"`
	Outcome *string `json:"outcome"`
}

// Result is a student's result for a unit under a scheme.
type Result struct {
	// Complete: every part has a mark (else the rest is empty).
	Complete bool    `json:"complete"`
	Total    *int    `json:"total"`
	Grade    string  `json:"grade"`
	Passed   bool    `json:"passed"`
	Outcome  *string `json:"outcome"`
}

// Compute works out a result in whole numbers only: the weighted total is
// rounded half up, (sum of score x weight + 50) / 100.
func Compute(s Scheme, marks map[string]Mark) Result {
	if s.Kind == "competency" {
		m, ok := marks[CompetencyComponent]
		if !ok || m.Outcome == nil {
			return Result{}
		}
		return Result{Complete: true, Outcome: m.Outcome, Passed: *m.Outcome == "competent"}
	}
	sum := 0
	for _, c := range s.Components {
		m, ok := marks[c.Key]
		if !ok || m.Score == nil {
			return Result{}
		}
		sum += *m.Score * c.Weight
	}
	total := (sum + 50) / 100
	r := Result{Complete: true, Total: &total, Passed: total >= s.PassMark}
	for _, b := range s.Bands {
		if total >= b.Min {
			r.Grade = b.Grade
			break
		}
	}
	return r
}
