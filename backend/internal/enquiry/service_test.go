package enquiry

import "testing"

func TestCountLinks(t *testing.T) {
	cases := map[string]int{
		"no links here":                      0,
		"see https://a.example":              1,
		"HTTP://A.EXAMPLE and www.b.example": 2,
		"https://a.example http://b www.c.d": 3,
	}
	for msg, want := range cases {
		if got := countLinks(msg); got != want {
			t.Errorf("countLinks(%q) = %d, want %d", msg, got, want)
		}
	}
}

// The per-contact limit must treat the same person the same way, however
// they type their details.
func TestContactKey(t *testing.T) {
	if contactKey(Submission{Email: "A@Example.invalid"}) != contactKey(Submission{Email: "a@example.invalid"}) {
		t.Error("email case changes the key")
	}
	if contactKey(Submission{Phone: "+254 000 000 001"}) != contactKey(Submission{Phone: "254000000001"}) {
		t.Error("phone spacing changes the key")
	}
	if contactKey(Submission{Email: "a@example.invalid", Phone: "1"}) != contactKey(Submission{Email: "a@example.invalid"}) {
		t.Error("email should be the key when given")
	}
}
