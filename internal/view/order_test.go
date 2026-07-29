package view

import (
	"strings"
	"testing"
)

// Navigation must follow on-screen order (blocked, working, quiet), not the data's
// sort order (blocked, quiet, working).
//
// Every row is a stop, including the background agents with no tab. Selection is not
// only a jump target: it is also what lifts a row out of the collapsed quiet tail, so
// skipping the tab-less rows made them countable and undrawable (EVIDENCE.md §9.14).
// Enter on one reports that there is nothing to focus.
func TestDisplayOrderMatchesScreen(t *testing.T) {
	got := DisplayOrder(fixture())
	want := []string{"K-BLK", "K-BG", "K-RUN", "K-OLD", "K-NEW"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("display order: got %v want %v", got, want)
	}
	// Keyed on the session, never on a row index or on the surface: a background agent
	// has no surface, and an index drifts when the fleet re-sorts.
	for _, id := range got {
		if id == "" {
			t.Error("a row with no key entered the navigation order; it could never be selected")
		}
	}
}

func TestStepClamps(t *testing.T) {
	o := DisplayOrder(fixture())
	cases := []struct {
		name  string
		sel   string
		delta int
		want  string
	}{
		{"first press down selects the top row", "", +1, "K-BLK"},
		{"first press up selects the bottom row", "", -1, "K-NEW"},
		{"down crosses bands", "K-BG", +1, "K-RUN"},
		{"up at the top clamps rather than wrapping onto a blocked row", "K-BLK", -1, "K-BLK"},
		{"down at the bottom clamps", "K-NEW", +1, "K-NEW"},
		// A session that disappeared between ticks must not strand the cursor.
		{"a vanished selection falls back to the top", "K-GONE", +1, "K-BLK"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Step(o, c.sel, c.delta); got != c.want {
				t.Errorf("Step(%q, %+d) = %q, want %q", c.sel, c.delta, got, c.want)
			}
		})
	}
	if got := Step(nil, "K-BLK", +1); got != "" {
		t.Errorf("Step on an empty fleet = %q, want empty", got)
	}
}
