package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// Column widths for the one-shot table. Fixed, unlike the dashboard's: this output
// is often piped or pasted, so it must not reflow with the fleet.
const (
	colState     = 12
	colLabel     = 44
	colWorkspace = 17
)

// Todos is the one-shot `board todo` output. It states the cap on every listing,
// because the cap is the part of §12 that will bite, and a refusal is easier to accept
// when you have watched the number climb.
func Todos(f board.Fleet) string {
	_, _, todos, _ := f.Bands()
	if len(todos) == 0 {
		return "no todos · add one with: board todo \"<text>\"\n"
	}
	var b strings.Builder
	for _, r := range todos {
		// The id is shown because two todos can read alike, and then it is the only
		// unambiguous handle for `board todo done`.
		id, _ := r.TodoID()
		fmt.Fprintf(&b, "▫ %7s  %-8s %s\n", since(r.Idle), id, r.Label)
	}
	fmt.Fprintf(&b, "\n%d of %d todos · finish one with: board todo done <text or id>\n",
		len(todos), f.TodoCap)
	return b.String()
}

// Table is the one-shot `board` output: no colour, no bands, no cursor — the same
// snapshot as the dashboard, flattened for a scrollback or a pipe.
func Table(f board.Fleet, threshold time.Duration) string {
	var b strings.Builder
	// First, before the rows it qualifies. This output is piped and pasted, so a reader
	// who takes only the head must not miss that everything below is partial (§9.26).
	if f.Trouble != "" {
		fmt.Fprintf(&b, "%s\n\n", f.Trouble)
	}
	fmt.Fprintf(&b, "%s %s %s %7s\n",
		pad("STATE", colState), pad("LABEL", colLabel), pad(wsHeader, colWorkspace), "IDLE")
	var running, todos int
	for _, r := range f.Rows {
		switch r.Rank {
		case board.RankWorking:
			running++
		case board.RankTodo:
			todos++
		}
		state := r.State
		if r.Stale {
			state += " " + staleGlyph
		}
		// A todo's age is a lifetime, not a gap, on this surface as much as in the frame.
		when := humanize(r.Idle)
		if r.Rank == board.RankTodo {
			when = since(r.Idle)
		}
		fmt.Fprintf(&b, "%s %s %s %7s\n",
			pad(state, colState), pad(r.Label, colLabel), pad(r.Where(), colWorkspace), when)
	}
	// Sessions only: a todo is not a session, and folding the two together would report
	// a fleet larger than the one running (§12). The cell appears only when non-zero,
	// the same exception rule the band follows.
	fmt.Fprintf(&b, "\n%d sessions · %d blocked · %d running · %d quiet >%s",
		f.Sessions(), f.Blocked, running, f.Stale, humanize(threshold))
	if todos > 0 {
		fmt.Fprintf(&b, " · %d todo", todos)
	}
	b.WriteString("\n")
	return b.String()
}
