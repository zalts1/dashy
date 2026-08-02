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

// burstWindow is how close together two caret steps have to be to be one gesture.
//
// **Measured, not chosen.** Probing Ghostty on the alternate screen, one flick of the wheel
// produced nine steps in nine separate reads, all inside the same millisecond — and it did
// so identically whether they arrived as arrow keys or as SGR reports (§9.32). So this is
// not a mouse rule: with reporting off the terminal translates notches into `↑`/`↓` itself,
// and board has been taking all nine since long before it knew the mouse existed.
//
// The number sits in the gap the measurement found, and it is sized for the slowest
// assumption rather than this machine's. A gesture spans under a millisecond. The fastest
// key repeat is 15ms on macOS — but X11 will take `xset r rate 100`, which is 10ms, and
// board ships as source, so 10ms is the floor that has to survive. 5ms leaves a factor of
// two under it and five times the widest burst yet seen.
//
// The two failure modes are not symmetric, which is what decides the bias. Too small and a
// flick moves a few rows instead of one — the behaviour board had for its whole life, mild
// and self-evident. Too large and a held arrow key silently drops repeats, which reads as a
// broken tool and gives the reader nothing to go on. When in doubt, shrink it.
const burstWindow = 5 * time.Millisecond

// stepClock collapses a burst of caret steps into one. It is deliberately blind to where a
// step came from: the whole finding is that the wheel and the arrow keys are the same
// events, so a rule that asked which would be answering the question that misled twice.
type stepClock struct{ last time.Time }

// allow reports whether this step may move the caret. The first one always may — a wheel
// whose opening event is swallowed reads as a wheel that does not work.
func (c *stepClock) allow(now time.Time) bool {
	if !c.last.IsZero() && now.Sub(c.last) < burstWindow {
		return false
	}
	c.last = now
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
