package watch

import (
	"strings"
	"testing"

	"board/internal/config"
)

// These are the loop's only writes, and they write user text, so they are pinned rather
// than eyeballed. $HOME is redirected: the real ~/.board.json is not a fixture.
func TestCommitTodo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := commitTodo("  reply to the ACME export  "); !strings.Contains(got, "reply to the ACME export") {
		t.Errorf("commit reported %q, want the trimmed text", got)
	}
	if todos := config.Load().Todos; len(todos) != 1 || todos[0].Text != "reply to the ACME export" {
		t.Fatalf("stored %v, want one trimmed todo", todos)
	}
	// Opening the mode by accident must cost nothing: empty text is a cancel, not an
	// error, and must not store a blank row.
	for _, blank := range []string{"", "   "} {
		if got := commitTodo(blank); got != "" {
			t.Errorf("commit(%q) reported %q, want silence", blank, got)
		}
	}
	if n := len(config.Load().Todos); n != 1 {
		t.Errorf("a blank commit changed the list: %d todos", n)
	}
}

func TestCommitTodoReportsTheCap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := range config.MaxTodos {
		if got := commitTodo("filler"); strings.Contains(got, "cap") {
			t.Fatalf("add %d refused early: %q", i+1, got)
		}
	}
	// The refusal has to reach the header, or the keystroke looks like it worked.
	if got := commitTodo("one too many"); !strings.Contains(got, "cap") {
		t.Errorf("commit past the cap reported %q, want the refusal", got)
	}
	if n := len(config.Load().Todos); n != config.MaxTodos {
		t.Errorf("%d todos stored, want the cap of %d", n, config.MaxTodos)
	}
}

func TestFinishTodo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	commitTodo("reply to the ACME export")
	td := config.Load().Todos[0]

	// The text comes back because it is the only record of it once it is gone: there is
	// no undo (DESIGN.md §12).
	if got := finishTodo(td.ID); !strings.Contains(got, "reply to the ACME export") {
		t.Errorf("finish reported %q, want the text it removed", got)
	}
	if n := len(config.Load().Todos); n != 0 {
		t.Errorf("%d todos left, want 0", n)
	}
	// A key the frame held from a stale tick must be a no-op, not a panic or a report.
	if got := finishTodo(td.ID); got != "" {
		t.Errorf("finishing a gone todo reported %q, want silence", got)
	}
}
