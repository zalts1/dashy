// Package board is the domain: one snapshot of the fleet, joined from every
// source and shared by both renderers so they can never disagree about state.
//
// Add a derived quantity to Fleet, not to a renderer.
package board

import (
	"strings"
	"time"
)

// Rank is the sort key and the band selector, in the order attention is owed.
// RankTodo is last because a todo is the only row with no process to report on, and both
// renderers draw it there: it is not a state a session can be in (DESIGN.md §12).
const (
	RankBlocked = 0
	RankQuiet   = 1
	RankWorking = 2
	RankTodo    = 3
)

// todoKey namespaces a todo's id inside the one selection space it shares with session
// ids. A collision would put the cursor on a row the user did not choose (§7).
const todoKey = "todo:"

// noWorkspace fills the workspace column for agents that have no cmux tab — which since
// §17 means Claude Code background agents and nothing else. It reads the same as claude's
// Kind by coincidence, not by dependency.
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
	// Folder is the worktree this session is working in — what an editor opens to show
	// the branch's changed files. Empty when board could not resolve the directory, and
	// for a todo, which has no process and so no directory (§18).
	Folder string
	// Preview is the URL a local dev server is serving this worktree at, or "" when
	// nothing is up. One row shows one preview: the nearest, resolved in Build so the
	// two renderers cannot disagree about which (§18).
	Preview string
	Idle    time.Duration
	Stale   bool // quiet past the threshold
	Rank    int
}

// Jumpable reports whether this row has a tab to focus. Selectable and jumpable are
// different: a background agent can be selected and read, but there is nowhere to go.
func (r Row) Jumpable() bool { return r.Surface != "" }

// TodoID reports the todo behind this row, if it is one. It gates the one destructive
// key in the frame, so it must never answer true for a session (§12).
func (r Row) TodoID() (string, bool) {
	if r.Rank == RankTodo && strings.HasPrefix(r.Key, todoKey) {
		return strings.TrimPrefix(r.Key, todoKey), true
	}
	return "", false
}

type Fleet struct {
	Rows       []Row
	Blocked    int
	Stale      int
	Workspaces int // distinct real workspaces; background agents have none
	Oldest     time.Duration
	// TodoCap is the ceiling both renderers state, so neither has to hardcode it and
	// they cannot disagree about what the refusal at the top will be (§12).
	TodoCap int
	// Trouble is what board could not read, in the words both renderers print, and
	// empty when the world was legible. Derived here for the same reason the counts
	// are: an unreadable fleet and an empty one are different facts, and two renderers
	// inventing their own phrasing for the difference is how they come to disagree
	// (§3, EVIDENCE.md §9.26).
	Trouble string
}

// Sessions counts the rows with a process behind them. Both renderers state a fleet
// size, and a todo is a row but not a session — derived here so they cannot disagree
// about it (§3, §12).
func (f Fleet) Sessions() int {
	n := 0
	for _, r := range f.Rows {
		if r.Rank != RankTodo {
			n++
		}
	}
	return n
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

// Bands splits the fleet into the four on-screen groups, preserving the sort. They are
// returned in the order the frame draws them, which is not the rank order: todos sit
// above the quiet tail so the collapse cannot delete them (§12).
func (f Fleet) Bands() (blocked, working, todo, quiet []Row) {
	for _, r := range f.Rows {
		switch r.Rank {
		case RankBlocked:
			blocked = append(blocked, r)
		case RankWorking:
			working = append(working, r)
		case RankTodo:
			todo = append(todo, r)
		default:
			quiet = append(quiet, r)
		}
	}
	return
}
