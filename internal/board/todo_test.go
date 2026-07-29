package board

import (
	"testing"
	"time"

	"board/internal/claude"
	"board/internal/cmux"
	"board/internal/config"
)

func todos(ts ...config.Todo) func(*Snapshot) {
	return func(s *Snapshot) { s.Todos = ts }
}

func todo(id, text string, age time.Duration) config.Todo {
	return config.Todo{ID: id, Text: text, Created: now.Add(-age)}
}

// A todo is the one row with no process behind it: no session, no pid, no tab. It is
// still a row, and the action on it is to start it (DESIGN.md §12).
func TestTodosBecomeRows(t *testing.T) {
	f := Build(Snapshot{Todos: []config.Todo{todo("ab12cd", "ACME csv export", 3*time.Hour)}}, now)
	if len(f.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(f.Rows))
	}
	r := f.Rows[0]
	if r.Rank != RankTodo {
		t.Errorf("rank = %d, want RankTodo (%d)", r.Rank, RankTodo)
	}
	if r.Label != "ACME csv export" {
		t.Errorf("label = %q, want the todo text", r.Label)
	}
	if r.Idle != 3*time.Hour {
		t.Errorf("idle = %v, want the age since capture (3h)", r.Idle)
	}
	if r.Jumpable() {
		t.Error("todo reported as jumpable; there is nothing to jump to yet")
	}
	if r.Workspace != "" {
		t.Errorf("workspace = %q, want empty: a todo belongs to no tab", r.Workspace)
	}
	// The key namespaces the id so a todo can never collide with a session id — both
	// live in one selection space, and a collision would move the cursor to a row the
	// user did not choose (§7).
	if r.Key == "ab12cd" {
		t.Error("todo key is the bare id; it shares a namespace with session ids")
	}
	if id, ok := r.TodoID(); !ok || id != "ab12cd" {
		t.Errorf("TodoID() = %q, %v; want ab12cd, true", id, ok)
	}
}

func TestSessionRowsAreNotTodos(t *testing.T) {
	r := one(t, snap(interactive()))
	if id, ok := r.TodoID(); ok {
		t.Errorf("session row reported TodoID %q; d would delete something that is not a todo", id)
	}
}

// Every KPI is a statement about sessions. A fortnight-old todo in Oldest would own the
// strip and misreport the fleet, and ⚠ marks an idle session past the threshold, which
// is not a thing a todo can be (§12).
func TestTodosStayOutOfTheFleetCounts(t *testing.T) {
	s := snap(interactive(), todos(todo("t1", "two weeks of not starting", 14*24*time.Hour)))
	f := Build(s, now)
	if len(f.Rows) != 2 {
		t.Fatalf("got %d rows, want the session and the todo", len(f.Rows))
	}
	if f.Oldest != 30*time.Minute {
		t.Errorf("oldest = %v, want the session's 30m — a todo is not an idle session", f.Oldest)
	}
	if f.Stale != 0 {
		t.Errorf("stale = %d, want 0: a todo older than the threshold is normal", f.Stale)
	}
	if f.Blocked != 0 {
		t.Errorf("blocked = %d, want 0", f.Blocked)
	}
	if f.Workspaces != 1 {
		t.Errorf("workspaces = %d, want 1: a todo has no workspace", f.Workspaces)
	}
	for _, r := range f.Rows {
		if _, ok := r.TodoID(); ok && r.Stale {
			t.Error("todo row marked stale; the ⚠ is an idle-session mark")
		}
	}
}

// Oldest first, like every other band: the thing you have ignored longest sits at the
// top of its group.
func TestTodosSortOldestFirst(t *testing.T) {
	s := Snapshot{Todos: []config.Todo{
		todo("t1", "yesterday", 24*time.Hour),
		todo("t2", "just now", time.Minute),
		todo("t3", "last week", 7*24*time.Hour),
	}}
	var got []string
	for _, r := range Build(s, now).Rows {
		got = append(got, r.Label)
	}
	want := []string{"last week", "yesterday", "just now"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// The flattened order is the one-shot table's; the frame places bands itself (§12).
// Todos come last there: they are the only rows with no process to report on.
func TestTodosSortAfterEverySession(t *testing.T) {
	agent := func(id string, pid int, status string) claude.Agent {
		return claude.Agent{SessionID: id, Pid: pid, Status: status}
	}
	s := Snapshot{
		Agents: []claude.Agent{agent("quiet", 1, "idle"), agent("busy", 2, "busy"), agent("blocked", 3, "waiting")},
		Titles: map[int]cmux.Titles{
			1: {ID: "S1", Surface: "quiet"}, 2: {ID: "S2", Surface: "busy"}, 3: {ID: "S3", Surface: "blocked"},
		},
		Clock:     map[string]time.Time{"quiet": now.Add(-time.Hour), "busy": now, "blocked": now.Add(-time.Minute)},
		Todos:     []config.Todo{todo("t1", "a todo", 90*24*time.Hour)},
		Threshold: 45 * time.Minute,
	}
	var got []string
	for _, r := range Build(s, now).Rows {
		got = append(got, r.Label)
	}
	want := []string{"blocked", "quiet", "busy", "a todo"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v (todos last, however old)", got, want)
		}
	}
}

// Both renderers state a fleet size, so the count of sessions is derived once here
// rather than twice in view (§3). A todo is a row and not a session.
func TestSessionsExcludesTodos(t *testing.T) {
	s := snap(interactive(), todos(todo("t1", "a todo", time.Hour), todo("t2", "another", time.Hour)))
	f := Build(s, now)
	if got := f.Sessions(); got != 1 {
		t.Errorf("Sessions() = %d, want 1 of %d rows", got, len(f.Rows))
	}
	if got := Build(Snapshot{}, now).Sessions(); got != 0 {
		t.Errorf("empty fleet Sessions() = %d, want 0", got)
	}
}

func TestBandsSplitTodosOut(t *testing.T) {
	f := Fleet{Rows: []Row{
		{Label: "b", Rank: RankBlocked}, {Label: "q1", Rank: RankQuiet},
		{Label: "t1", Rank: RankTodo}, {Label: "q2", Rank: RankQuiet},
		{Label: "w", Rank: RankWorking}, {Label: "t2", Rank: RankTodo},
	}}
	blocked, working, todo, quiet := f.Bands()
	if len(blocked) != 1 || len(working) != 1 || len(todo) != 2 || len(quiet) != 2 {
		t.Fatalf("bands = %d/%d/%d/%d, want 1/1/2/2",
			len(blocked), len(working), len(todo), len(quiet))
	}
	if todo[0].Label != "t1" || quiet[0].Label != "q1" {
		t.Errorf("a band was reordered: todo=%v quiet=%v", todo, quiet)
	}
}
