package watch

import (
	"testing"
	"time"
)

// A click arrives as an escape sequence like any arrow key, and the decoder's job is to
// hand over the one thing a keystroke never carries: where it landed. SGR (?1006) is the
// form board asks for, because the older encoding puts the coordinates in single bytes
// and dies past column 223 — on a wide monitor that is the right-hand half of the frame.
//
// Only a left press is a click. A release, a drag and a wheel tick all arrive on the same
// channel, and a decoder that let them through would fire the same action three times.
func TestDecodeMouse(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []event
	}{
		{"a left press is a click on its row, counted from zero", "\x1b[<0;12;9M",
			[]event{{k: keyClick, row: 8}}},
		{"past column 223, where the older encoding gives up", "\x1b[<0;400;40M",
			[]event{{k: keyClick, row: 39}}},
		{"the release of that same click is not a second one", "\x1b[<0;12;9m", nil},
		{"the right button does nothing here", "\x1b[<2;12;9M", nil},
		{"the middle button does nothing here", "\x1b[<1;12;9M", nil},
		{"a drag is motion, not a press", "\x1b[<32;12;9M", nil},
		// The wheel steps the selection, which is what it did before board asked for mouse
		// reports at all: on the alternate screen the terminal was translating notches into
		// arrow keys, and turning reporting on took that away (EVIDENCE.md §9.32). It carries
		// no row — a notch is a direction, and treating it as a position would let it act on
		// whatever the pointer happened to be resting over. Kept apart from the arrow keys,
		// and not because they do anything different: one
		// flick of a trackpad is not one notch, so a wheel step is rationed and a keypress
		// never is (§9.32).
		{"the wheel steps up", "\x1b[<64;12;9M", []event{{k: keyScrollUp}}},
		{"the wheel steps down", "\x1b[<65;12;9M", []event{{k: keyScrollDown}}},
		{"a shifted wheel still steps", "\x1b[<69;12;9M", []event{{k: keyScrollDown}}},
		{"the horizontal wheel does nothing: the list has one axis", "\x1b[<66;12;9M", nil},
		{"a notch is a direction, not a place", "\x1b[<64;12;40M", []event{{k: keyScrollUp}}},
		{"a click types nothing, whatever mode is on", "\x1b[<0;5;5M", []event{{k: keyClick, row: 4}}},
		// The trap the arrow keys already taught: the tail of a sequence is printable, so a
		// click that is not consumed whole types "<0;12;9M" into a todo — and the `0` and `M`
		// are the least of it, since a stray `q` in those bytes would quit.
		{"a click coalesced with a keystroke", "\x1b[<0;1;3Mx",
			[]event{{k: keyClick, row: 2}, {k: keyNone, text: "x"}}},
		{"two clicks in one read", "\x1b[<0;1;3M\x1b[<0;1;7M",
			[]event{{k: keyClick, row: 2}, {k: keyClick, row: 6}}},
		{"a truncated sequence is never guessed at", "\x1b[<0;12", nil},
		{"nonsense coordinates are dropped, not clamped to row 0", "\x1b[<0;12;0M", nil},
		{"a report with a coordinate missing is dropped", "\x1b[<0;12M", nil},
		{"an empty parameter list is dropped", "\x1b[<;;M", nil},
		// Not a mouse report at all: `ESC [ 2 J` and friends share the CSI shape, and the
		// parameters only mean coordinates when the final byte says so.
		{"another CSI sequence is not a click", "\x1b[<0;12;9H", nil},
		// The pre-SGR encoding, which board never asks for but a terminal that declined
		// ?1006 would send anyway. Its three coordinate bytes are printable: left unswallowed
		// they are text, and one of them can be `q`.
		{"the old encoding is swallowed whole rather than typed", "\x1b[M q!", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decode(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("%q decoded to %+v, want %+v", c.in, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("%q event %d = %+v, want %+v", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}

// One flick of a trackpad is not one notch: the terminal reports a step per scroll line,
// and nine of them arrived from a single gesture — the caret crossed a whole band (§9.32).
// The wheel is therefore rationed to one step per interval, and the ration is on the wheel
// alone: a held arrow key must still repeat at whatever rate the terminal sends it.
func TestWheelIsRationedButKeysAreNot(t *testing.T) {
	t0 := time.Date(2026, 8, 2, 17, 16, 0, 0, time.UTC)
	var w wheelClock

	if !w.allow(t0) {
		t.Fatal("the first notch of a gesture was swallowed; a wheel that ignores the first event is a dead wheel")
	}
	// The rest of that flick, arriving every 16ms as the trackpad decelerates.
	for i := 1; i < 9; i++ {
		if w.allow(t0.Add(time.Duration(i) * 16 * time.Millisecond)) {
			t.Fatalf("notch %d of one gesture stepped the caret; that is the nine-row jump", i)
		}
	}
	// A deliberate second scroll, well after the first has stopped.
	if !w.allow(t0.Add(400 * time.Millisecond)) {
		t.Error("a second gesture was swallowed; the ration must not outlast the flick")
	}
	// Sustained scrolling still moves, just at a readable rate rather than the wire's.
	steps := 0
	for ms := 400; ms <= 1400; ms += 16 {
		if w.allow(t0.Add(time.Duration(ms) * time.Millisecond)) {
			steps++
		}
	}
	if steps < 3 || steps > 8 {
		t.Errorf("a second of continuous scrolling moved %d rows; want a handful, not the 62 events sent", steps)
	}
}

// The hit map comes from the frame that is on the screen, so a click below the last line
// or above the first has to resolve to nothing. Chrome resolves to nothing too, and both
// have to be the same answer: the loop treats "" as "the reader missed".
func TestHitAtIsBounded(t *testing.T) {
	hits := []string{"", "", "K-BLK", "", "K-RUN"}
	cases := []struct {
		row  int
		want string
	}{
		{2, "K-BLK"}, {4, "K-RUN"},
		{0, ""},  // the header
		{3, ""},  // a band heading between two rows
		{9, ""},  // past the end of a short frame
		{-1, ""}, // a coordinate no terminal should send
	}
	for _, c := range cases {
		if got := hitAt(hits, c.row); got != c.want {
			t.Errorf("hitAt(row %d) = %q, want %q", c.row, got, c.want)
		}
	}
	if got := hitAt(nil, 3); got != "" {
		t.Errorf("hitAt on a frame drawn before any hit map = %q, want empty", got)
	}
}
