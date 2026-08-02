package view

// Frame is the ambient dashboard's single screen. It is a pure function of the
// snapshot plus the terminal size and the transient selection, which is what makes
// every layout decision below testable without a terminal to drive.
//
// Form follows the data-viz method: >7 classes that all carry meaning is a table,
// not a chart, and the right form for "one thing matters, the rest are context" is
// emphasis — one accent, everything else recessive. So exactly one element is
// allowed to shout, and it is BLOCKED.

import (
	"fmt"
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// Screen is where and when the frame is being drawn.
type Screen struct {
	Now       time.Time
	Interval  time.Duration
	Threshold time.Duration
	Rows      int
	Cols      int
}

// UI is the transient interaction state, kept separate from the fleet snapshot
// because it belongs to the viewer, not to the fleet.
type UI struct {
	Sel    string // selected row key (the session id), "" when ambient
	Paused bool
	Notice string // shown in the header; the board tab may be hidden when it appears
	// Typing is the one mode this UI has: capturing a todo. Input is what has been
	// typed so far. It occupies the legend's line rather than a new one, so entering the
	// mode cannot change the frame's height (§12).
	Typing bool
	Input  string
	// QuietCollapsed folds the quiet band to its count. It defaults to false — the band
	// starts open, and folding is something the viewer asks for. Distinct from the fit
	// loop's trim, which is the terminal's height talking, not the reader (§9.21).
	QuietCollapsed bool
}

// Frame renders the screen so that it always fits the terminal.
//
// Fit is measured, not estimated. It used to be a hard-coded count of chrome lines,
// which was two short: the frame overflowed, the terminal scrolled, and the header —
// the first thing written — was the first thing lost. Rendering and measuring is
// cheap, and it cannot drift out of step with the layout the way a constant did.
func Frame(f board.Fleet, s Screen, u UI) string {
	out, _ := FrameHits(f, s, u)
	return out
}

// FrameHits is Frame plus the hit map: one entry per screen line, holding the key of
// the row drawn there and empty for every line of chrome. It exists so a click can be
// resolved to a session, and it comes out of the drawing pass rather than being derived
// beside it — a hit map computed separately is a second copy of the layout, and it
// drifts the first time a band moves (§3, §7).
func FrameHits(f board.Fleet, s Screen, u UI) (string, []string) {
	_, _, todo, quiet := f.Bands()
	keepQuiet, keepTodo := len(quiet), len(todo)
	// A folded band draws no rows, so trimming its tail would shed nothing and the stages
	// below would spin without converging. Zero here makes both quiet stages inert.
	if u.QuietCollapsed {
		keepQuiet = 0
	}
	out, hits := "", []string(nil)
	// Absorbers in order of expendability, each cutting by the exact overflow so a stage
	// converges in one pass: the quiet tail to its floor, then the list to its own, then
	// either of them to nothing. **A band collapsed to its count still reports** — "QUIET
	// · 14" with no rows says what matters — so shedding rows always beats letting clip
	// take a whole band off the bottom.
	//
	// The list earns its position this way rather than by hiding from the collapse. It
	// used to be drawn above QUIET purely so the trim could not reach it, which put it
	// inside a ranking of process states it is not part of (EVIDENCE.md §9.19).
	stages := []struct {
		keep  *int
		floor int
	}{
		{&keepQuiet, minQuietRows}, {&keepTodo, minTodoRows}, {&keepQuiet, 0}, {&keepTodo, 0},
	}
	for range 10 {
		out, hits = compose(f, s, u, pick(quiet, keepQuiet, u.Sel), pick(todo, keepTodo, u.Sel))
		over := height(out) - s.Rows
		if over <= 0 {
			return out, hits
		}
		shrunk := false
		for _, st := range stages {
			if *st.keep > st.floor {
				*st.keep, shrunk = max(st.floor, *st.keep-over), true
				break
			}
		}
		if !shrunk {
			// A tab too short even for two count lines. Cut the bottom rather than let the
			// terminal cut the top: losing the legend costs a key, losing the header costs
			// the clock and the blocked count.
			return clip(out, s.Rows), clipHits(hits, s.Rows-1)
		}
	}
	return clip(out, s.Rows), clipHits(hits, s.Rows-1)
}

func compose(f board.Fleet, s Screen, u UI, quiet, todo band) (string, []string) {
	p := &painter{}
	b := &p.b
	labelW, tailW, barW := columns(f, s.Cols)
	bars := barW > 0
	// The columns between a label and its duration: the bar, the warn mark, and the
	// gaps around them.
	midCols := 3
	if bars {
		midCols = barW + 4
	}
	blocked, working, _, _ := f.Bands()

	p.put(kpiStrip(f, s, len(blocked), len(working)) + "\n")

	// The state mark is right-aligned inside the gutter so it hugs the label
	// instead of leaving a gap. Width is passed in because a badge's printed width
	// is not its string length.
	mark := func(s string, width int) string {
		return strings.Repeat(" ", gutter-width-1) + s + " "
	}
	p.put("\n   " + dim(strings.Repeat(" ", gutter)+pad("LABEL", labelW)+
		strings.Repeat(" ", midCols)+fmt.Sprintf("%*s", idleW, "IDLE")+"  "+
		cut(wsHeader, tailW)) + "\n")

	// when and tail are passed rather than derived, because a todo row states a lifetime
	// and belongs to no workspace, while a session row states a gap and does (§9.19).
	line := func(state, label string, showBar bool, r board.Row, when, tail string) string {
		warn := " "
		if r.Stale {
			warn = fg(statusWarning, "⚠")
		}
		mid := " " + warn + " "
		if bars {
			// An empty cell, not a missing one: a working row has no bar but still has
			// to line its duration up with everything else.
			barCell := strings.Repeat(" ", barW)
			if showBar {
				barCell = bar(r.Idle, barW)
			}
			mid = " " + barCell + " " + warn + " "
		}
		lead, text := "   ", body(pad(label, labelW))
		if u.Sel != "" && r.Key == u.Sel {
			lead, text = " "+fg(inkPrimary, "▸")+" ", fg(inkPrimary, pad(label, labelW))
		}
		return lead + state + text + mid +
			body(fmt.Sprintf("%*s", idleW, when)) + "  " +
			dim(cut(tail, tailW)) + "\n"
	}
	// row is the session form of line: idle time as a gap, and the workspace it lives in.
	row := func(state, label string, showBar bool, r board.Row) string {
		return line(state, label, showBar, r, humanize(r.Idle), r.Workspace)
	}

	if len(blocked) > 0 {
		p.put("\n  " + fg(statusCritical, "NEEDS YOU") + "\n")
		for _, r := range blocked {
			// Blocked rows carry the bar too: the same quantity on the same absolute
			// scale, so "waiting 3h" is comparable to anything in QUIET.
			p.row(r.Key, row(mark(badge(inkPrimary, statusCritical, " BLOCKED "), 9), r.Label, true, r))
		}
	} else {
		p.put("\n  " + dim("NEEDS YOU") + "   " + dim("nothing blocked") + "\n")
	}

	if len(working) > 0 {
		p.put("\n  " + fg(statusGood, "WORKING") + " " + dim(fmt.Sprintf("· %d", len(working))) + "\n")
		for _, r := range working {
			// No bar: for a working agent elapsed time is progress, not rot.
			p.row(r.Key, row(mark(fg(statusGood, "◐"), 1), r.Label, false, r))
		}
	}

	if total := len(quiet.rows) + quiet.hidden; total > 0 {
		head := "\n  " + fg(inkSecondary, "QUIET") + " " + dim(fmt.Sprintf("· %d", total))
		// Folded, the band costs one line and still states its size. `collapsed` is what
		// separates the two ways rows can be missing: this one the reader chose, the trim
		// below is the terminal's height. Neither may be silent (§9.14).
		if u.QuietCollapsed {
			p.put(head + dim(" · collapsed") + "\n")
		} else {
			p.put(head + "\n")
			for _, r := range quiet.rows {
				p.row(r.Key, row(mark(dim("○"), 1), r.Label, true, r))
			}
		}
		// The count stays visible so the backlog can never hide by being collapsed. It is
		// still a count and not a control — the chevron that used to sit here promised a key
		// on the row itself, and the key that exists now is named in the legend where the
		// other keys are, and works from anywhere (§9.14, §9.21).
		//
		// Folded, this line would restate the header's own count as "+13 quiet".
		if quiet.hidden > 0 && !u.QuietCollapsed {
			p.put("   " + strings.Repeat(" ", gutter) + dim(fmt.Sprintf("+%d quiet", quiet.hidden)) + "\n")
		}
	}

	// Last, after the whole fleet: a todo is not a state a session can be in, and drawing
	// it between the bands put it inside a ranking of process states (§9.19). It survives
	// a short tab by having a floor of its own, not by sitting where the collapse cannot
	// reach.
	total := len(todo.rows) + todo.hidden
	// The one band drawn with nothing in it. It costs two lines and reports no exception,
	// which §9.13 argues against — but an unused feature that renders nothing is a feature
	// nobody discovers, and the empty form is how you learn the list exists. NEEDS YOU
	// carries the same line for a different reason (§12).
	if total == 0 {
		p.put("\n  " + dim("TODO") + "        " + dim("nothing on your list") + "\n")
	}
	if total > 0 {
		cap := ""
		if f.TodoCap > 0 {
			cap = fmt.Sprintf(" of %d", f.TodoCap)
		}
		p.put("\n  " + fg(inkSecondary, "TODO") + " " +
			dim(fmt.Sprintf("· %d%s", total, cap)) + "\n")
		for _, r := range todo.rows {
			// No bar, and no workspace: a bar means rot on the idle scale, and a todo's age
			// is a lifetime that only grows rather than a gap that resets. The mark is an
			// empty box — nothing is running, and nothing has been done.
			p.row(r.Key, line(mark(fg(inkSecondary, "▫"), 1), r.Label, false, r, since(r.Idle), ""))
		}
		if todo.hidden > 0 {
			p.put("   " + strings.Repeat(" ", gutter) + dim(fmt.Sprintf("+%d todo", todo.hidden)) + "\n")
		}
	}

	// A value scale without a key is decoration — and a key without its scale is noise,
	// so the legend goes wherever the bars went.
	p.put("\n  " + bottom(f, u, bars) + "\n")

	// The header is written last and measured against everything below it, so the frame
	// has one right edge instead of two that agree only at 118 columns (§9.29).
	head := "\n" + header(f, s, u, frameEdge(b.String(), s.Cols, headerEdge(f, s, u))) + "\n\n"
	frame := head + b.String()

	// Belt to the arithmetic's braces: nothing may wrap, whatever the width.
	lines := strings.Split(frame, "\n")
	for i, l := range lines {
		lines[i] = clampLine(l, s.Cols)
	}
	// The body's hits shift down by whatever the header prefix occupies — counted from
	// the prefix itself, so a header that ever grows a line cannot silently offset every
	// click by one row.
	return strings.Join(lines, "\n"),
		append(make([]string, strings.Count(head, "\n")), p.hits...)
}

// bottom is the frame's last line, and it is one line in every state: the scale legend
// plus the keys while ambient, the capture prompt while typing. Same slot, so entering
// the mode cannot change the height and collapse the quiet tail under the typist (§12).
//
// A value scale without a key is decoration — and a key without its scale is noise, so
// the legend goes wherever the bars went.
func bottom(f board.Fleet, u UI, bars bool) string {
	if u.Typing {
		// The caret is what proves the mode is on before the first keystroke lands.
		return dim("new todo  ") + fg(inkPrimary, u.Input) + fg(inkPrimary, "▌")
	}
	legend := ""
	if bars {
		legend = dim("elapsed ") + scaleLegend()
	}
	// `a` always works, so it is always named. `d` fires on a selected todo and nowhere
	// else, so it is named only there: §9.14 was a chevron promising a key that did not
	// exist, and the same rule read forwards means an available key should say so.
	hints := dim("   a new todo")
	if FirstTodo(f) != "" {
		hints += dim("   t list")
	}
	// Named only when there is a band to fold, and named in both states: once the rows are
	// folded away this is the only route back to them (§9.14).
	if _, _, _, quiet := f.Bands(); len(quiet) > 0 {
		if u.QuietCollapsed {
			hints += dim("   z unfold")
		} else {
			hints += dim("   z fold")
		}
	}
	if r, ok := f.ByKey(u.Sel); ok {
		if _, isTodo := r.TodoID(); isTodo {
			hints += dim("   d done")
		}
	}
	return legend + hints + dim("   ctrl-c to exit")
}

// kpiStrip is the sub-second read, and blocked is the only thing in it allowed to
// shout. Cells are shed whole from the right when the window is too narrow — cut
// mid-word, "oldest 52d06h" becomes a stray "ol" that reads as a rendering fault.
func kpiStrip(f board.Fleet, s Screen, blocked, working int) string {
	blockedCell := dim(fmt.Sprintf("%d blocked", blocked))
	if blocked > 0 {
		blockedCell = badge(inkPrimary, statusCritical, fmt.Sprintf(" %d BLOCKED ", blocked))
	}
	// Plain text alongside the painted cell: an escape code costs no columns.
	cells := []struct{ plain, painted string }{
		{fmt.Sprintf(" %d BLOCKED ", blocked), blockedCell},
		{fmt.Sprintf("%d working", working), body(fmt.Sprintf("%d working", working))},
		{fmt.Sprintf("%d quiet >%s", f.Stale, short(s.Threshold)),
			body(fmt.Sprintf("%d quiet >%s", f.Stale, short(s.Threshold)))},
		{"oldest " + humanize(f.Oldest), dim("oldest ") + body(humanize(f.Oldest))},
	}
	if blocked == 0 {
		cells[0].plain = fmt.Sprintf("%d blocked", blocked)
	}
	out, w := "  ", headMargin
	for i, c := range cells {
		if i > 0 {
			if w+5+runes(c.plain) > s.Cols-headMargin {
				break
			}
			out, w = out+dim("     "), w+5
		}
		out, w = out+c.painted, w+runes(c.plain)
	}
	return out
}
