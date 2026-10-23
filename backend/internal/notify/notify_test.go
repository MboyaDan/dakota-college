package notify

import (
	"slices"
	"testing"
)

func TestRender(t *testing.T) {
	got := Render("Hello {name}, ref {reference}. {unknown} stays; {{name}} braces.", map[string]string{"name": "Test Applicant 001", "reference": "DTC-APP-2026-000001"})
	want := "Hello Test Applicant 001, ref DTC-APP-2026-000001. {unknown} stays; {Test Applicant 001} braces."
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestUnknownPlaceholders(t *testing.T) {
	bad := unknownPlaceholders(ApplicantSignInLink, "{name} {link} {reference} {programme} {reference} {Name}")
	if !slices.Equal(bad, []string{"reference", "programme"}) {
		t.Fatalf("got %v", bad)
	}
	for key, names := range Placeholders {
		for _, n := range names {
			if b := unknownPlaceholders(key, "{"+n+"}"); len(b) != 0 {
				t.Fatalf("%s rejects its own placeholder %s", key, n)
			}
		}
	}
}
