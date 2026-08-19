package main

import (
	"fmt"
	"os"

	"github.com/zalts1/dashy/internal/board"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/doctor"
	"github.com/zalts1/dashy/internal/editor"
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

// chooseEditor is `board editor`: the chooser for what a row's folder link opens.
//
// It exists because the obvious place for the question is unreachable. board hands the URL
// to the terminal and the terminal opens it, so there is no moment of the click for board
// to notice and no "open with" panel it could raise — an editor board never launches cannot
// be chosen at launch time (DESIGN.md §18). So the choice is made ahead of time, here, and
// with no argument the command is a listing rather than an error: being asked what the
// options are is the whole reason somebody types this.
func chooseEditor(args []string) error {
	st := config.Load()
	if len(args) == 0 {
		fmt.Print(editor.Format(editor.Gather(st.Config.Editor, config.Path())))
		return nil
	}
	name := args[0]
	// `auto` is the way back, and it is a word rather than an empty argument because an
	// empty one is how a shell expands a variable that was never set.
	if name == "auto" {
		st.SetEditor("")
		if err := st.Save(); err != nil {
			return err
		}
		fmt.Print(editor.Format(editor.Gather("", config.Path())))
		return nil
	}
	e, ok := editor.Lookup(name)
	if !ok {
		// The refusal names the whole vocabulary: a reader who guessed the spelling wrong
		// should not have to guess again.
		return fmt.Errorf("board does not know the editor %q — try one of: %s", name, editor.Names())
	}
	st.SetEditor(e.Name)
	if err := st.Save(); err != nil {
		return err
	}
	fmt.Print(editor.Format(editor.Gather(e.Name, config.Path())))
	return nil
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
