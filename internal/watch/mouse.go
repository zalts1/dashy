package watch

import (
	"strconv"
	"strings"
	"time"
)

// Mouse reporting is off unless ~/.board.json turns it on, because turning it on costs
// the reader drag-to-select: with the terminal forwarding presses, copying a workspace
// name out of the frame needs a modifier. That is a real loss on a tab left open all day,
// so it is the reader's trade to make and not board's (DESIGN.md §7).
//
// ?1000 reports button presses only — not motion, which would redraw on every twitch for
// a frame that has nothing to hover. ?1006 is the SGR encoding, and it is not optional:
// the original form packs each coordinate into one byte and stops reporting past column
// 223, which on a wide monitor is the right-hand half of the table.
//
// Asking for reports also takes the wheel away. On the alternate screen a terminal has no
// scrollback to move, so it translates notches into arrow keys — and that is suppressed
// the moment an application says it wants mouse events. The wheel is bound back to the
// same two keys here, which is not a new feature but the one board silently removed
// (EVIDENCE.md §9.32).
const (
	mouseOn  = "\033[?1000h\033[?1006h"
	mouseOff = "\033[?1006l\033[?1000l"
)

// sgrMouse reads one `ESC [ < b ; x ; y M|m` report. Only a left press is a click: the
// release arrives as the same sequence ending in `m`, and letting it through would fire
// every action twice.
func sgrMouse(params string, final rune) (event, bool) {
	if final != 'M' || !strings.HasPrefix(params, "<") {
		return event{}, false
	}
	f := strings.Split(params[1:], ";")
	if len(f) != 3 {
		return event{}, false
	}
	b, err1 := strconv.Atoi(f[0])
	y, err3 := strconv.Atoi(f[2])
	if _, err2 := strconv.Atoi(f[1]); err1 != nil || err2 != nil || err3 != nil {
		return event{}, false
	}
	// Bit 5 is motion, bit 6 is the wheel, and the low two bits name the button or the
	// direction. Everything above that is a modifier, which changes neither.
	if b&0b100000 != 0 {
		return event{}, false
	}
	if b&0b1000000 != 0 {
		// A notch carries no row deliberately: it is a direction, and reading it as a
		// position would step the selection to wherever the pointer was resting instead.
		switch b & 0b11 {
		case 0:
			return event{k: keyScrollUp}, true
		case 1:
			return event{k: keyScrollDown}, true
		}
		return event{}, false // the horizontal wheel: the list has one axis
	}
	if b&0b11 != 0 {
		return event{}, false // the middle and right buttons are bound to nothing
	}
	// Terminals count from one. A zero is not a row to clamp to the top, it is a report
	// board did not understand.
	if y < 1 {
		return event{}, false
	}
	return event{k: keyClick, row: y - 1}, true
}

// wheelInterval is the least time between two caret steps the wheel is allowed to make.
//
// **A gesture is not a notch.** A trackpad flick reports a step per scroll line — nine of
// them from one swipe, which carried the caret across a whole band and past the row the
// reader was aiming at (§9.32). The mouse cannot say how many of its reports were one
// intent, so the rate is what separates them: everything inside a flick is one step, and a
// scroll held down still moves at about five rows a second, which is a list being read
// rather than a list being fired past.
const wheelInterval = 200 * time.Millisecond

// wheelClock rations the wheel and nothing else. A held arrow key must still repeat at
// whatever rate the terminal sends it: that rate is the reader's finger, and it is already
// the right one.
type wheelClock struct{ last time.Time }

// allow reports whether this notch may move the caret, and is why the first one always
// does — a wheel whose opening event is swallowed reads as a wheel that does not work.
func (w *wheelClock) allow(now time.Time) bool {
	if now.Sub(w.last) < wheelInterval {
		return false
	}
	w.last = now
	return true
}

// hitAt resolves a click against the frame that is on the screen. Out of range and
// on-chrome are the same answer — the reader missed, and a miss must never act.
func hitAt(hits []string, row int) string {
	if row < 0 || row >= len(hits) {
		return ""
	}
	return hits[row]
}
