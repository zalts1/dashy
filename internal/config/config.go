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
	"path/filepath"
	"time"

	"github.com/zalts1/dashy/internal/host"
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
		// Mouse is off by default because turning it on costs drag-to-select: the terminal
		// forwards the press instead of starting a selection. That is the reader's trade,
		// not board's (DESIGN.md §7).
		Mouse bool `json:"mouse"`
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

// Save writes a temp file and renames it over the target, because the rename is the only
// step a reader can see and it is atomic. os.WriteFile truncated first and wrote second,
// so between those two syscalls this file was zero bytes — and Load turns an unparseable
// file into defaults rather than an error, so a reader in that window was handed an empty
// todo list instead of a failure (EVIDENCE.md §9.27). The content exists nowhere else.
//
// No fsync before the rename, deliberately. Rename already covers what board can be
// killed by — a crash, a Ctrl-C, a shell racing the watch tab — because the old file
// stands until the new one is complete. Beyond that is power loss, and Sync on darwin is
// a real barrier there: Go issues F_FULLFSYNC, which flushes the drive's own cache. That
// is the reason to skip it, not a reason to add it — a full flush on every todo and every
// label, on a path a human takes interactively, to protect a list that can be retyped.
func (s *State) Save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := Path()
	// Same directory: a rename across filesystems is a copy, and a copy is not atomic.
	f, err := os.CreateTemp(filepath.Dir(path), ".board.json-*")
	if err != nil {
		return err
	}
	// Removed on every path. After a successful rename nothing is there and this is a
	// harmless miss; on every failure it is what keeps a temp file from accumulating
	// next to the real one.
	defer os.Remove(f.Name())
	// CreateTemp already opens at 0600, which is the mode this file must keep. Rename
	// carries it, so a target a user had loosened is tightened rather than widened.
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
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
