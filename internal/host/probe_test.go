package host

import "testing"

// Probe's only judgement is which line of stdout to believe. `cmux --version` prints
// one line; a tool that prints a banner first must not turn into a multi-line answer
// wedged into a one-line report.
func TestFirstLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"cmux 0.64.16 (96) [5321becb6]\n", "cmux 0.64.16 (96) [5321becb6]"},
		{"2.1.220 (Claude Code)", "2.1.220 (Claude Code)"},
		{"first\nsecond\nthird\n", "first"},
		{"  padded  \n", "padded"},
		{"", ""},
		{"\n\n", ""},
	}
	for _, c := range cases {
		if got := firstLine([]byte(c.in)); got != c.want {
			t.Errorf("firstLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
