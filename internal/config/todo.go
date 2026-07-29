package config

// Todos are the one thing in this file that is not derivable from the fleet and not
// tuning: text you wrote, which nothing else in the stack holds. DESIGN.md §12 is why
// they exist and why the cap is the load-bearing part.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// MaxTodos is a refusal, not a trim. Uncapped, the band becomes a backlog and dilutes
// the one question board answers; evicting the oldest to make room would lose the
// thing you wrote first, silently. Refusing the 11th forces the decision instead (§12).
const MaxTodos = 10

// Todo carries text and age and nothing else — no status, no priority, no due date.
// §1 keeps that structure in Linear.
type Todo struct {
	ID      string    `json:"id"`
	Text    string    `json:"text"`
	Created time.Time `json:"created"`
}

// Age is the only quantity a todo has, and it is a reproach: it is shown and never
// hidden (§12).
func (t Todo) Age(now time.Time) time.Duration {
	if d := now.Sub(t.Created); d > 0 {
		return d
	}
	return 0
}

// AddTodo appends one, taking the clock as an argument so the cap and the age are
// testable without one. now is stored as written: a todo's age is measured from when
// you admitted to it.
func (s *State) AddTodo(text string, now time.Time) (Todo, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Todo{}, fmt.Errorf("a todo needs text")
	}
	if len(s.Todos) >= MaxTodos {
		return Todo{}, fmt.Errorf("at the cap of %d todos — finish or drop one first", MaxTodos)
	}
	td := Todo{ID: s.newTodoID(now), Text: text, Created: now}
	s.Todos = append(s.Todos, td)
	return td, nil
}

// FindTodo resolves a user's argument the way jump resolves a session: an id prefix or
// a text substring, case-insensitively, with ambiguity an error rather than a guess.
// Removal has no undo in v1, so guessing would be unrecoverable (§12).
func (s *State) FindTodo(q string) (Todo, error) {
	if q = strings.TrimSpace(q); q == "" {
		return Todo{}, fmt.Errorf("usage: board todo done <text or id>")
	}
	lower := strings.ToLower(q)
	var hits []Todo
	for _, td := range s.Todos {
		if strings.HasPrefix(td.ID, lower) || strings.Contains(strings.ToLower(td.Text), lower) {
			hits = append(hits, td)
		}
	}
	switch len(hits) {
	case 0:
		return Todo{}, fmt.Errorf("no todo matching %q", q)
	case 1:
		return hits[0], nil
	default:
		// With the ids, because the candidates may all read alike and the id is then the
		// only handle that resolves.
		var texts []string
		for _, td := range hits {
			texts = append(texts, fmt.Sprintf("%q (%s)", td.Text, td.ID))
		}
		return Todo{}, fmt.Errorf("%d todos match %q: %s", len(hits), q, strings.Join(texts, ", "))
	}
}

// DeleteTodo removes by exact id and returns what it removed, so the caller can report
// the text — the only record of it once it is gone (§12: no undo in v1). An unknown id
// is a false return, not an error: the frame can hold a stale key between ticks.
func (s *State) DeleteTodo(id string) (Todo, bool) {
	for i, td := range s.Todos {
		if td.ID == id {
			s.Todos = append(s.Todos[:i:i], s.Todos[i+1:]...)
			return td, true
		}
	}
	return Todo{}, false
}

// newTodoID is short because it is typed at a prompt, and random because ids must
// never be reused: selection is keyed on identity, so a recycled id would point the
// cursor at a row the user did not choose (§7). Six hex chars over a list of ten.
func (s *State) newTodoID(now time.Time) string {
	taken := map[string]bool{}
	for _, td := range s.Todos {
		taken[td.ID] = true
	}
	for range 8 {
		var b [3]byte
		if _, err := rand.Read(b[:]); err != nil {
			// Never fail an add over entropy: the clock is unique enough at nanoseconds.
			return fmt.Sprintf("%06x", now.UnixNano()&0xffffff)
		}
		if id := hex.EncodeToString(b[:]); !taken[id] {
			return id
		}
	}
	return fmt.Sprintf("%06x", now.UnixNano()&0xffffff)
}
