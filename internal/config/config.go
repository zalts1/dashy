// Package config is board's on-disk state: ~/.board.json, the only file board
// writes. It holds tuning, surface-id → label, and the todo list — nothing that can
// be derived from the fleet.
//
// Todos are the only user content here, capped at MaxTodos by a refusal rather than a
// trim (DESIGN.md §12). Everything else in the file is tuning or a name.
package config

import (
	"encoding/json"
	"os"
	"time"

	"board/internal/host"
)

const (
	defaultThreshold = 45 * time.Minute
	defaultPoll      = 10 * time.Second
)

// State mirrors the file layout. The nested Config block is part of the user
// contract in README.md; do not flatten it.
type State struct {
	Config struct {
		IdleThresholdMinutes int    `json:"idle_threshold_minutes"`
		PollSeconds          int    `json:"poll_seconds"`
		NotifyCmd            string `json:"notify_cmd"`
	} `json:"config"`
	Labels map[string]string `json:"labels"`
	// Todos are the one thing here that cannot be derived from the fleet: work with no
	// session behind it (§12). Capped at MaxTodos.
	Todos []Todo `json:"todos"`
}

func Path() string { return host.Home(".board.json") }

// Load never fails: a missing or corrupt file yields defaults, because board is a
// reporting surface and must still report.
func Load() *State {
	s := &State{Labels: map[string]string{}}
	if b, err := os.ReadFile(Path()); err == nil {
		json.Unmarshal(b, s)
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

func (s *State) Save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), b, 0o600)
}

func (s *State) Threshold() time.Duration {
	if s.Config.IdleThresholdMinutes > 0 {
		return time.Duration(s.Config.IdleThresholdMinutes) * time.Minute
	}
	return defaultThreshold
}

func (s *State) Poll() time.Duration {
	if s.Config.PollSeconds > 0 {
		return time.Duration(s.Config.PollSeconds) * time.Second
	}
	return defaultPoll
}

// SetLabel records or clears the label for a surface.
func (s *State) SetLabel(surface, text string) {
	if text == "" {
		delete(s.Labels, surface)
		return
	}
	s.Labels[surface] = text
}
