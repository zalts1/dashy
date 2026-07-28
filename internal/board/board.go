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
	State     string
	Label     string
	Workspace string
	Surface   string // cmux surface id; empty for background agents, which have no tab
	Idle      time.Duration
	Stale     bool // quiet past the threshold
	Rank      int
}

// Jumpable reports whether this row has a tab to focus.
func (r Row) Jumpable() bool { return r.Surface != "" }

type Fleet struct {
	Rows       []Row
	Blocked    int
	Stale      int
	Workspaces int // distinct real workspaces; background agents have none
	Oldest     time.Duration
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
