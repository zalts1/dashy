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
	"bytes"
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
	// EditorScheme builds the folder link — `cursor`, `vscode`, `zed` — and is empty when
	// board found no editor, which drops the glyph rather than pointing it at nothing.
	//
	// It sits here beside Threshold rather than on Fleet because it is the same kind of
	// fact: read from ~/.board.json, about the machine and not about the fleet. Fleet stays
	// free of it, so `board` knows nothing about editors (§18).
	EditorScheme string
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
	// Help swaps the bottom line for the legend: the idle scale's rungs, and what each link
	// glyph opens. A display mode like QuietCollapsed rather than a mode that owns input — it
	// does not pause the refresh, because nothing about it goes stale (§9.38).
	Help bool
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
	_, _, todo, quiet := f.Bands()
	keepQuiet, keepTodo := len(quiet), len(todo)
	// A folded band draws no rows, so trimming its tail would shed nothing and the stages
	// below would spin without converging. Zero here makes both quiet stages inert.
	if u.QuietCollapsed {
		keepQuiet = 0
	}
	// The airy form first, whole: a blank line between rows, which on a laptop with room to spare
	// is the difference between a dashboard and a wall of text. Tried before anything is shed
	// because it is a luxury and shedding is not — if the spaced frame does not fit, every row
	// compact beats some rows airy, since the rows are the information and the spacing is not
	// (§9.44).
	if airy := compose(f, s, u, pick(quiet, keepQuiet, u.Sel), pick(todo, keepTodo, u.Sel), true); height(airy) <= s.Rows {
		return airy
	}

	out := ""
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
		out = compose(f, s, u, pick(quiet, keepQuiet, u.Sel), pick(todo, keepTodo, u.Sel), false)
		over := height(out) - s.Rows
		if over <= 0 {
			return out
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
			return clip(out, s.Rows)
		}
	}
	return clip(out, s.Rows)
}

func compose(f board.Fleet, s Screen, u UI, quiet, todo band, airy bool) string {
	// One blank line between rows when the frame can afford it. Between and not after: a band's
	// header already opens with one, so a trailing gap would double it.
	gap := ""
	if airy {
		gap = "\n"
	}
	var b bytes.Buffer
	folders := s.EditorScheme != ""
	gutterW := gutterFor(f)
	labelW, tailW, barW := columns(f, s.Cols, folders)
	actW := actionCols(f.Rows, s.Cols, folders)
	bars := barW > 0
	// The columns between a label and its duration: the bar, the warn mark, and the
	// gaps around them.
	midCols := 1
	if bars {
		midCols = barW + 2
	}
	blocked, working, _, _ := f.Bands()

	b.WriteString(kpiStrip(f, s, len(blocked), len(working)) + "\n")

	// The state mark is left-aligned in the gutter, so it starts one column after the lead and
	// every row's mark is in the same place whatever follows it. It used to be right-aligned to
	// hug the label, which put eleven blank columns in front of a quiet row's ○ — the widest
	// thing in the gutter is the BLOCKED badge, and every other row paid for it (§9.41).
	//
	// The stale glyph rides here rather than out by the duration, because it qualifies the
	// state: `○ ⧗` reads as "quiet, and has been for a while" in one glance, where the same mark
	// three columns from the IDLE value read as a property of the number.
	//
	// Width is passed in because a badge's printed width is not its string length.
	mark := func(s string, width int, stale bool) string {
		out := s
		if stale {
			out += " " + fg(statusWarning, staleGlyph)
			width += 2
		}
		return out + strings.Repeat(" ", max(0, gutterW-width))
	}
	b.WriteString("\n   " + dim(strings.Repeat(" ", gutterW)+pad("LABEL", labelW)+
		strings.Repeat(" ", midCols)+fmt.Sprintf("%*s", idleW, "IDLE")+"  "+
		cut(wsHeader, tailW)) + "\n")

	// when and tail are passed rather than derived, because a todo row states a lifetime
	// and belongs to no workspace, while a session row states a gap and does (§9.19).
	line := func(state, label string, showBar bool, r board.Row, when, tail string) string {
		mid := " "
		if bars {
			// An empty cell, not a missing one: a working row has no bar but still has
			// to line its duration up with everything else.
			barCell := strings.Repeat(" ", barW)
			if showBar {
				barCell = bar(r.Idle, barW)
			}
			mid = " " + barCell + " "
		}
		lead, text := "   ", body(pad(label, labelW))
		if u.Sel != "" && r.Key == u.Sel {
			lead, text = " "+fg(inkPrimary, "▸")+" ", fg(inkPrimary, pad(label, labelW))
		}
		// The workspace name is padded only when something follows it, so a row with nothing
		// to point at — and every todo, which has no process and so no directory (§12) —
		// keeps the shape it had before links existed. actW is the reservation; whether this
		// row spends it is the row's own business.
		tailCell := whereCell(tail, tailW, false)
		if acts := actionCell(r, s.EditorScheme); actW > 0 && acts != "" {
			tailCell = whereCell(tail, tailW, true) + strings.Repeat(" ", actionsGap) + acts
		}
		return lead + state + text + mid +
			body(fmt.Sprintf("%*s", idleW, when)) + "  " + tailCell + "\n"
	}
	// row is the session form of line: idle time as a gap, and the place it lives in.
	row := func(state, label string, showBar bool, r board.Row) string {
		return line(state, label, showBar, r, humanize(r.Idle), r.Where())
	}

	if len(blocked) > 0 {
		b.WriteString("\n  " + fg(statusCritical, "NEEDS YOU") + "\n")
		for i, r := range blocked {
			if i > 0 {
				b.WriteString(gap)
			}
			// Blocked rows carry the bar too: the same quantity on the same absolute
			// scale, so "waiting 3h" is comparable to anything in QUIET.
			b.WriteString(row(mark(badge(inkPrimary, statusCritical, " BLOCKED "), 9, r.Stale), r.Label, true, r))
		}
	} else {
		b.WriteString("\n  " + dim("NEEDS YOU") + "   " + dim("nothing blocked") + "\n")
	}

	if len(working) > 0 {
		b.WriteString("\n  " + fg(statusGood, "WORKING") + " " + dim(fmt.Sprintf("· %d", len(working))) + "\n")
		for i, r := range working {
			if i > 0 {
				b.WriteString(gap)
			}
			// No bar: for a working agent elapsed time is progress, not rot.
			b.WriteString(row(mark(fg(statusGood, workingGlyph), 1, false), r.Label, false, r))
		}
	}

	if total := len(quiet.rows) + quiet.hidden; total > 0 {
		head := "\n  " + fg(inkSecondary, "QUIET") + " " + dim(fmt.Sprintf("· %d", total))
		// Folded, the band costs one line and still states its size. `collapsed` is what
		// separates the two ways rows can be missing: this one the reader chose, the trim
		// below is the terminal's height. Neither may be silent (§9.14).
		if u.QuietCollapsed {
			b.WriteString(head + dim(" · collapsed") + "\n")
		} else {
			b.WriteString(head + "\n")
			for i, r := range quiet.rows {
				if i > 0 {
					b.WriteString(gap)
				}
				b.WriteString(row(mark(dim(quietGlyph), 1, r.Stale), r.Label, true, r))
			}
		}
		// The count stays visible so the backlog can never hide by being collapsed. It is
		// still a count and not a control — the chevron that used to sit here promised a key
		// on the row itself, and the key that exists now is named in the legend where the
		// other keys are, and works from anywhere (§9.14, §9.21).
		//
		// Folded, this line would restate the header's own count as "+13 quiet".
		if quiet.hidden > 0 && !u.QuietCollapsed {
			b.WriteString("   " + strings.Repeat(" ", gutterW) + dim(fmt.Sprintf("+%d quiet", quiet.hidden)) + "\n")
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
		b.WriteString("\n  " + dim("TODO") + "        " + dim("nothing on your list") + "\n")
	}
	if total > 0 {
		cap := ""
		if f.TodoCap > 0 {
			cap = fmt.Sprintf(" of %d", f.TodoCap)
		}
		b.WriteString("\n  " + fg(inkSecondary, "TODO") + " " +
			dim(fmt.Sprintf("· %d%s", total, cap)) + "\n")
		for i, r := range todo.rows {
			if i > 0 {
				b.WriteString(gap)
			}
			// No bar, and no workspace: a bar means rot on the idle scale, and a todo's age
			// is a lifetime that only grows rather than a gap that resets. The mark is an
			// empty box — nothing is running, and nothing has been done.
			b.WriteString(line(mark(fg(inkSecondary, todoGlyph), 1, false), r.Label, false, r, since(r.Idle), ""))
		}
		if todo.hidden > 0 {
			b.WriteString("   " + strings.Repeat(" ", gutterW) + dim(fmt.Sprintf("+%d todo", todo.hidden)) + "\n")
		}
	}

	// A value scale without a key is decoration — and a key without its scale is noise,
	// so the legend goes wherever the bars went.
	b.WriteString("\n  " + bottom(f, u, bars, actW > 0, s.Cols) + "\n")

	// The header is written last and measured against everything below it, so the frame
	// has one right edge instead of two that agree only at 118 columns (§9.29).
	frame := "\n" + header(f, s, u, frameEdge(b.String(), s.Cols, headerEdge(f, s, u))) + "\n\n" + b.String()

	// Belt to the arithmetic's braces: nothing may wrap, whatever the width.
	lines := strings.Split(frame, "\n")
	for i, l := range lines {
		lines[i] = clampLine(l, s.Cols)
	}
	return strings.Join(lines, "\n")
}

// bottom is the frame's last line, and it is one line in every state: the legend plus the keys
// while ambient, the whole legend spelled out under `?`, the capture prompt while typing. Same
// slot, so entering any of them cannot change the height and collapse the quiet tail under the
// reader (§12).
//
// The ambient line names each thing and `?` gives its resolution. That is an amendment to §6's
// "a value scale without a key is decoration": the key is still there — the bar is labelled
// `elapsed` — and only the rung values moved, which buys the width for the two link hints and
// for a legend that can afford to spell out what the glyphs mean (§9.38).
func bottom(f board.Fleet, u UI, bars, links bool, cols int) string {
	if u.Typing {
		// The caret is what proves the mode is on before the first keystroke lands. Checked
		// first because it is the most specific state and the only one holding unsaved text.
		return dim("new todo  ") + fg(inkPrimary, u.Input) + fg(inkPrimary, "▌")
	}
	if u.Help {
		return helpLine(bars, cols)
	}
	legend := ""
	if bars {
		// The dimension, not the scale. A bar you cannot read the value of is still a bar you
		// know is time, and `?` is one keystroke away.
		legend = swatchDim() + dim(" elapsed")
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
	tail := dim("   ctrl-c to exit")
	// The gesture, and only while the cell it describes is on screen — the same condition the
	// cell itself uses, so a tab too narrow for the links cannot advertise it. Last on the
	// line deliberately: it is the newest and most expendable hint, and `ctrl-c to exit` is
	// the one thing that must never be what clipping takes.
	if links {
		tail += dim("   ⌘-click opens")
	}
	return legend + hints + tail + dim("   ? keys")
}

// helpLine is `?`: everything the ambient line abbreviates, spelled out. Every mark the frame
// draws that is not a word — the stale mark, the four link glyphs, and the two shapes the pull
// request takes — plus the scale's rungs, the gesture, and the way back.
//
// All of them whether or not this fleet has any: this is help rather than a status line, and a
// legend that hid a feature nobody happened to be using that minute would answer a different
// question than the one asked.
//
// It **sheds** rather than clips, in a ladder from most complete to least, because a legend cut
// mid-word is worse than a shorter legend and this is the one line a reader opened deliberately.
// What gives way is ordered by what `?` is for: the scale's rungs first, since the ambient line
// already labels the bar; then the gesture, which the README also carries. The glyph meanings
// never give way — they are the question being asked (§9.42).
func helpLine(bars bool, cols int) string {
	glyphs := fg(statusWarning, staleGlyph) + dim(" quiet a while")
	glyphs += "   " + fg(linkStorybook, storybookGlyph) + dim(" storybook")
	glyphs += "  " + fg(linkPreview, previewGlyph) + dim(" preview")
	glyphs += "  " + fg(linkFolder, folderGlyph) + dim(" folder")
	glyphs += "  " + fg(linkPR, prGlyph) + dim(" pr")
	glyphs += "  " + fg(linkPR, prMergedGlyph) + dim(" merged")
	gesture, out := dim("   ⌘-click"), dim("   esc")

	ladder := []string{glyphs + gesture + out, glyphs + out}
	if bars {
		ladder = append([]string{scaleLegend() + "   " + glyphs + gesture + out}, ladder...)
	}
	for _, line := range ladder {
		// headMargin twice: the two columns compose() indents by, and the same right-hand margin
		// every other line keeps.
		if printed(line)+2*headMargin <= cols {
			return line
		}
	}
	// Narrower than the shortest rung. clampLine takes it from here, which is the one case where
	// this line is allowed to be cut: there is nothing left to shed.
	return ladder[len(ladder)-1]
}

// whereCell paints the location column. The repository is dim, like every other piece of
// context on the row, and the worktree inside it takes the preview's green — the branch you are
// on is the part worth catching the eye, and it is the half a reader scanning a fleet of
// worktrees is actually looking for (§18).
//
// The green is reused rather than new. It already means "this is the live thing" on the preview
// glyph, and a worktree is the live branch, so the two readings agree; adding a seventh hue to
// the frame to say the same thing would not (§6, §9.39).
//
// Truncation happens on the plain text before anything is painted, so the column's width is
// the width board measured — and the seam is found in what survived, so a cut that lands inside
// the repository half simply leaves nothing green.
func whereCell(plain string, w int, padded bool) string {
	shown := cut(plain, w)
	fit := func(s string) string {
		if padded {
			return s + strings.Repeat(" ", max(0, w-runes(shown)))
		}
		return s
	}
	i := strings.Index(shown, board.TreeArrow)
	if i < 0 {
		return fit(dim(shown))
	}
	return fit(dim(shown[:i+len(board.TreeArrow)]) + fg(linkPreview, shown[i+len(board.TreeArrow):]))
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
