package finance

import (
	"testing"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestSplitAddsUpExactly(t *testing.T) {
	cases := []struct {
		total int64
		plan  []Instalment
		want  []int64
	}{
		{100, nil, []int64{100}},
		{100, []Instalment{{50, 0}, {50, 30}}, []int64{50, 50}},
		// One cent cannot be split: the last part takes the remainder.
		{1, []Instalment{{50, 0}, {50, 30}}, []int64{0, 1}},
		{1000, []Instalment{{33, 0}, {33, 30}, {34, 60}}, []int64{330, 330, 340}},
		{1001, []Instalment{{33, 0}, {33, 30}, {34, 60}}, []int64{330, 330, 341}},
		{3_000_000_000_000, []Instalment{{1, 0}, {99, 30}}, []int64{30_000_000_000, 2_970_000_000_000}},
	}
	for _, c := range cases {
		got := Split(c.total, c.plan)
		var sum int64
		for i := range got {
			sum += got[i]
			if got[i] != c.want[i] {
				t.Errorf("Split(%d, %v) = %v, want %v", c.total, c.plan, got, c.want)
				break
			}
		}
		if sum != c.total {
			t.Errorf("Split(%d) parts add up to %d", c.total, sum)
		}
	}
}

func TestCheckInstalments(t *testing.T) {
	for _, c := range []struct {
		plan []Instalment
		ok   bool
	}{
		{nil, true},
		{[]Instalment{{100, 0}}, true},
		{[]Instalment{{60, 0}, {40, 45}}, true},
		{[]Instalment{{60, 0}, {30, 45}}, false},  // adds up to 90
		{[]Instalment{{50, 30}, {50, 30}}, false}, // same day twice
		{[]Instalment{{50, 60}, {50, 30}}, false}, // out of order
		{[]Instalment{{0, 0}, {100, 30}}, false},  // empty share
		{[]Instalment{{100, 731}}, false},         // too far out
		{make([]Instalment, 13), false},           // too many
		{[]Instalment{{101, 0}, {-1, 10}}, false}, // out of range
		{[]Instalment{{50, -1}, {50, 10}}, false}, // negative days
		{[]Instalment{{25, 0}, {25, 1}, {50, 2}}, true},
	} {
		var pr content.Problems
		CheckInstalments(&pr, c.plan)
		if (len(pr) == 0) != c.ok {
			t.Errorf("%v: problems %v, want ok=%v", c.plan, pr, c.ok)
		}
	}
}

func TestCheckLines(t *testing.T) {
	var pr content.Problems
	out := CheckLines(&pr, []Line{
		{Label: "  Tuition   fee ", Category: "tuition", AmountCents: 4_500_000},
		{Label: "", Category: "books", AmountCents: 0},
		{Label: "Too much", Category: "other", AmountCents: MaxAmountCents + 1},
	})
	if out[0].Label != "Tuition fee" {
		t.Errorf("label not tidied: %q", out[0].Label)
	}
	want := map[string]bool{"lines.1.label": true, "lines.1.category": true, "lines.1.amount_cents": true, "lines.2.amount_cents": true}
	if len(pr) != len(want) {
		t.Fatalf("problems %v", pr)
	}
	for _, p := range pr {
		if !want[p.Field] {
			t.Errorf("unexpected problem %v", p)
		}
	}

	pr = nil
	CheckStructureLines(&pr, nil)
	if len(pr) != 1 || pr[0].Field != "lines" {
		t.Errorf("empty structure: %v", pr)
	}
	pr = nil
	CheckStructureLines(&pr, []StructureLine{{Label: "Tuition", Category: "tuition", Charge: "yearly", AmountCents: 1}})
	if len(pr) != 1 || pr[0].Field != "lines.0.charge" {
		t.Errorf("bad charge: %v", pr)
	}
}

func TestSchedule(t *testing.T) {
	today := day("2027-03-01")
	dues := []Due{
		{On: day("2027-04-01"), Amount: 30_000},
		{On: day("2027-01-15"), Amount: 50_000},
		{On: day("2027-03-01"), Amount: 20_000}, // due today counts as due
	}
	cases := []struct {
		charges, settled, overdue int64
		next                      string
		nextAmount                int64
	}{
		{0, 0, 70_000, "2027-04-01", 30_000},
		{0, 60_000, 10_000, "2027-04-01", 30_000},
		{0, 80_000, 0, "2027-04-01", 20_000},
		{0, 100_000, 0, "", 0},
		{0, 120_000, 0, "", 0}, // credit balance
		{5_000, 70_000, 5_000, "2027-04-01", 30_000},
	}
	for _, c := range cases {
		overdue, on, amount := Schedule(dues, c.charges, c.settled, today)
		got := ""
		if on != nil {
			got = on.Format(time.DateOnly)
		}
		if overdue != c.overdue || got != c.next || amount != c.nextAmount {
			t.Errorf("charges %d settled %d: overdue %d next %q %d; want %d %q %d",
				c.charges, c.settled, overdue, got, amount, c.overdue, c.next, c.nextAmount)
		}
	}
}

func TestRunOrdersAndBalances(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			panic(err)
		}
		return v
	}
	entries, closing := Run([]Entry{
		NewEntry("payment", "Payment", "R2", 0, 30_000, day("2027-01-20"), at("2027-01-21T08:00:00Z")),
		NewEntry("invoice", "Invoice", "I1", 100_000, 0, day("2027-01-10"), at("2027-01-10T09:00:00Z")),
		NewEntry("credit", "Bursary", "", 0, 10_000, day("2027-01-20"), at("2027-01-20T07:00:00Z")),
		NewEntry("charge", "Late fee", "", 1_000, 0, day("2027-02-01"), at("2027-02-01T07:00:00Z")),
	})
	wantRefs := []string{"I1", "", "R2", ""}
	wantBal := []int64{100_000, 90_000, 60_000, 61_000}
	for i, e := range entries {
		if e.Reference != wantRefs[i] || e.BalanceCents != wantBal[i] {
			t.Errorf("entry %d: %+v", i, e)
		}
	}
	if closing != 61_000 {
		t.Errorf("closing %d", closing)
	}
}

func TestDateOfUsesNairobi(t *testing.T) {
	// 22:30 UTC on 31 December is already 1 January in Nairobi.
	got := dateOf(time.Date(2026, 12, 31, 22, 30, 0, 0, time.UTC))
	if got.Format(time.DateOnly) != "2027-01-01" {
		t.Errorf("dateOf = %s", got)
	}
}
