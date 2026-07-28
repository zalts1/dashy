// Package board is the domain: one snapshot of the fleet, joined from every
// source and shared by both renderers so they can never disagree about state.
//
// Add a derived quantity to Fleet, not to a renderer.
package board

import "time"

// Rank is the sort key and the band selector, in the order attention is owed.
const (
	RankBlocked = 0
	RankQuiet   = 1
	RankWorking = 2
)

// noWorkspace fills the workspace column for agents that have no cmux tab. It reads
// the same as claude's Kind by coincidence, not by dependency.
const noWorkspace = "background"

type Row struct {
	// Key identifies the row for selection. It is the session id, which every row has
	// — Surface is empty for background agents, and selection has to reach them: it is
	// what lifts a row out of the collapsed quiet tail as well as what Enter acts on.
	Key       string
	State     string
	Label     string
	Workspace string
	Surface   string // cmux surface id; empty for background agents, which have no tab
	Idle      time.Duration
	Stale     bool // quiet past the threshold
	Rank      int
}

// Jumpable reports whether this row has a tab to focus. Selectable and jumpable are
// different: a background agent can be selected and read, but there is nowhere to go.
func (r Row) Jumpable() bool { return r.Surface != "" }

type Fleet struct {
	Rows       []Row
	Blocked    int
	Stale      int
	Workspaces int // distinct real workspaces; background agents have none
	Oldest     time.Duration
}

// ByKey resolves a selection to its row. An empty key is the ambient state and must
// never resolve, or clearing the selection would leave Enter pointing at a session.
func (f Fleet) ByKey(key string) (Row, bool) {
	if key == "" {
		return Row{}, false
	}
	for _, r := range f.Rows {
		if r.Key == key {
			return r, true
		}
	}
	return Row{}, false
}

// Bands splits the fleet into the three on-screen groups, preserving the sort.
func (f Fleet) Bands() (blocked, working, quiet []Row) {
	for _, r := range f.Rows {
		switch r.Rank {
		case RankBlocked:
			blocked = append(blocked, r)
		case RankWorking:
			working = append(working, r)
		default:
			quiet = append(quiet, r)
		}
	}
	return
}
