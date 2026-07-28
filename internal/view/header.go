package view

import (
	"fmt"
	"strings"

	"board/internal/board"
)

// The header is the frame's title bar, and it answers two questions and no others:
// what am I looking at (identity, then the fleet's span) and is what I am looking at
// current (the clock and its cadence, or the mode that replaced them). Everything
// about the fleet's *state* belongs to the KPI strip one line below — a header that
// also reported state would give the frame two headlines competing for the same
// half-second.
//
// The two blocks are pinned to opposite edges. Clumped at the left the line read as a
// dim footnote that happened to be at the top of the screen; split, it reads as a bar,
// and it puts mode changes in the one place the eye already goes to check freshness.

// headMargin matches the indent every other band uses, on both edges: the right block
// stops short of the last column so no terminal can be tempted to wrap it.
const headMargin = 2

func header(f board.Fleet, s Screen, u UI) string {
	room := s.Cols - 2*headMargin
	if room < 6 {
		return ""
	}
	span, notice, mode := spanText(f, true), u.Notice, modeText(s, u, true)

	left := func() string { return strings.TrimRight("BOARD  "+span, " ") }
	right := func() string {
		if notice == "" {
			return mode
		}
		return notice + "   " + mode
	}
	// One space between the blocks is the floor; anything less and they read as one
	// string. over is measured against that floor, so a positive value is real overflow.
	over := func() int { return runes(left()) + 1 + runes(right()) - room }

	// Shed in order of expendability, re-measuring each time. The interval is static
	// config; the workspace span is context; the count and the clock are the point.
	if over() > 0 {
		mode = modeText(s, u, false)
	}
	if over() > 0 {
		span = spanText(f, false)
	}
	if over() > 0 && notice != "" {
		notice = cut(notice, runes(notice)-over())
	}
	if over() > 0 {
		span = cut(span, runes(span)-over())
	}
	if over() > 0 {
		mode = cut(mode, runes(mode)-over())
	}

	// Measured on the plain text, then painted: an escape code costs no columns, so
	// sizing the painted strings would overstate every width fivefold.
	gap := room - runes(left()) - runes(right())

	leftPainted := fg(inkPrimary, "BOARD")
	if span != "" {
		leftPainted += "  " + body(span)
	}
	rightPainted := paintMode(u, mode)
	if notice != "" {
		rightPainted = fg(statusWarning, notice) + "   " + rightPainted
	}
	return strings.Repeat(" ", headMargin) + leftPainted +
		strings.Repeat(" ", max(gap, 1)) + rightPainted
}

// spanText says how much work there is and how far it is spread. The workspace count
// is the one fact no band below carries; sessions with no tab have no workspace, so a
// fleet of only background agents has no span to state.
func spanText(f board.Fleet, withSpan bool) string {
	if len(f.Rows) == 0 {
		return "no sessions"
	}
	s := fmt.Sprintf("%d %s", len(f.Rows), plural(len(f.Rows), "session"))
	if withSpan && f.Workspaces > 0 {
		s += fmt.Sprintf(" in %d %s", f.Workspaces, plural(f.Workspaces, "workspace"))
	}
	return s
}

// modeText is the freshness block. While a selection is live the data stops
// refreshing, so the clock would be stating a time that is no longer true — the mode
// takes its place rather than sitting beside it.
func modeText(s Screen, u UI, withInterval bool) string {
	if u.Paused {
		return "paused · esc to resume"
	}
	// Seconds, not minutes: at a 10s cadence a minute-precision clock cannot be told
	// apart from a frozen one, which is the only thing this element is here to prove.
	t := s.Now.Format("15:04:05")
	if withInterval {
		t += " · every " + short(s.Interval)
	}
	return t
}

func paintMode(u UI, mode string) string {
	if u.Paused {
		return fg(statusWarning, mode)
	}
	return dim(mode)
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
