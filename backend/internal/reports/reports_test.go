package reports

import "testing"

func TestKES(t *testing.T) {
	for cents, want := range map[int64]string{0: "0.00", 5: "0.05", 123456: "1234.56", 100: "1.00", -250: "-2.50"} {
		if got := KES(cents); got != want {
			t.Errorf("KES(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestPercent(t *testing.T) {
	for _, c := range []struct {
		part, whole int64
		want        string
	}{{2, 3, "66.7"}, {1, 3, "33.3"}, {0, 5, "0.0"}, {5, 5, "100.0"}, {1, 8, "12.5"}, {1, 0, ""}} {
		if got := Percent(c.part, c.whole); got != c.want {
			t.Errorf("Percent(%d, %d) = %q, want %q", c.part, c.whole, got, c.want)
		}
	}
}

func TestFormulaCellsAreGuarded(t *testing.T) {
	for in, want := range map[string]string{"=HYPERLINK(1)": "'=HYPERLINK(1)", "+1": "'+1", "-x": "'-x", "@SUM": "'@SUM", "Nursing": "Nursing", "": ""} {
		if got := text(in); got != want {
			t.Errorf("text(%q) = %q, want %q", in, got, want)
		}
	}
}
