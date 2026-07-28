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

	"board/internal/board"
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
	Sel    string // selected surface id, "" when ambient
	Paused bool
	Notice string // shown in the header; the board tab may be hidden when it appears
}

// Frame renders the screen so that it always fits the terminal.
//
// Fit is measured, not estimated. It used to be a hard-coded count of chrome lines,
// which was two short: the frame overflowed, the terminal scrolled, and the header —
// the first thing written — was the first thing lost. Rendering and measuring is
// cheap, and it cannot drift out of step with the layout the way a constant did.
func Frame(f board.Fleet, s Screen, u UI) string {
	_, _, quiet := f.Bands()
	keepQuiet := len(quiet)
	out := ""
	// The quiet tail is the designed absorber, so it gives way first. Each pass cuts by
	// the exact overflow, so this converges in two or three.
	for range 6 {
		shown, hidden := pickQuiet(quiet, keepQuiet, u.Sel)
		out = compose(f, s, u, shown, hidden)
		over := height(out) - s.Rows
		switch {
		case over <= 0:
			return out
		case keepQuiet > minQuietRows:
			keepQuiet = max(minQuietRows, keepQuiet-over)
		default:
			// A tab too short even for the floor. Cut the bottom rather than let the
			// terminal cut the top: losing the legend costs a key, losing the header
			// costs the clock and the blocked count.
			return clip(out, s.Rows)
		}
	}
	return clip(out, s.Rows)
}

func compose(f board.Fleet, s Screen, u UI, quiet []board.Row, hiddenQuiet int) string {
	var b bytes.Buffer
	labelW, tailW, bars := columns(f, s.Cols)
	// The columns between a label and its duration: the bar, the warn mark, and the
	// gaps around them.
	midCols := 3
	if bars {
		midCols = barCells + 4
	}
	blocked, working, _ := f.Bands()

	b.WriteString("\n" + header(f, s, u) + "\n\n")

	b.WriteString(kpiStrip(f, s, len(blocked), len(working)) + "\n")

	// The state mark is right-aligned inside the gutter so it hugs the label
	// instead of leaving a gap. Width is passed in because a badge's printed width
	// is not its string length.
	mark := func(s string, width int) string {
		return strings.Repeat(" ", gutter-width-1) + s + " "
	}
	b.WriteString("\n   " + dim(strings.Repeat(" ", gutter)+pad("LABEL", labelW)+
		strings.Repeat(" ", midCols)+fmt.Sprintf("%*s", idleW, "IDLE")+"  "+
		cut(wsHeader, tailW)) + "\n")

	line := func(state, label string, showBar bool, r board.Row) string {
		warn := " "
		if r.Stale {
			warn = fg(statusWarning, "⚠")
		}
		mid := " " + warn + " "
		if bars {
			// An empty cell, not a missing one: a working row has no bar but still has
			// to line its duration up with everything else.
			barCell := strings.Repeat(" ", barCells)
			if showBar {
				barCell = bar(r.Idle)
			}
			mid = " " + barCell + " " + warn + " "
		}
		lead, text := "   ", body(pad(label, labelW))
		if u.Sel != "" && r.Surface == u.Sel {
			lead, text = " "+fg(inkPrimary, "▸")+" ", fg(inkPrimary, pad(label, labelW))
		}
		return lead + state + text + mid +
			body(fmt.Sprintf("%*s", idleW, humanize(r.Idle))) + "  " +
			dim(cut(r.Workspace, tailW)) + "\n"
	}

	if len(blocked) > 0 {
		b.WriteString("\n  " + fg(statusCritical, "NEEDS YOU") + "\n")
		for _, r := range blocked {
			// Blocked rows carry the bar too: the same quantity on the same absolute
			// scale, so "waiting 3h" is comparable to anything in QUIET.
			b.WriteString(line(mark(badge(inkPrimary, statusCritical, " BLOCKED "), 9), r.Label, true, r))
		}
	} else {
		b.WriteString("\n  " + dim("NEEDS YOU") + "   " + dim("nothing blocked") + "\n")
	}

	if len(working) > 0 {
		b.WriteString("\n  " + fg(statusGood, "WORKING") + " " + dim(fmt.Sprintf("· %d", len(working))) + "\n")
		for _, r := range working {
			// No bar: for a working agent elapsed time is progress, not rot.
			b.WriteString(line(mark(fg(statusGood, "◐"), 1), r.Label, false, r))
		}
	}

	if total := len(quiet) + hiddenQuiet; total > 0 {
		b.WriteString("\n  " + fg(inkSecondary, "QUIET") + " " + dim(fmt.Sprintf("· %d", total)) + "\n")
		for _, r := range quiet {
			b.WriteString(line(mark(dim("○"), 1), r.Label, true, r))
		}
		// The count stays visible so the backlog can never hide by being collapsed.
		if hiddenQuiet > 0 {
			b.WriteString("   " + strings.Repeat(" ", gutter) + dim(fmt.Sprintf("⌄  %d more", hiddenQuiet)) + "\n")
		}
	}

	// A value scale without a key is decoration — and a key without its scale is noise,
	// so the legend goes wherever the bars went.
	legend := ""
	if bars {
		legend = dim("elapsed ") + scaleLegend()
	}
	b.WriteString("\n  " + legend + dim("   ctrl-c to exit") + "\n")

	// Belt to the arithmetic's braces: nothing may wrap, whatever the width.
	lines := strings.Split(b.String(), "\n")
	for i, l := range lines {
		lines[i] = clampLine(l, s.Cols)
	}
	return strings.Join(lines, "\n")
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
