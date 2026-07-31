package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// full builds a state the size of a real one: the cap in todos, each a sentence, plus
// labels for a fleet's worth of surfaces. Size matters here — the window this file is
// about is proportional to how long the write takes.
func full(t *testing.T) *State {
	t.Helper()
	s := &State{Labels: map[string]string{}}
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	for i := range MaxTodos {
		text := "reply to the ACME csv export request and copy the finance thread " +
			strings.Repeat("x", 40)
		if _, err := s.AddTodo(text, now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("building the fixture: %v", err)
		}
	}
	for i := range 20 {
		s.SetLabel(string(rune('A'+i))+"-39C7A0C-0000-0000", "migrate auth handlers to v2")
	}
	return s
}

// A read concurrent with a save must never see a file with no todos in it. This is the
// whole of item 11: os.WriteFile truncates and then writes, so between those two syscalls
// ~/.board.json is zero bytes — and Load swallows the parse error and returns defaults,
// so the reader is handed an empty todo list rather than an error. That is user content
// that exists nowhere else, disappearing silently. board todo from a shell racing the
// watch tab's own save is the case.
func TestASaveIsNeverVisibleHalfDone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := full(t)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var empties, saves int
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(stop)
		for range 300 {
			if err := s.Save(); err != nil {
				t.Errorf("save failed: %v", err)
				return
			}
			mu.Lock()
			saves++
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if got := len(Load().Todos); got != MaxTodos {
				mu.Lock()
				empties++
				mu.Unlock()
			}
		}
	}()
	wg.Wait()

	if empties > 0 {
		t.Errorf("a concurrent reader saw a todo list that was not there %d times over %d saves; "+
			"the write is not atomic and the loss is silent", empties, saves)
	}
}

// A save that cannot happen must leave the previous file exactly as it was. This is the
// other half of atomicity: the target is never opened for writing at all, so there is no
// state in which board has begun replacing the file and stopped.
func TestAFailedSaveLeavesThePreviousFileIntact(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := full(t)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}

	// No writes in the directory, so the temp file cannot be created. Restored on the way
	// out or t.TempDir's own cleanup fails.
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(home, 0o700) })

	if _, err := s.AddTodo("this one cannot be written", time.Now()); err == nil {
		t.Fatal("fixture is at the cap; the add should have been refused")
	}
	s.Labels["S-NEW"] = "a change that cannot be saved"
	if err := s.Save(); err == nil {
		t.Error("Save reported success into a directory it cannot write")
	}

	after, err := os.ReadFile(Path())
	if err != nil {
		t.Fatalf("the previous file is gone after a failed save: %v", err)
	}
	if string(after) != string(before) {
		t.Error("a failed save changed the file it could not replace")
	}
	if got := len(Load().Todos); got != MaxTodos {
		t.Errorf("todos after a failed save = %d, want %d — user content did not survive", got, MaxTodos)
	}
}

// The mode is part of the contract: this file holds the user's own text, and rename
// carries the temp file's mode onto the target. Tightening a loosened file is fine;
// widening it silently would not be.
func TestSaveKeepsTheFilePrivateAndLeavesNoTemp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(Path(), []byte(`{"todos":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := full(t)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %04o, want 0600 — a file of the user's own text must not widen", got)
	}

	// Nothing but the config itself: a temp file left beside it would be a copy of the
	// same private content with nothing to clean it up.
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Join(home, e.Name()) != Path() {
			t.Errorf("left %q behind next to the config", e.Name())
		}
	}
}

// Nothing pinned the round trip, and the nested config block is part of the user contract
// in README.md — a flattened field would still compile and would silently stop loading.
func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := full(t)
	s.Config.IdleThresholdMinutes = 90
	s.Config.PollSeconds = 30
	s.Config.NotifyCmd = "curl -sS -d @- https://ntfy.sh/my-topic"
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	got := Load()
	if got.Threshold() != 90*time.Minute || got.Poll() != 30*time.Second {
		t.Errorf("tuning did not survive: %v, %v", got.Threshold(), got.Poll())
	}
	if got.Config.NotifyCmd != s.Config.NotifyCmd {
		t.Errorf("notify_cmd = %q, want %q", got.Config.NotifyCmd, s.Config.NotifyCmd)
	}
	if len(got.Todos) != MaxTodos || len(got.Labels) != len(s.Labels) {
		t.Fatalf("round trip lost content: %d todos, %d labels", len(got.Todos), len(got.Labels))
	}
	if got.Todos[0].ID != s.Todos[0].ID || got.Todos[0].Text != s.Todos[0].Text {
		t.Error("todo identity did not survive; selection is keyed on it (§7)")
	}
	if !got.Todos[0].Created.Equal(s.Todos[0].Created) {
		t.Errorf("created = %v, want %v — the age is the only quantity a todo has",
			got.Todos[0].Created, s.Todos[0].Created)
	}
}
