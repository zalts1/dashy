package watch

import (
	"strconv"
	"strings"
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
	// Bit 5 is motion and bit 6 is the wheel; the low two bits name the button. The wheel
	// is dropped rather than bound because the frame always fits — there is nothing under
	// the bottom line to scroll to.
	if b&0b1100011 != 0 {
		return event{}, false
	}
	// Terminals count from one. A zero is not a row to clamp to the top, it is a report
	// board did not understand.
	if y < 1 {
		return event{}, false
	}
	return event{k: keyClick, row: y - 1}, true
}

// hitAt resolves a click against the frame that is on the screen. Out of range and
// on-chrome are the same answer — the reader missed, and a miss must never act.
func hitAt(hits []string, row int) string {
	if row < 0 || row >= len(hits) {
		return ""
	}
	return hits[row]
}
