package view

import (
	"strings"
	"testing"
)

// A hyperlink is an OSC sequence, not the SGR the width functions were written for, and
// the two are terminated differently: SGR ends at `m`, OSC at ST or BEL. A scanner that
// stops at the first `m` swallows the visible text of any link whose URL has no `m` in it
// and truncates one that does — either way the arithmetic that keeps the frame inside the
// terminal is reading a width that is not there (§18, EVIDENCE.md §9.34).
func TestPrintedIgnoresHyperlinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"plain", "abc", 3},
		{"sgr", fg(inkPrimary, "abc"), 3},
		// No `m` anywhere in the URL: a scan-to-`m` swallows the rest of the line.
		{"link", link("https://api.localhost", "↗"), 1},
		// An `m` inside the URL is where a scan-to-`m` stops early and then counts the
		// remainder of the URL as visible text.
		{"link with m in url", link("https://team.localhost/admin", "↗"), 1},
		{"painted link", link("https://api.localhost", fg(inkSecondary, "↗")), 1},
		{"link between text", "ab" + link("https://api.localhost", "↗") + "cd", 5},
		{"bel terminated", "\033]8;;https://api.localhost\aX\033]8;;\a", 1},
	}
	for _, c := range cases {
		if got := printed(c.in); got != c.want {
			t.Errorf("printed(%s) = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestClampLineKeepsHyperlinksWhole(t *testing.T) {
	l := "abc" + link("https://api.localhost", "↗")
	// Wide enough for everything: the link survives byte for byte.
	if got := clampLine(l, 10); got != l {
		t.Errorf("clampLine did not pass a whole link through:\n got %q\nwant %q", got, l)
	}

	// Cut before the link. An escape costs no columns, so the four printed columns of
	// "abc↗" must be what decides — and the cut must close the hyperlink it left open,
	// or every line written after it inherits the link and the whole screen below turns
	// clickable.
	got := clampLine(l, 3)
	if printed(got) != 3 {
		t.Errorf("clampLine(%q, 3) printed %d columns: %q", l, printed(got), got)
	}
	if strings.Contains(got, "\033]8;;https") && !strings.HasSuffix(got, "\033]8;;\033\\") {
		t.Errorf("clamped line leaves a hyperlink open: %q", got)
	}

	// A line with no link is untouched by any of this.
	if got := clampLine("abcdef", 3); got != "abc\033[0m" {
		t.Errorf("clampLine of plain text = %q", got)
	}
}

// link is inert without a URL, so a row with nothing to point at renders exactly the
// glyph and no escape at all — the frame of a fleet with no previews is byte-identical
// to the frame before any of this existed.
func TestLinkWithoutURL(t *testing.T) {
	if got := link("", "↗"); got != "↗" {
		t.Errorf("link with no url = %q, want the bare text", got)
	}
}
