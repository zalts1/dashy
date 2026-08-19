package cmux

import "testing"

// `defaults read` prints a boolean as 0 or 1, and prints nothing at all for a key that was never
// set. An absent preference is cmux's own default rather than an unknown, which is why the
// fallback is a parameter and not a zero value: board reports what will happen, and what will
// happen when nobody has chosen is "inside cmux".
func TestParseBoolDefault(t *testing.T) {
	cases := []struct {
		in   string
		def  bool
		want bool
	}{
		{"0\n", true, false},
		{"1\n", false, true},
		{"false\n", true, false},
		{"true\n", false, true},
		// Never set: `defaults read` errors and prints nothing, so the default stands.
		{"", true, true},
		{"", false, false},
		// Something unrecognised is not a reason to claim the opposite of the default.
		{"maybe\n", true, true},
	}
	for _, c := range cases {
		if got := parseBoolDefault(c.in, c.def); got != c.want {
			t.Errorf("parseBoolDefault(%q, %v) = %v, want %v", c.in, c.def, got, c.want)
		}
	}
}
