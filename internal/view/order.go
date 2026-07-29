package view

import "board/internal/board"

// Navigation lives with the renderer because it must follow the screen, not the
// data. Two rules taken from prior art rather than invented: selection is keyed on
// the session, never a row index (htop's Follow key exists because index-based
// selection drifts when the list re-sorts under you), and the caller pauses the
// refresh while a selection is live (less's +F makes the streaming/interacting
// boundary explicit instead of clever).

// DisplayOrder lists row keys in the order Frame draws them, so ↑/↓ move the way the
// screen looks rather than the way the data is sorted. The todo list is last, as the
// frame draws it (§9.19).
//
// Every row is a stop, including the tab-less background agents. Selection does two
// jobs — it picks the jump target and it lifts a row out of the collapsed quiet tail —
// so skipping them made a row that the frame counts and can never draw (§9.14). Enter
// on one reports that there is no tab.
func DisplayOrder(f board.Fleet) []string {
	blocked, working, todo, quiet := f.Bands()
	var out []string
	for _, group := range [][]board.Row{blocked, working, quiet, todo} {
		for _, r := range group {
			out = append(out, r.Key)
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
