package config

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDefaultsSurviveAnEmptyOrBrokenFile(t *testing.T) {
	// Load never fails, so the zero value has to be usable: board is a reporting
	// surface and must still report when its config is missing or garbage.
	s := &State{}
	if got := s.Threshold(); got != defaultThreshold {
		t.Errorf("threshold = %v, want %v", got, defaultThreshold)
	}
	if got := s.Poll(); got != defaultPoll {
		t.Errorf("poll = %v, want %v", got, defaultPoll)
	}
}

func TestConfiguredValuesWin(t *testing.T) {
	s := &State{}
	s.Config.IdleThresholdMinutes = 90
	s.Config.PollSeconds = 30
	if got := s.Threshold(); got != 90*time.Minute {
		t.Errorf("threshold = %v, want 90m", got)
	}
	if got := s.Poll(); got != 30*time.Second {
		t.Errorf("poll = %v, want 30s", got)
	}
	// Zero and negative are "unset", not "instant": a 0s poll would spin.
	s.Config.PollSeconds = -1
	if got := s.Poll(); got != defaultPoll {
		t.Errorf("negative poll = %v, want the default", got)
	}
}

// Mouse reporting is opt-in, and the default matters more than the key: enabling it
// costs drag-to-select, so a file that does not mention it must leave it off.
func TestMouseIsOffUnlessTheFileSaysOtherwise(t *testing.T) {
	s := &State{}
	if s.Config.Mouse {
		t.Error("mouse defaults on; drag-to-select would break for a reader who never asked")
	}
	if err := json.Unmarshal([]byte(`{"config":{"mouse":true}}`), s); err != nil {
		t.Fatal(err)
	}
	if !s.Config.Mouse {
		t.Error(`"mouse": true did not parse; the key in README.md is the contract`)
	}
}

func TestSetLabel(t *testing.T) {
	s := &State{Labels: map[string]string{}}
	s.SetLabel("S-1", "merge app#1497")
	if s.Labels["S-1"] != "merge app#1497" {
		t.Fatalf("label not stored: %v", s.Labels)
	}
	// Empty text clears rather than storing a blank, so a cleared label falls back to
	// the tab title instead of rendering as an empty row.
	s.SetLabel("S-1", "")
	if _, ok := s.Labels["S-1"]; ok {
		t.Errorf("empty label stored instead of clearing: %v", s.Labels)
	}
}

// The cap is the feature's immune system (DESIGN.md §12): the 11th is refused, and it
// is refused rather than evicting the oldest, because the refusal is what forces a
// decision instead of silently losing the thing you wrote first.
func TestAddTodoRefusesThe11th(t *testing.T) {
	s := &State{}
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	for i := range MaxTodos {
		if _, err := s.AddTodo("todo", now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("add %d of %d failed: %v", i+1, MaxTodos, err)
		}
	}
	if _, err := s.AddTodo("one too many", now); err == nil {
		t.Fatal("the 11th todo was accepted; the cap is not enforced")
	}
	if len(s.Todos) != MaxTodos {
		t.Errorf("stored %d todos, want %d", len(s.Todos), MaxTodos)
	}
	// The oldest must still be there: eviction is what a cap must not do quietly.
	if s.Todos[0].Created != now {
		t.Errorf("oldest todo was evicted: %v", s.Todos[0].Created)
	}
}

func TestAddTodoRejectsBlankAndTrims(t *testing.T) {
	s := &State{}
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	for _, blank := range []string{"", "   ", "\t"} {
		if _, err := s.AddTodo(blank, now); err == nil {
			t.Errorf("AddTodo(%q) was accepted; a blank row reports nothing", blank)
		}
	}
	td, err := s.AddTodo("  review the export PR  ", now)
	if err != nil {
		t.Fatal(err)
	}
	if td.Text != "review the export PR" {
		t.Errorf("text = %q, want it trimmed", td.Text)
	}
	if td.ID == "" {
		t.Error("todo has no id; selection is keyed on identity, never position (§7)")
	}
	if td.Created != now {
		t.Errorf("created = %v, want the passed clock %v", td.Created, now)
	}
}

func TestAddTodoIDsAreUnique(t *testing.T) {
	s := &State{}
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	seen := map[string]bool{}
	for range MaxTodos {
		td, err := s.AddTodo("same text every time", now)
		if err != nil {
			t.Fatal(err)
		}
		if seen[td.ID] {
			t.Fatalf("duplicate id %q — two rows would share one selection key", td.ID)
		}
		seen[td.ID] = true
	}
}

// Matching takes an id prefix or a text substring, like jump does for labels: the ids
// exist for selection, not for the user to read off a screen and retype.
func TestFindTodo(t *testing.T) {
	s := &State{}
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	acme, _ := s.AddTodo("ACME csv export request", now)
	s.AddTodo("review Jeff's PR", now)
	s.AddTodo("reply to the security questionnaire", now)

	if got, err := s.FindTodo("csv"); err != nil || got.ID != acme.ID {
		t.Errorf("FindTodo(csv) = %v, %v; want the ACME todo", got.Text, err)
	}
	if got, err := s.FindTodo("ACME"); err != nil || got.ID != acme.ID {
		t.Errorf("FindTodo is not case-insensitive: %v, %v", got.Text, err)
	}
	if got, err := s.FindTodo(acme.ID); err != nil || got.ID != acme.ID {
		t.Errorf("FindTodo(%q) by id = %v, %v", acme.ID, got.Text, err)
	}
	if _, err := s.FindTodo("nothing like this"); err == nil {
		t.Error("a miss must be an error, not the first todo")
	}
	// Ambiguity is an error rather than a guess: removing the wrong one is unrecoverable
	// in v1, which has no undo (§12).
	if _, err := s.FindTodo("re"); err == nil {
		t.Error("an ambiguous match must be an error, not a guess")
	}
}

func TestDeleteTodo(t *testing.T) {
	s := &State{}
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	keep, _ := s.AddTodo("keep me", now)
	drop, _ := s.AddTodo("drop me", now)

	if got, ok := s.DeleteTodo(drop.ID); !ok || got.Text != "drop me" {
		t.Fatalf("DeleteTodo = %v, %v; want the dropped todo returned so it can be reported", got, ok)
	}
	if len(s.Todos) != 1 || s.Todos[0].ID != keep.ID {
		t.Errorf("todos after delete = %v, want only %q", s.Todos, keep.Text)
	}
	if _, ok := s.DeleteTodo("no-such-id"); ok {
		t.Error("deleting an unknown id reported success")
	}
	// Deleting the same id twice must not panic or drop a neighbour: the frame can hold
	// a stale key between ticks.
	if _, ok := s.DeleteTodo(drop.ID); ok {
		t.Error("second delete of the same id reported success")
	}
}
