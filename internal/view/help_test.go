package view

import (
	"strings"
	"testing"
	"time"
)

// The bottom line has three states now and they all occupy the same one line, so entering any
// of them cannot change the frame's height — the rule §12 set when capture moved into the
// frame. Typing is the most specific and wins; help is next; ambient is the rest.
func TestBottomLineStates(t *testing.T) {
	f := linkFleet()
	s := Screen{Now: time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC),
		Interval: 10 * time.Second, Threshold: 45 * time.Minute, Rows: 44, Cols: 118,
		EditorScheme: "cursor"}

	height := func(u UI) int { return strings.Count(Frame(f, s, u), "\n") }
	base := height(UI{})
	for name, u := range map[string]UI{
		"help":   {Help: true},
		"typing": {Typing: true, Paused: true, Input: "a new todo"},
		"both":   {Help: true, Typing: true, Paused: true, Input: "x"},
	} {
		if got := height(u); got != base {
			t.Errorf("%s changed the frame height: %d, want %d", name, got, base)
		}
	}
	// Typing is the more specific state and owns the line while it is on.
	both := bottom(f, UI{Help: true, Typing: true, Input: "x"}, true, true, 118)
	if !strings.Contains(both, "new todo  ") {
		t.Errorf("typing did not take the line back from help: %q", both)
	}
}

// The ambient line names the bar's *dimension* and the two routes that exist; the rung values
// and the glyph meanings move behind `?`. That is the §6 amendment: a scale still has a key on
// screen — "elapsed" — and `?` is where its resolution lives (§9.38).
func TestAmbientLineNamesTheRoutes(t *testing.T) {
	got := bottom(linkFleet(), UI{}, true, true, 118)
	for _, want := range []string{"elapsed", "⌘-click opens", "? keys"} {
		if !strings.Contains(got, want) {
			t.Errorf("ambient line does not say %q:\n%q", want, got)
		}
	}
	// The rungs are not on the ambient line any more.
	if strings.Contains(got, "12h") {
		t.Errorf("the scale's rungs are still on the ambient line:\n%q", got)
	}
	// And it is shorter than the line it replaced, not longer.
	if printed(got) > 90 {
		t.Errorf("ambient line grew to %d columns:\n%q", printed(got), got)
	}
}

// `?` is help, so it spells out everything the ambient line abbreviated: the scale's rungs, all
// three glyphs by name, the gesture, and the way out. All three glyphs whether or not this fleet
// has them — a help line that hid a feature would be answering a different question.
func TestHelpLineSpellsEverythingOut(t *testing.T) {
	got := bottom(linkFleet(), UI{Help: true}, true, true, 118)
	for _, want := range []string{
		"1h", "3h", "12h", "2d", "7d",
		previewGlyph, storybookGlyph, folderGlyph, prGlyph, staleGlyph,
		"storybook", "preview", "folder", "pr", "merged", "quiet a while", "⌘-click", "esc",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help line does not say %q:\n%q", want, got)
		}
	}
	// It fits the terminal it was asked for, which is the bar that matters: this is the one line a
	// reader opened deliberately, and a legend clipped mid-word is worse than a shorter one.
	if w := printed(got) + 2*headMargin; w > 118 {
		t.Errorf("help line needs %d columns at 118:\n%q", w, got)
	}
	// It sheds in a ladder rather than clipping, and the glyph meanings never give way — they are
	// the question `?` is answering. The scale goes first, then the gesture.
	for _, cols := range []int{118, 100, 90, 80} {
		narrow := bottom(linkFleet(), UI{Help: true}, true, true, cols)
		if w := printed(narrow) + 2*headMargin; w > cols {
			t.Errorf("help line needs %d columns at %d:\n%q", w, cols, narrow)
		}
		for _, g := range []string{staleGlyph, storybookGlyph, previewGlyph, folderGlyph,
			prGlyph, prMergedGlyph} {
			if !strings.Contains(narrow, g) {
				t.Errorf("at %d cols the legend dropped %q, which is what it is for:\n%q",
					cols, g, narrow)
			}
		}
	}
	narrow := bottom(linkFleet(), UI{Help: true}, true, true, 80)
	for _, g := range []string{staleGlyph, storybookGlyph, previewGlyph, folderGlyph,
		prGlyph, prMergedGlyph} {
		if !strings.Contains(narrow, g) {
			t.Errorf("the narrow help line dropped %q, which is the part it is for:\n%q", g, narrow)
		}
	}
	if strings.Contains(narrow, "12h") {
		t.Errorf("the narrow help line kept the scale rungs instead of shedding them:\n%q", narrow)
	}
	// A fleet with no links still gets the whole legend: it is help, not a status line.
	bare := linkFleet()
	for i := range bare.Rows {
		bare.Rows[i].Preview, bare.Rows[i].Storybook, bare.Rows[i].Folder = "", "", ""
	}
	if h := bottom(bare, UI{Help: true}, true, false, 118); !strings.Contains(h, "storybook") {
		t.Errorf("help hid a feature the fleet is not using:\n%q", h)
	}
}

// The gesture hint is named only when the cell is actually on screen — §9.14 read forwards, and
// the same condition the cell itself uses. A tab too narrow for the cell must not advertise it.
func TestGestureHintFollowsTheCell(t *testing.T) {
	if got := bottom(linkFleet(), UI{}, true, false, 118); strings.Contains(got, "⌘-click") {
		t.Errorf("the gesture is named with no cell on screen:\n%q", got)
	}
}
