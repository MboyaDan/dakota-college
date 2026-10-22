package content

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Problems collects field errors so the editor sees every fix needed at once.
type Problems []FieldProblem

type FieldProblem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (p *Problems) add(field, format string, args ...any) {
	*p = append(*p, FieldProblem{Field: field, Message: fmt.Sprintf(format, args...)})
}

func (p Problems) Err() error {
	if len(p) == 0 {
		return nil
	}
	return &ValidationError{Problems: p}
}

// ValidationError lists what to fix; the API returns it as 422.
type ValidationError struct{ Problems Problems }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Field + ": " + p.Message
	}
	return strings.Join(parts, "; ")
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (p *Problems) required(field, v string, max int) {
	switch {
	case strings.TrimSpace(v) == "":
		p.add(field, "Fill this in.")
	case utf8.RuneCountInString(v) > max:
		p.add(field, "Keep this under %d characters.", max)
	}
}

func (p *Problems) optional(field, v string, max int) {
	if utf8.RuneCountInString(v) > max {
		p.add(field, "Keep this under %d characters.", max)
	}
}

func (p *Problems) slug(field, v string) {
	if !slugPattern.MatchString(v) || len(v) > 120 {
		p.add(field, "Use lowercase letters, numbers and single hyphens, for example diploma-in-nursing.")
	}
}

func (p *Problems) oneOf(field, v string, allowed ...string) {
	if !slices.Contains(allowed, v) {
		p.add(field, "Choose one of: %s.", strings.Join(allowed, ", "))
	}
}

// link accepts a site path (/admissions) or an https URL.
func (p *Problems) link(field, v string) {
	if v == "" {
		return
	}
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") {
		return
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		p.add(field, "Use a site path like /admissions or a full https:// link.")
	}
}

func (p *Problems) email(field, v string) {
	if v == "" {
		return
	}
	if a, err := mail.ParseAddress(v); err != nil || a.Address != v {
		p.add(field, "Enter an email address like admissions@college.example.")
	}
}

var phonePattern = regexp.MustCompile(`^\+?[0-9 ]{7,20}$`)

func (p *Problems) phone(field, v string) {
	if v != "" && !phonePattern.MatchString(v) {
		p.add(field, "Enter a phone number with digits and spaces, like +254 712 345 678.")
	}
}

// Date is a calendar date serialised as YYYY-MM-DD.
type Date struct{ time.Time }

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.Format(time.DateOnly)) }

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return fmt.Errorf("dates must look like 2027-01-11")
	}
	d.Time = t
	return nil
}

func datePtr(t *time.Time) *Date {
	if t == nil {
		return nil
	}
	return &Date{*t}
}

func timePtr(d *Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}
