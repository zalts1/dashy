package watch

// The loop's only writes. Both report the line the header should show, because after a
// jump the board tab may not be the visible one and after a removal the text is the only
// record left (DESIGN.md §12).

import (
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/config"
)

// commitTodo stores what was typed. Empty text is a cancel rather than an error: the
// mode is one keystroke away, so opening it by accident must cost nothing.
func commitTodo(text string) string {
	if text = strings.TrimSpace(text); text == "" {
		return ""
	}
	st := config.Load()
	td, err := st.AddTodo(text, time.Now())
	if err != nil {
		// The cap, usually. It has to reach the header, or the keystroke looks like it
		// worked and the thought is lost.
		return err.Error()
	}
	if err := st.Save(); err != nil {
		return err.Error()
	}
	return "todo: " + td.Text
}

// finishTodo removes one by id. An id the frame held from a stale tick is silence, not
// an error: nothing was lost, and there is nothing for the user to do about it.
func finishTodo(id string) string {
	st := config.Load()
	td, ok := st.DeleteTodo(id)
	if !ok {
		return ""
	}
	if err := st.Save(); err != nil {
		return err.Error()
	}
	return "done: " + td.Text
}
