package view

import (
	"strings"
	"testing"
)

// Navigation must follow on-screen order (blocked, working, quiet), not the data's
// sort order (blocked, quiet, working).
func TestDisplayOrderMatchesScreen(t *testing.T) {
	got := DisplayOrder(fixture())
	want := []string{"S-BLK", "S-RUN", "S-OLD", "S-NEW"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("display order: got %v want %v", got, want)
	}
	// The background agent in the fixture has no surface and must not be a stop on
	// the way down the list — there is no tab to focus.
	for _, id := range got {
		if id == "" {
			t.Error("a row with no tab entered the navigation order")
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
		{"first press down selects the top row", "", +1, "S-BLK"},
		{"first press up selects the bottom row", "", -1, "S-NEW"},
		{"down crosses bands", "S-BLK", +1, "S-RUN"},
		{"up at the top clamps rather than wrapping onto a blocked row", "S-BLK", -1, "S-BLK"},
		{"down at the bottom clamps", "S-NEW", +1, "S-NEW"},
		// A session that disappeared between ticks must not strand the cursor.
		{"a vanished selection falls back to the top", "S-GONE", +1, "S-BLK"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Step(o, c.sel, c.delta); got != c.want {
				t.Errorf("Step(%q, %+d) = %q, want %q", c.sel, c.delta, got, c.want)
			}
		})
	}
	if got := Step(nil, "S-BLK", +1); got != "" {
		t.Errorf("Step on an empty fleet = %q, want empty", got)
	}
}
