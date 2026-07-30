package board

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
)

// Snapshot is everything read from the outside world for one tick. Build is a pure
// function of it, so every join rule below is testable without a live fleet — the
// rules that used to be wrong (EVIDENCE.md §9.1, §9.3) all lived in this join.
type Snapshot struct {
	Agents    []claude.Agent
	Titles    map[int]cmux.Titles  // by pid — cmux surface nodes carry no session id
	Clock     map[string]time.Time // session id -> last activity, already resolved
	JobLabels map[string]string    // session id -> Claude's own label for background agents
	Labels    map[string]string    // surface id -> user label
	Todos     []config.Todo        // work with no session behind it (§12)
	Threshold time.Duration
}

func Build(s Snapshot, now time.Time) Fleet {
	var f Fleet
	rows := make([]Row, 0, len(s.Agents))
	spans := map[string]bool{}
	for _, a := range s.Agents {
		t := s.Titles[a.Pid]
		// Interactive sessions with no cmux surface are subagents or sessions run
		// outside cmux. This board is a view of tabs, so they are not rows — a row
		// you cannot jump to is a row that cannot be acted on.
		if t.ID == "" && !a.IsBackground() {
			continue
		}
		ws := t.Workspace
		if ws == "" {
			ws = noWorkspace
		} else {
			spans[ws] = true
		}

		idle := time.Duration(0)
		if last := s.Clock[a.SessionID]; !last.IsZero() {
			if idle = now.Sub(last); idle < 0 {
				idle = 0
			}
		}

		r := Row{
			Key:       a.SessionID,
			State:     "done",
			Label:     label(a, t, s.Labels, s.JobLabels),
			Workspace: ws,
			Surface:   t.ID,
			Idle:      idle,
			Rank:      RankQuiet,
		}
		switch {
		case a.Blocked():
			r.State, r.Rank = "blocked →", RankBlocked
			f.Blocked++
		case a.Running():
			r.State, r.Rank = "running", RankWorking
		}
		// A working session is never stale: elapsed time is progress, not rot.
		if r.Rank != RankWorking && idle > s.Threshold {
			r.Stale = true
			f.Stale++
		}
		if idle > f.Oldest {
			f.Oldest = idle
		}
		rows = append(rows, r)
	}
	// Todos are rows with no process, so they take no state mark, no workspace and no
	// tab — and they feed none of the counters above, which are statements about
	// sessions: a fortnight-old todo in Oldest would own the KPI strip, and the stale ⚠
	// marks an idle session past a threshold, which a todo cannot be (§12).
	f.TodoCap = config.MaxTodos
	for _, td := range s.Todos {
		rows = append(rows, Row{
			Key:   todoKey + td.ID,
			State: "todo",
			Label: td.Text,
			Idle:  td.Age(now),
			Rank:  RankTodo,
		})
	}
	// Band first, then oldest within a band: the thing you have ignored longest
	// sits at the top of its group.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Rank != rows[j].Rank {
			return rows[i].Rank < rows[j].Rank
		}
		return rows[i].Idle > rows[j].Idle
	})
	f.Rows = rows
	f.Workspaces = len(spans)
	return f
}

// label picks the most specific name available: what the user called it, then what
// Claude Code calls it, then the tab title, then the directory.
//
// Every level is passed through verbatim, including the all-caps ones. Board does not
// restyle a name it was given: caps may be deliberate, and the board is reporting on
// the fleet, not editing it (§9.20).
func label(a claude.Agent, t cmux.Titles, userLabels, jobLabels map[string]string) string {
	if l := userLabels[t.ID]; l != "" {
		return l
	}
	if l := jobLabels[a.SessionID]; l != "" {
		return l
	}
	if t.Surface != "" {
		return t.Surface
	}
	return filepath.Base(a.Cwd)
}
