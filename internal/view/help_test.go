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
	// The fullest rung, on a tab wide enough for it: the scale's values, every glyph by name, the
	// gesture and the way back.
	full := bottom(linkFleet(), UI{Help: true}, true, true, 140)
	for _, want := range []string{
		"1h", "3h", "12h", "2d", "7d",
		previewGlyph, storybookGlyph, folderGlyph, prGlyph, prMergedGlyph, absentGlyph, staleGlyph,
		"storybook", "preview", "folder", "pr", "merged", "none", "quiet a while", "⌘-click", "esc",
	} {
		if !strings.Contains(full, want) {
			t.Errorf("the full help line does not say %q:\n%q", want, full)
		}
	}

	// It sheds in a ladder as the tab narrows — the scale's values first, then the gesture — and
	// what it never sheds is the glyph meanings, because they are the question `?` is answering.
	widths := []int{140, 130, 118, 100, 90}
	prev := 0
	for _, cols := range widths {
		line := bottom(linkFleet(), UI{Help: true}, true, true, cols)
		if w := printed(line) + 2*headMargin; w > cols {
			t.Errorf("help line needs %d columns at %d:\n%q", w, cols, line)
		}
		if w := printed(line); prev != 0 && w > prev {
			t.Errorf("at %d cols the line grew to %d from %d", cols, w, prev)
		}
		prev = printed(line)
		for _, g := range []string{staleGlyph, storybookGlyph, previewGlyph, folderGlyph,
			prGlyph, prMergedGlyph, absentGlyph} {
			if !strings.Contains(line, g) {
				t.Errorf("at %d cols the legend dropped %q, which is what it is for:\n%q",
					cols, g, line)
			}
		}
	}

	// Below the shortest rung there is nothing left to shed, and clampLine takes it — the one case
	// this line is allowed to be cut. helpLine itself must still return the shortest rung whole,
	// rather than cutting it and hiding the fact.
	shortest := bottom(linkFleet(), UI{Help: true}, true, true, 90)
	if got := bottom(linkFleet(), UI{Help: true}, true, true, 40); got != shortest {
		t.Errorf("at 40 cols helpLine returned something other than its shortest rung:\n%q", got)
	}

	// A fleet with no links still gets the whole legend: it is help, not a status line.
	bare := linkFleet()
	for i := range bare.Rows {
		bare.Rows[i].Preview, bare.Rows[i].Storybook = "", ""
		bare.Rows[i].Folder, bare.Rows[i].PR = "", ""
	}
	if h := bottom(bare, UI{Help: true}, true, false, 140); !strings.Contains(h, "storybook") {
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
