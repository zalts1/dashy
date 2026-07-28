package view

import "board/internal/board"

// Navigation lives with the renderer because it must follow the screen, not the
// data. Two rules taken from prior art rather than invented: selection is keyed on
// the session's surface id, never a row index (htop's Follow key exists because
// index-based selection drifts when the list re-sorts under you), and the caller
// pauses the refresh while a selection is live (less's +F makes the
// streaming/interacting boundary explicit instead of clever).

// DisplayOrder lists surface ids in the order Frame draws them, so ↑/↓ move the way
// the screen looks rather than the way the data is sorted. Rows with no tab are
// skipped — there is nothing to jump to.
func DisplayOrder(f board.Fleet) []string {
	blocked, working, quiet := f.Bands()
	var out []string
	for _, group := range [][]board.Row{blocked, working, quiet} {
		for _, r := range group {
			if r.Jumpable() {
				out = append(out, r.Surface)
			}
		}
	}
	return out
}

// Step moves the selection by delta, clamping at both ends. It deliberately does
// not wrap: wrapping past the bottom of QUIET lands on a blocked session, which is
// the one row you must never act on by accident.
func Step(order []string, sel string, delta int) string {
	if len(order) == 0 {
		return ""
	}
	if sel == "" {
		if delta > 0 {
			return order[0]
		}
		return order[len(order)-1]
	}
	for i, id := range order {
		if id == sel {
			j := i + delta
			if j < 0 {
				j = 0
			}
			if j >= len(order) {
				j = len(order) - 1
			}
			return order[j]
		}
	}
	return order[0] // the selected session vanished between ticks
}
