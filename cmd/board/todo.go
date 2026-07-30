package main

// Capture is a command rather than a keystroke in the frame: in-frame text entry is an
// insert mode, and §2 retired "no TUI" only on the condition that there be none. The
// thought arrives while you are in a terminal anyway (DESIGN.md §12).

import (
	"fmt"
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/board"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/view"
)

// todo is add, list and done. Listing builds a Fleet from the todos alone rather than
// calling Collect: the list is not a fleet report, and reading it must not cost two
// subprocesses or require cmux to be running at all.
func todo(args []string) error {
	st := config.Load()
	switch {
	case len(args) == 0:
		fmt.Print(view.Todos(todoFleet(st)))
		return nil
	case args[0] == "done" && len(args) > 1:
		return todoDone(st, strings.Join(args[1:], " "))
	default:
		return todoAdd(st, strings.Join(args, " "))
	}
}

func todoAdd(st *config.State, text string) error {
	td, err := st.AddTodo(text, time.Now())
	if err != nil {
		return err
	}
	if err := st.Save(); err != nil {
		return err
	}
	fmt.Printf("todo: %s  (%d of %d)\n", td.Text, len(st.Todos), config.MaxTodos)
	return nil
}

// todoDone resolves the argument before removing anything, so an ambiguous match is an
// error naming the candidates rather than a guess: there is no undo in v1 (§12).
func todoDone(st *config.State, q string) error {
	td, err := st.FindTodo(q)
	if err != nil {
		return err
	}
	st.DeleteTodo(td.ID)
	if err := st.Save(); err != nil {
		return err
	}
	fmt.Printf("done: %s  (%d left)\n", td.Text, len(st.Todos))
	return nil
}

func todoFleet(st *config.State) board.Fleet {
	return board.Build(board.Snapshot{Todos: st.Todos}, time.Now())
}
