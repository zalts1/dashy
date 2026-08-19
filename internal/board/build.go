package board

import (
	"errors"
	"path/filepath"
	"sort"
	"time"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/maki"
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
	// Maki is the second agent's roster: the processes running now and the reports they
	// wrote. Two reads rather than one because a report outlives its process (§17).
	Maki maki.Roster
	// RosterErr is why the claude roster did not arrive, or nil. Without it Build cannot
	// tell an empty fleet from an unreadable one, which is the whole of EVIDENCE.md §9.26.
	RosterErr error
	// MakiErr is the same fact for the maki roster, kept apart because the two agents fail
	// independently and a fleet can be half readable.
	MakiErr error
	// NoCmux records that the multiplexer board reports on is absent. A quieter failure
	// than the roster's: the roster still arrives, and the loop below then drops every
	// interactive session in it for having no tab.
	NoCmux bool
}

func Build(s Snapshot, now time.Time) Fleet {
	var f Fleet
	rows := make([]Row, 0, len(s.Agents))
	spans := map[string]bool{}
	// track is the bookkeeping every session row shares, whichever agent produced it:
	// the counters the header states, and the stale mark.
	track := func(r *Row, idle time.Duration) {
		if r.Workspace != "" && r.Workspace != noWorkspace {
			spans[r.Workspace] = true
		}
		if r.Rank == RankBlocked {
			f.Blocked++
		}
		// A working session is never stale: elapsed time is progress, not rot. A blocked
		// one is exactly what the mark is for — a question unanswered for three hours.
		if r.Rank != RankWorking && idle > s.Threshold {
			r.Stale = true
			f.Stale++
		}
		if idle > f.Oldest {
			f.Oldest = idle
		}
	}

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
		}
		idle := idleFor(s.Clock, a.SessionID, now)
		r := Row{
			Key:       a.SessionID,
			State:     "done",
			Label:     label(s.Labels[t.ID], s.JobLabels[a.SessionID], t.Surface, a.Cwd),
			Workspace: ws,
			Surface:   t.ID,
			Idle:      idle,
			Rank:      RankQuiet,
		}
		switch {
		case a.Blocked():
			r.State, r.Rank = "blocked →", RankBlocked
		case a.Running():
			r.State, r.Rank = "running", RankWorking
		}
		track(&r, idle)
		rows = append(rows, r)
	}

	// maki's rows, joined on the same key and stated in the same words (§17). The walk is
	// over the process list rather than the reports, because that is what settles liveness:
	// nothing on disk says a maki has exited, so a report nobody is running is not a row.
	// Pids arrive sorted, so the order two sessions tie in is the same on every tick.
	//
	// A tab is taken once however many pids resolve to it. cmux lists a surface's whole
	// process tree, so one maki started from a shell inside a tab routinely arrives as two
	// pids — and without this each of them would emit that tab's sessions again, two rows
	// sharing one key and every counter doubled (EVIDENCE.md §9.33).
	unreported := 0
	taken := map[string]bool{}
	for _, pid := range s.Maki.Pids {
		t := s.Titles[pid]
		if t.ID == "" || taken[t.ID] {
			continue
		}
		taken[t.ID] = true
		rep, ok := s.Maki.Reports[t.ID]
		if !ok {
			// A maki running in a tab and saying nothing. Counted, not guessed at — a row
			// with an invented state would be worse than an absent one.
			unreported++
			continue
		}
		ws := t.Workspace
		if ws == "" {
			ws = noWorkspace
		}
		for _, sess := range rep.Sessions {
			idle := idleFor(s.Clock, sess.ID, now)
			r := Row{
				Key:       sess.ID,
				State:     "done",
				Label:     label(s.Labels[t.ID], sess.Title, t.Surface, rep.Cwd),
				Workspace: ws,
				Surface:   t.ID,
				Idle:      idle,
				Rank:      RankQuiet,
			}
			switch {
			case sess.Blocked():
				r.State, r.Rank = "blocked →", RankBlocked
			case sess.Running():
				r.State, r.Rank = "running", RankWorking
			}
			track(&r, idle)
			rows = append(rows, r)
		}
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
	f.Trouble = trouble(s, unreported)
	return f
}

// idleFor is the one idle rule both agents get. A missing clock reads as no idle time
// rather than as fifty-odd years, and a clock in the future reads as none rather than as
// a negative gap.
func idleFor(clock map[string]time.Time, key string, now time.Time) time.Duration {
	last := clock[key]
	if last.IsZero() {
		return 0
	}
	if idle := now.Sub(last); idle > 0 {
		return idle
	}
	return 0
}

// trouble turns an unread world into the one line both renderers print. Each phrase
// names the fact and then `doctor`, which is where the same failure is enumerated in
// full — a report that says only "something is wrong" leaves the reader to guess, and
// the whole point of §9.14 read forwards is that an available route should say so.
//
// The order is by how much of the board the fact costs. The claude roster outranks
// everything: with no roster there are no claude rows at all, and it is the bulk of most
// fleets. Then cmux, without which every interactive session loses its tab. Then maki,
// which costs one agent's rows. One line has room for the most fundamental fact and
// nothing here is the last word — `doctor` lists them all.
func trouble(s Snapshot, unreportedMaki int) string {
	// claude's absence stops being trouble once maki is reporting: board has a fleet on
	// screen and the tool that is missing was a choice. It is the reports that settle
	// that and not the binary — an installed maki nobody is running leaves board with
	// nothing to show, which is the state §9.26 is about.
	noClaude := errors.Is(s.RosterErr, claude.ErrNotInstalled)
	switch {
	case noClaude && len(s.Maki.Reports) == 0:
		return "claude not found · board doctor"
	case errors.Is(s.RosterErr, claude.ErrUnreadable):
		return "roster unreadable · board doctor"
	case s.RosterErr != nil && !noClaude:
		// Includes ErrQueryFailed and anything a future caller puts here: an unrecognised
		// failure is still a failure, and defaulting to silence is the bug being fixed.
		return "roster unavailable · board doctor"
	case s.NoCmux:
		return "cmux not found · board doctor"
	case unreportedMaki > 0 && len(s.Maki.Reports) == 0:
		// A maki running in a tab, and not one report on the machine: the shape of a hook
		// that was never installed, which is the one maki failure a reader can fix.
		//
		// Both halves are needed. cmux counts a surface's whole process tree, so any maki
		// a script or a `maki --print` starts inside a tab looks like an unreported one —
		// and on a wired machine that would cry wolf on every tick, which is how a signal
		// stops being read. With no reports at all there is nothing to cry wolf about
		// (EVIDENCE.md §9.33).
		return "maki not reporting · board doctor"
	case s.MakiErr != nil:
		return "maki roster unreadable · board doctor"
	}
	return ""
}

// label picks the most specific name available: what the user called it, then what the
// agent calls it — Claude Code's open question for a background job, maki's own session
// title — then the tab title, then the directory.
//
// Every level is passed through verbatim, including the all-caps ones. Board does not
// restyle a name it was given: caps may be deliberate, and the board is reporting on
// the fleet, not editing it (§9.20).
func label(userLabel, agentLabel, tabTitle, cwd string) string {
	for _, l := range []string{userLabel, agentLabel, tabTitle} {
		if l != "" {
			return l
		}
	}
	return filepath.Base(cwd)
}
