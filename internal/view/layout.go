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
	// gutterBlocked is the state-mark column when a blocked row is on screen: the BLOCKED badge
	// is 9 printed columns and needs one after it. gutterPlain is what every other fleet needs
	// — a one-column mark, a space, the stale glyph, a space.
	//
	// Elastic rather than always the badge's width, because a fleet with nothing blocked is the
	// ordinary case and it was paying six columns for a badge that was not there (§9.41). The
	// cost is that the table shifts right when something blocks — accepted, because a row
	// entering NEEDS YOU already moves every band on the screen, and the six columns are the
	// label's the rest of the time.
	// Both include the stale glyph and a trailing space, because a blocked row can be stale too
	// — a question unanswered for three hours is exactly what that mark is for (§4) — and a
	// gutter sized without it makes the blocked row one column wider than every other, which is
	// the alignment §9.29 is about.
	gutterBlocked = 12 // " BLOCKED " + " ⧗" + one space
	gutterPlain   = 4  // "○" + " ⧗" + one space
	idleW         = 7  // the IDLE column, widest at "52d06h"
	maxLabelW     = 80
	minLabelW     = 18
	// rowChromeBare is every column a row spends outside the label, the tail, the bar and the
	// gutter: the lead, the IDLE column, and the gaps between them. Derived from the pieces
	// rather than guessed — the fixed reserve of 46 it replaces was short by the whole tail, so
	// any long name wrapped the row (EVIDENCE.md §9.12). The gutter is added by rowChrome, since
	// it is a function of the fleet now and no longer a constant.
	rowChromeBare = 3 + 1 + idleW + 2
	// wsHeader names the location column, and it is the tail column's floor too: a column keeps
	// room for its own label. It was `WORKSPACE` until §9.39 and then `REPO`, and it is `WHERE`
	// now: the cell shows `repo -> worktree` as often as a bare repository, and both are answers
	// to *where*, which `REPO` was only half of (§19).
	wsHeader = "WHERE"
	// sessionHeader names the label column. `LABEL` was implementation vocabulary — what the
	// column holds is the session, which is what the reader is looking for (§19).
	sessionHeader = "SESSION"
	// relatedHeader names the link cell. It had no name at all, which left four glyphs at the
	// right-hand end of every row belonging to no column (§19).
	relatedHeader = "RELATED"
	// The quiet tail never shrinks below this: a QUIET band of one row reads as a
	// quiet fleet, which is the opposite of the truth.
	minQuietRows = 3
	// actionsW is the trailing link cell: four glyphs on stable columns, with actionsSpace between
	// each, so a row carrying only one of them leaves the others' columns empty rather than sliding
	// into them. Four because a row points at four things — the Storybook listening in its
	// worktree, the dev server serving it, the worktree itself, and the pull request its branch has
	// open (§18).
	//
	// Two columns between glyphs rather than one. They are all drawn from one Unicode block and
	// most of them are boxes, so at a terminal's cell width a single space left them reading as one
	// run of ink rather than four marks — the same reason the KPI strip uses five (§9.44).
	actionsSpace = 2
	actionsW     = 4 + 3*actionsSpace
	// actionsGap separates the cell from the workspace name, which is left-aligned and
	// truncating: one space would read as part of the name it follows.
	actionsGap = 2
	// The todo list keeps fewer, because it gives way second and the count carries the
	// rest — but never none: a list collapsed to a header is a reminder that stopped
	// reminding (§12).
	minTodoRows = 2
)

// gutterFor is the width of the state-mark column on this fleet: the badge's width when a row is
// blocked, and the narrow form otherwise. The one place that decides, read by both the row
// renderer and the arithmetic (§9.41).
func gutterFor(f board.Fleet) int {
	for _, r := range f.Rows {
		if r.Rank == board.RankBlocked {
			return gutterBlocked
		}
	}
	return gutterPlain
}

// rowChrome is rowChromeBare plus the fleet's gutter, plus a bar of barW cells and the space
// before it. A barW of 0 is the bare row a tab too narrow for a whole bar gets: a cut bar is
// worse than no bar, the same glyph run reporting a smaller number on an absolute scale.
func rowChrome(barW, gutterW int) int {
	if barW == 0 {
		return rowChromeBare + gutterW
	}
	return rowChromeBare + gutterW + barW + 1
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
// Cutting from the end means anything hidden belongs below everything shown, which is what keeps
// the `+N` count honest. What lands at the end differs by band since §9.46: todos are still
// oldest-first, so the tail is the least reproachful; **sessions are newest-first, so the tail is
// the most neglected** and a short tab now hides the rows that most want attention. minQuietRows is
// the floor that limits it, and the header's `oldest 2d22h` plus the strip's `N quiet >45m` are what
// stop it being silent.
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
// folders says whether a folder link can be built at all — board found an editor. Passed as
// a fact about the machine rather than as the scheme itself, because the arithmetic has no
// business knowing what a URL looks like; `pointsSomewhere` is the one predicate both this
// and the renderer answer, and a test pins them to the same answer.
func actionCols(rows []board.Row, cols int, folders bool) int {
	linked := false
	for _, r := range rows {
		if pointsSomewhere(r, folders) {
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
	if rowChromeBare+gutterPlain+minLabelW+runes(wsHeader)+actionsGap+actionsW > cols-headMargin {
		return 0
	}
	return actionsGap + actionsW
}

// pointsSomewhere reports whether this row would draw a link cell. A row's folder counts
// only when there is an editor to open it in: without one the glyph would point at nothing,
// so the column must not be reserved for it either (§18).
func pointsSomewhere(r board.Row, folders bool) bool {
	return r.Preview != "" || r.Storybook != "" || r.PR != "" || (folders && r.Folder != "")
}

// columns sizes the row's three elastic columns: the label, the tail — the workspace,
// unbounded in the data and therefore the thing that used to overflow — and the bar.
//
// Order is meaning first, in both directions. Squeezing, the label gives way to its
// floor and only then does the tail truncate. Spending, the label is filled out whole
// before anything else, and the bar takes what is left over — which is where the surplus
// belongs, because the gap it closes is the one between a label and its bar (§9.29).
func columns(f board.Fleet, cols int, folders bool) (labelW, tailW, barW int) {
	gutterW := gutterFor(f)
	whole := 0
	// A grouped row's label starts groupIndent columns further in, so the column has to be that
	// much wider or it truncates the label it was sized for (§19).
	grouped := f.Grouped()
	for _, r := range f.Rows {
		w := runes(r.Label)
		if grouped[r.Key] {
			w += groupIndent
		}
		whole = max(whole, w)
		// Where and not Workspace: the column prints a repository and the worktree inside it,
		// and sizing on the field it no longer draws is how a column comes to truncate text it
		// had room for (§9.39).
		tailW = max(tailW, runes(r.Where()))
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
	act := actionCols(f.Rows, cols, folders)
	if cols-headMargin-act < rowChrome(barW, gutterW)+minLabelW {
		barW = 0
	}
	avail := cols - headMargin - rowChrome(barW, gutterW) - act

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
