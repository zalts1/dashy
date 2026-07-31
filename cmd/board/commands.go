package main

import (
	"fmt"
	"os"

	"github.com/zalts1/dashy/internal/board"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/doctor"
	"github.com/zalts1/dashy/internal/view"
)

// show is the one-shot table. Not an error to have nothing to report: board is a
// reporting surface, and "no sessions" is a report.
//
// It no longer tests for cmux itself. It used to, and `watch` did not, which is how the
// two entry points came to disagree — one printed a sentence, the other painted a blank
// dashboard. Both now read the same phrase off the fleet (EVIDENCE.md §9.26).
func show() {
	st := config.Load()
	f := board.Collect()
	// Nothing wrong and nothing running is the one case worth a sentence instead of an
	// empty table. With trouble, the table leads with it and the header rows still say
	// what board was trying to show.
	if f.Trouble == "" && len(f.Rows) == 0 {
		fmt.Println("board: no live agent sessions.")
		return
	}
	fmt.Print(view.Table(f, st.Threshold()))
}

// diagnose is `board doctor`: the wiring, on a machine that is not the author's. It
// takes no arguments and cannot fail — every unreadable thing is a row that says so.
func diagnose() {
	fmt.Print(doctor.Format(doctor.Gather()))
}

// label names the session board was invoked from. Labels are keyed on the surface
// id, so they survive the session ending and being resumed in the same tab.
func label(text string) error {
	surface := cmux.CurrentSurface()
	if surface == "" {
		return fmt.Errorf("CMUX_SURFACE_ID unset — run this from inside a cmux session")
	}
	st := config.Load()
	st.SetLabel(surface, text)
	if err := st.Save(); err != nil {
		return err
	}
	if text == "" {
		fmt.Println("label cleared")
	} else {
		fmt.Printf("labeled: %s\n", text)
	}
	return nil
}

// jump focuses the one session whose label or workspace matches q. Ambiguity is an
// error with the candidates listed: focusing the wrong tab moves the user away from
// what they were doing.
func jump(q string) error {
	if q == "" {
		return fmt.Errorf("usage: board jump <substring>")
	}
	hits := board.Find(board.Collect().Rows, q)
	switch len(hits) {
	case 0:
		return fmt.Errorf("no session matching %q", q)
	case 1:
		if err := cmux.Focus(hits[0].Surface); err != nil {
			return err
		}
		fmt.Printf("→ %s  [%s]\n", hits[0].Label, hits[0].Workspace)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "%d sessions match %q:\n", len(hits), q)
		for _, r := range hits {
			fmt.Fprintf(os.Stderr, "   %s  [%s]\n", r.Label, r.Workspace)
		}
		return fmt.Errorf("be more specific")
	}
}
