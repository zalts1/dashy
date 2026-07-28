package view

// Layout is the frame's arithmetic: how many lines it may occupy, how wide each
// elastic column is, and what gives way first when the terminal is too small for
// everything. It is kept apart from the rendering so the sizes can be asserted
// directly — every fit bug in DESIGN.md §9.10 and §9.12 was an arithmetic bug that
// looked correct in the rendering code.

import (
	"strings"

	"board/internal/board"
)

// Layout constants shared by every band so labels line up on one left edge.
const (
	gutter    = 10 // width of the state-mark column
	idleW     = 7  // the IDLE column, widest at "52d06h"
	maxLabelW = 80
	minLabelW = 18
	// rowChrome is every column a row spends outside the label and the tail: the lead,
	// the state gutter, the bar, the warn mark, the IDLE column, and the gaps between
	// them. Derived from the pieces rather than guessed — the fixed reserve of 46 it
	// replaces was short by the whole tail, so any long workspace name wrapped the row
	// (DESIGN.md §9.12).
	rowChrome = 3 + gutter + 1 + barCells + 1 + 1 + 1 + idleW + 2
	// rowChromeBare is the same row without the bar column, for terminals too narrow to
	// hold one whole. A cut bar is worse than no bar: same glyph run, smaller number.
	rowChromeBare = rowChrome - barCells - 1
	wsHeader      = "WORKSPACE" // also the tail column's floor: a column keeps room for its label
	// The quiet tail never shrinks below this: a QUIET band of one row reads as a
	// quiet fleet, which is the opposite of the truth.
	minQuietRows = 3
)

// height is how many terminal lines this frame occupies once written. The watch loop
// splits on newlines and writes each line with a trailing one, so the final newline
// moves the cursor one row past the last line — and a terminal scrolls if that lands
// past the bottom. Everything above the fold goes with it, starting with the header.
func height(frame string) int { return strings.Count(frame, "\n") + 2 }

// pickQuiet keeps the first n quiet rows, plus the selected one wherever it sits, and
// reports how many are hidden. It copies rather than reslicing: the caller reuses the
// band across passes.
func pickQuiet(quiet []board.Row, n int, sel string) (shown []board.Row, hidden int) {
	if n >= len(quiet) {
		return quiet, 0
	}
	n = max(n, 0)
	shown = make([]board.Row, n, n+1)
	copy(shown, quiet[:n])
	// A selection must stay visible even if it sits in the collapsed tail: short of a
	// taller tab it is the only way to read a hidden row. Appending is in order, not a
	// shortcut — quiet is sorted by idle descending and the collapse cuts from the fresh
	// end, so anything hidden belongs below everything shown (§9.14).
	for _, r := range quiet[n:] {
		if sel != "" && r.Key == sel {
			shown = append(shown, r)
			break
		}
	}
	return shown, len(quiet) - len(shown)
}

// clip drops trailing lines so the written frame cannot scroll the terminal.
func clip(frame string, rows int) string {
	if rows < 2 {
		return ""
	}
	lines := strings.Split(frame, "\n")
	if len(lines) <= rows-1 {
		return frame
	}
	return strings.Join(lines[:rows-1], "\n")
}

// columns sizes the two elastic columns to the content actually present: the label,
// so bars sit next to the text instead of across a gap of padding, and the tail — the
// workspace — which is unbounded in the data and was therefore the thing that
// overflowed.
//
// Sizing order is meaning first: the label takes the squeeze down to its floor, and
// only then does the tail give way. Both are measured, so a resize changes the label
// column and leaves every other column where it was.
func columns(f board.Fleet, cols int) (labelW, tailW int, bars bool) {
	for _, r := range f.Rows {
		labelW = max(labelW, runes(r.Label))
		tailW = max(tailW, runes(r.Workspace))
	}
	labelW = min(labelW, maxLabelW)
	tailW = max(tailW, runes(wsHeader))

	// headMargin keeps the same right-hand margin the header keeps, so the table's
	// ragged right edge stops where the clock does.
	bars = cols-headMargin >= rowChrome+minLabelW
	chrome := rowChromeBare
	if bars {
		chrome = rowChrome
	}
	avail := cols - headMargin - chrome
	if over := labelW + tailW - avail; over > 0 {
		labelW = max(minLabelW, labelW-over)
	}
	if over := labelW + tailW - avail; over > 0 {
		tailW = max(0, tailW-over)
	}
	return max(labelW, minLabelW), tailW, bars
}
