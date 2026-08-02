package view

import (
	"strings"
	"testing"
)

// The hit map is the only place in this package where a position becomes an identity,
// and everything else here is an identity (§7). So it may only ever name a row the
// frame actually drew, on the line it drew it — a map built from anything other than
// the drawing pass is a second copy of the layout, and it drifts the first time a band
// moves (§7).
func TestHitsNameTheRowDrawnOnThatLine(t *testing.T) {
	f := fixture()
	frame, hits := FrameHits(f, screen(44, 130), UI{})
	lines := strings.Split(frame, "\n")
	if len(hits) > len(lines) {
		t.Fatalf("hit map is %d lines, frame is %d — a click could resolve past the end",
			len(hits), len(lines))
	}
	var got []string
	for i, key := range hits {
		if key == "" {
			continue
		}
		r, ok := f.ByKey(key)
		if !ok {
			t.Fatalf("line %d names %q, which is not a row in the fleet", i, key)
		}
		if !strings.Contains(lines[i], r.Label) {
			t.Errorf("line %d is mapped to %q but does not draw its label %q:\n%s",
				i, key, r.Label, lines[i])
		}
		got = append(got, key)
	}
	// Nothing trimmed at this size, so the click targets are exactly the keyboard's
	// stops. The two orders are the same order: both follow the screen.
	want := DisplayOrder(f, UI{})
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("hit order %v, want the display order %v", got, want)
	}
}

// Chrome is not a target. A click on the header, the KPI strip, a band heading or the
// legend has to resolve to nothing rather than to whatever row happens to be nearest —
// a near miss that selects is worse than a near miss that does nothing.
func TestChromeLinesAreNotClickable(t *testing.T) {
	frame, hits := FrameHits(fixture(), screen(44, 130), UI{})
	for i, l := range strings.Split(frame, "\n") {
		if i >= len(hits) {
			break
		}
		switch {
		case strings.TrimSpace(l) == "",
			strings.Contains(l, "BOARD"),
			strings.Contains(l, "NEEDS YOU"),
			strings.Contains(l, "WORKING"),
			strings.Contains(l, "QUIET"),
			strings.Contains(l, "LABEL"),
			strings.Contains(l, "ctrl-c to exit"):
			if hits[i] != "" {
				t.Errorf("line %d is chrome but maps to %q: %q", i, hits[i], l)
			}
		}
	}
}

// Every state the frame can be in, on the fleet whose quiet tail overflows: the hit map
// must stay inside the frame and must never name a row twice, whatever the fit loop shed.
func TestHitsSurviveEveryTrimAndFold(t *testing.T) {
	f := goldenFleet()
	order := map[string]int{}
	for i, k := range DisplayOrder(f, UI{}) {
		order[k] = i
	}
	cases := []struct {
		name string
		s    Screen
		u    UI
	}{
		{"wide", screen(44, 118), UI{}},
		{"narrow", screen(24, 90), UI{}},
		{"folded", screen(44, 118), UI{QuietCollapsed: true}},
		{"selected in the trimmed tail", screen(24, 118), UI{Sel: "K-F29", Paused: true}},
		{"a tab too short for the chrome", screen(6, 118), UI{}},
		{"typing", screen(44, 118), UI{Typing: true, Input: "a new note"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			frame, hits := FrameHits(f, c.s, c.u)
			if len(hits) > len(strings.Split(frame, "\n")) {
				t.Fatalf("hit map outruns the frame: %d entries, %d lines",
					len(hits), len(strings.Split(frame, "\n")))
			}
			seen, last := map[string]bool{}, -1
			for i, key := range hits {
				if key == "" {
					continue
				}
				if seen[key] {
					t.Errorf("%q is on two lines; one click would be ambiguous", key)
				}
				seen[key] = true
				pos, ok := order[key]
				if !ok {
					t.Errorf("line %d names %q, which the keyboard cannot reach", i, key)
					continue
				}
				// A folded quiet row leaves DisplayOrder, so it must leave the hit map too:
				// the mouse and the arrows have to agree about what is on the screen.
				if pos <= last {
					t.Errorf("%q is drawn out of display order", key)
				}
				last = pos
			}
		})
	}
	// The one that would otherwise pass by accident: folding must remove the quiet rows
	// from the map, not just from the picture.
	_, folded := FrameHits(f, screen(44, 118), UI{QuietCollapsed: true})
	for _, key := range folded {
		if strings.HasPrefix(key, "K-F") || key == "K-OLD" || key == "K-NEW" {
			t.Errorf("folded quiet row %q is still clickable", key)
		}
	}
}

// Frame is FrameHits without the map, and it must stay byte-identical: the golden
// frames are the pin on the rendering, and a hit map is not a rendering change.
func TestFrameMatchesFrameHits(t *testing.T) {
	for _, u := range []UI{{}, {QuietCollapsed: true}, {Sel: "K-OLD", Paused: true}} {
		with, _ := FrameHits(goldenFleet(), screen(44, 118), u)
		if got := Frame(goldenFleet(), screen(44, 118), u); got != with {
			t.Error("Frame and FrameHits disagree about the frame")
		}
	}
}
