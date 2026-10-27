package letters

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

var pageObject = regexp.MustCompile(`/Type\s*/Page[^s]`)

func TestFormatKES(t *testing.T) {
	for cents, want := range map[int64]string{
		0: "KES 0.00", 5: "KES 0.05", 4500000: "KES 45,000.00", 123456789: "KES 1,234,567.89", 100000: "KES 1,000.00", -250: "KES -2.50",
	} {
		if got := FormatKES(cents); got != want {
			t.Errorf("FormatKES(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestOfferLetter(t *testing.T) {
	long := strings.Repeat("This paragraph is long enough to wrap across several lines of the letter. ", 40)
	pdf, err := OfferLetter(Offer{
		Date: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), Reference: "DTC-APP-2026-000001",
		Name: "Test Applicant 001", Email: "applicant001@example.invalid",
		Heading: "Offer of admission", Body: "Dear Test,\n\n" + long + "\n\nLine one\nLine two",
		Conditions: "Bring your original KCSE certificate on the first day.",
		Fees: []Fee{
			{Label: "Tuition", Frequency: "per_term", AmountCents: 4500000},
			{Label: "Registration (Sample) with a label long enough to wrap inside its column", Frequency: "once", AmountCents: 150000, Notes: "paid once"},
		},
		Sample: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatal("not a PDF")
	}
	// The long body runs onto a second page.
	if n := len(pageObject.FindAll(pdf, -1)); n < 2 {
		t.Fatalf("%d pages, want at least 2", n)
	}
	if len(pdf) > 600_000 {
		t.Fatalf("letter is %d bytes; fonts should be subset", len(pdf))
	}
}

func TestResultsDocument(t *testing.T) {
	for _, official := range []bool{true, false} {
		pdf, err := ResultsDocument(Results{Official: official, Number: "DTC/TR/2026/0001", Date: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
			Name: "Test Student 001", StudentNumber: "DTC/SAMPLE/0001", Programme: "Diploma (Sample)",
			Terms: []ResultTerm{{Name: "Term 1", Rows: []ResultRow{{"ICT 101", "Basics", "73", "A"}, {"ICT 102", "More", "", "Competent"}}}}})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF-")) || len(pageObject.FindAll(pdf, -1)) != 1 {
			t.Fatalf("official=%v: not a one-page PDF", official)
		}
	}
	// No released results still makes a document that says so.
	if _, err := ResultsDocument(Results{Date: time.Now(), Name: "X", StudentNumber: "Y", Programme: "Z"}); err != nil {
		t.Fatal(err)
	}
}
