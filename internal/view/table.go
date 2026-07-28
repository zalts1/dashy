package view

import (
	"fmt"
	"strings"
	"time"

	"board/internal/board"
)

// Column widths for the one-shot table. Fixed, unlike the dashboard's: this output
// is often piped or pasted, so it must not reflow with the fleet.
const (
	colState     = 12
	colLabel     = 44
	colWorkspace = 17
)

// Table is the one-shot `board` output: no colour, no bands, no cursor — the same
// snapshot as the dashboard, flattened for a scrollback or a pipe.
func Table(f board.Fleet, threshold time.Duration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s %6s\n",
		pad("STATE", colState), pad("LABEL", colLabel), pad("WORKSPACE", colWorkspace), "IDLE")
	var running int
	for _, r := range f.Rows {
		if r.Rank == board.RankWorking {
			running++
		}
		state := r.State
		if r.Stale {
			state += " ⚠"
		}
		fmt.Fprintf(&b, "%s %s %s %6s\n",
			pad(state, colState), pad(r.Label, colLabel), pad(r.Workspace, colWorkspace), humanize(r.Idle))
	}
	fmt.Fprintf(&b, "\n%d sessions · %d blocked · %d running · %d quiet >%s\n",
		len(f.Rows), f.Blocked, running, f.Stale, humanize(threshold))
	return b.String()
}
