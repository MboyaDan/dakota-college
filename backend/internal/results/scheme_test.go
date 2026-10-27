package results

import "testing"

func score(n int) Mark { return Mark{Score: &n} }

func TestComputeMarks(t *testing.T) {
	s := Default()
	for _, c := range []struct {
		cat, exam int
		total     int
		grade     string
		passed    bool
	}{
		{100, 100, 100, "A", true},
		{70, 70, 70, "A", true},
		{50, 40, 43, "D", true}, // 15 + 28 = 43
		{39, 39, 39, "E", false},
		{0, 0, 0, "E", false},
		{45, 35, 38, "E", false}, // 13.5 + 24.5 = 38
		{41, 40, 40, "D", true},  // 12.3 + 28 = 40.3, rounds to 40
		{1, 0, 0, "E", false},    // 0.3 rounds to 0
		{2, 0, 1, "E", false},    // 0.6 rounds to 1
		{60, 59, 59, "C", true},  // 18 + 41.3 = 59.3
	} {
		r := Compute(s, map[string]Mark{"cat": score(c.cat), "exam": score(c.exam)})
		if !r.Complete || *r.Total != c.total || r.Grade != c.grade || r.Passed != c.passed {
			t.Errorf("cat %d exam %d: %+v total %d, want %d %s %v", c.cat, c.exam, r, *r.Total, c.total, c.grade, c.passed)
		}
	}
	if r := Compute(s, map[string]Mark{"cat": score(80)}); r.Complete || r.Total != nil {
		t.Errorf("incomplete: %+v", r)
	}
}

func TestComputeCompetency(t *testing.T) {
	s := Scheme{Kind: "competency"}
	if err := s.Normalise(); err != nil {
		t.Fatal(err)
	}
	yes, no := "competent", "not_yet_competent"
	if r := Compute(s, map[string]Mark{CompetencyComponent: {Outcome: &yes}}); !r.Complete || !r.Passed {
		t.Errorf("competent: %+v", r)
	}
	if r := Compute(s, map[string]Mark{CompetencyComponent: {Outcome: &no}}); !r.Complete || r.Passed {
		t.Errorf("not yet competent: %+v", r)
	}
	if r := Compute(s, nil); r.Complete {
		t.Errorf("no outcome: %+v", r)
	}
}

func TestNormalise(t *testing.T) {
	ok := Scheme{Kind: "marks", Components: []Component{{Name: "Practical work", Weight: 60}, {Name: "Theory exam", Weight: 40}},
		PassMark: 50, Bands: []Band{{"Distinction", 80}, {"Credit", 65}, {"Pass", 50}, {"Fail", 0}}}
	if err := ok.Normalise(); err == nil {
		t.Fatal("a 11-character grade was accepted")
	}
	ok.Bands = []Band{{"D", 80}, {"C", 65}, {"P", 50}, {"F", 0}}
	if err := ok.Normalise(); err != nil {
		t.Fatalf("valid scheme refused: %v", err)
	}
	if ok.Components[0].Key != "practical_work" || ok.IsDefault {
		t.Fatalf("keys not filled: %+v", ok.Components)
	}
	for name, bad := range map[string]Scheme{
		"weights not 100":      {Kind: "marks", Components: []Component{{Name: "A", Weight: 50}, {Name: "B", Weight: 40}}, PassMark: 40, Bands: []Band{{"A", 0}}},
		"duplicate parts":      {Kind: "marks", Components: []Component{{Name: "Exam", Weight: 50}, {Name: "exam", Weight: 50}}, PassMark: 40, Bands: []Band{{"A", 0}}},
		"bands not descending": {Kind: "marks", Components: []Component{{Name: "Exam", Weight: 100}}, PassMark: 40, Bands: []Band{{"B", 50}, {"A", 70}, {"E", 0}}},
		"lowest band not 0":    {Kind: "marks", Components: []Component{{Name: "Exam", Weight: 100}}, PassMark: 40, Bands: []Band{{"A", 70}, {"B", 40}}},
		"no parts":             {Kind: "marks", PassMark: 40, Bands: []Band{{"A", 0}}},
		"unknown kind":         {Kind: "letters"},
		"pass mark too high":   {Kind: "marks", Components: []Component{{Name: "Exam", Weight: 100}}, PassMark: 101, Bands: []Band{{"A", 0}}},
	} {
		if err := bad.Normalise(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
