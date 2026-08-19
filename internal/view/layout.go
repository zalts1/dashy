package view

// Layout is the frame's arithmetic: how many lines it may occupy, how wide each
// elastic column is, and what gives way first when the terminal is too small for
// everything. It is kept apart from the rendering so the sizes can be asserted
// directly — every fit bug in EVIDENCE.md §9.10 and §9.12 was an arithmetic bug that
// looked correct in the rendering code.

import (
	"math"
	"slices"
	"strings"

	"github.com/zalts1/dashy/internal/board"
)

// Layout constants shared by every band so labels line up on one left edge.
const (
	gutter    = 10 // width of the state-mark column
	idleW     = 7  // the IDLE column, widest at "52d06h"
	maxLabelW = 80
	minLabelW = 18
	// rowChromeBare is every column a row spends outside the label, the tail and the bar:
	// the lead, the state gutter, the warn mark, the IDLE column, and the gaps between
	// them. Derived from the pieces rather than guessed — the fixed reserve of 46 it
	// replaces was short by the whole tail, so any long workspace name wrapped the row
	// (EVIDENCE.md §9.12).
	rowChromeBare = 3 + gutter + 3 + idleW + 2
	wsHeader      = "WORKSPACE" // also the tail column's floor: a column keeps room for its label
	// The quiet tail never shrinks below this: a QUIET band of one row reads as a
	// quiet fleet, which is the opposite of the truth.
	minQuietRows = 3
	// actionsW is the trailing link cell: two glyphs on stable columns with a space between
	// them, so a row carrying only one of them leaves the other's column empty rather than
	// sliding into it. Fixed at two, because a row points at exactly two things — the
	// preview serving its worktree, and the worktree itself (§18).
	actionsW = 3
	// actionsGap separates the cell from the workspace name, which is left-aligned and
	// truncating: one space would read as part of the name it follows.
	actionsGap = 2
	// The todo list keeps fewer, because it gives way second and the count carries the
	// rest — but never none: a list collapsed to a header is a reminder that stopped
	// reminding (§12).
	minTodoRows = 2
)

// rowChrome is rowChromeBare plus a bar of barW cells and the space before it. A barW
// of 0 is the bare row a tab too narrow for a whole bar gets: a cut bar is worse than
// no bar, the same glyph run reporting a smaller number on an absolute scale.
func rowChrome(barW int) int {
	if barW == 0 {
		return rowChromeBare
	}
	return rowChromeBare + barW + 1
}

// height is how many terminal lines this frame occupies once written. The watch loop
// splits on newlines and writes each line with a trailing one, so the final newline
// moves the cursor one row past the last line — and a terminal scrolls if that lands
// past the bottom. Everything above the fold goes with it, starting with the header.
func height(frame string) int { return strings.Count(frame, "\n") + 2 }

// band is a collapsible group: the rows to draw, and how many are not drawn. The count
// is what keeps a collapse honest — nothing may disappear silently (§9.13, §9.14).
type band struct {
	rows   []board.Row
	hidden int
}

// pick keeps the first n rows, plus the selected one wherever it sits, and reports how
// many are hidden. It copies rather than reslicing: the caller reuses the group across
// passes.
//
// Both collapsible groups are sorted worst-first — quiet by idle descending, todos by
// age — so cutting from the end always drops the least reproachful, and anything hidden
// belongs below everything shown.
func pick(rows []board.Row, n int, sel string) band {
	if n >= len(rows) {
		return band{rows: rows}
	}
	n = max(n, 0)
	shown := make([]board.Row, n, n+1)
	copy(shown, rows[:n])
	// A selection must stay visible even if it sits in a collapsed group: short of a
	// taller tab it is the only way to read a hidden row (§9.14).
	for _, r := range rows[n:] {
		if sel != "" && r.Key == sel {
			shown = append(shown, r)
			break
		}
	}
	return band{rows: shown, hidden: len(rows) - len(shown)}
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

// actionCols is what the trailing link cell costs on this fleet at this width, and it is
// the one place that decides — both the arithmetic and the rendering read it, so they
// cannot disagree about whether the column exists.
//
// Nothing at all when no row has anything to point at: a fleet with no previews and no
// resolvable folders renders the frame it did before links existed, escape for escape.
// And nothing again when the terminal cannot hold a bare row beside it — shed whole, like
// the KPI strip's cells, because half a link cell is a glyph that no longer lines up with
// the one above it and that reads as a rendering fault rather than as an absent link (§18).
func actionCols(rows []board.Row, cols int) int {
	linked := false
	for _, r := range rows {
		if r.Preview != "" || r.Folder != "" {
			linked = true
			break
		}
	}
	if !linked {
		return 0
	}
	// The links come last, after both floors: a row that cannot hold its label and name its
	// workspace has nothing to spare, and losing either of those to gain a glyph would be
	// the wrong trade in a tool whose first job is to be read.
	if rowChromeBare+minLabelW+runes(wsHeader)+actionsGap+actionsW > cols-headMargin {
		return 0
	}
	return actionsGap + actionsW
}

// columns sizes the row's three elastic columns: the label, the tail — the workspace,
// unbounded in the data and therefore the thing that used to overflow — and the bar.
//
// Order is meaning first, in both directions. Squeezing, the label gives way to its
// floor and only then does the tail truncate. Spending, the label is filled out whole
// before anything else, and the bar takes what is left over — which is where the surplus
// belongs, because the gap it closes is the one between a label and its bar (§9.29).
func columns(f board.Fleet, cols int) (labelW, tailW, barW int) {
	whole := 0
	for _, r := range f.Rows {
		whole = max(whole, runes(r.Label))
		tailW = max(tailW, runes(r.Workspace))
	}
	whole = min(whole, maxLabelW)
	tailW = max(tailW, runes(wsHeader))

	// A bar at its base width is what decides whether there is a bar at all — a wider one
	// is a bonus and may not be what keeps a narrow tab from having one. headMargin keeps
	// the same right-hand margin the header keeps, so the table's right edge and the
	// header's are the same column.
	barW = barCells
	// Reserved before anything elastic is sized, and taken off avail rather than out of a
	// column: the cell is a fixed width at the row's right-hand end, so every column left
	// of it is sized inside what remains.
	act := actionCols(f.Rows, cols)
	if cols-headMargin-act < rowChrome(barW)+minLabelW {
		barW = 0
	}
	avail := cols - headMargin - rowChrome(barW) - act

	// The whole label while the row can hold it, the p90 once it cannot: a column that
	// truncates on a window with columns to spare is choosing to lose text it could have
	// shown, and the outlier it is guarding against only costs anything when space is
	// scarce (§9.29).
	labelW = whole
	if labelW+tailW > avail {
		labelW = min(labelColumn(f.Rows), maxLabelW)
	}
	if over := labelW + tailW - avail; over > 0 {
		labelW = max(minLabelW, labelW-over)
	}
	if over := labelW + tailW - avail; over > 0 {
		tailW = max(0, tailW-over)
	}
	labelW = max(labelW, minLabelW)

	// Whatever the two of them left goes to the bar, so the row ends on the frame's right
	// edge instead of stopping short of it — up to barMaxCells, past which a wider bar
	// stops being more readable and starts being the row's loudest mark. Beyond that the
	// surplus is margin: a left-aligned tail cannot put ink on the right edge, so the bar
	// is the only column that could have spent it.
	if barW > 0 {
		barW = min(barMaxCells, barW+max(0, avail-labelW-tailW))
	}
	return labelW, tailW, barW
}

// labelColumn is the width the labels ask for: the p90 of those present, not the
// longest. The bar column is shared — bars encode one absolute scale, so they have to
// start on one column — which meant a single long title pushed every other row's bar
// across a corridor of padding to reach it. p90 keeps a fleet of uniformly long labels
// whole and spends the ellipsis only on the tail that is out of step with the rest.
func labelColumn(rows []board.Row) int {
	if len(rows) == 0 {
		return 0
	}
	w := make([]int, len(rows))
	for i, r := range rows {
		w[i] = runes(r.Label)
	}
	slices.Sort(w)
	return w[int(math.Round(0.9*float64(len(w)-1)))]
}

// frameEdge is the column the header's right block lands on: the frame's own right
// edge, not the terminal's. Bounded by the terminal because a wrapped header scrolls
// the whole frame away (EVIDENCE.md §9.10), and floored by what the header itself has
// to say, so a fleet too small to fill the line never costs the reader the clock.
func frameEdge(body string, cols, floor int) int {
	w := 0
	for l := range strings.SplitSeq(body, "\n") {
		w = max(w, printed(l))
	}
	return min(cols-headMargin, max(w, floor))
}
