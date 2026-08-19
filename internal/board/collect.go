package board

import (
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/host"
	"github.com/zalts1/dashy/internal/maki"
	"github.com/zalts1/dashy/internal/preview"
)

// Collect gathers one Snapshot and builds a Fleet from it. This is the only impure
// part of the package: everything it reads is read-only, and all the judgement
// lives in Build.
func Collect() Fleet {
	st := config.Load()

	// The subprocess calls are independent and the roster is not cheap: claude agents
	// ~250ms, cmux top ~50ms, pgrep a few. Overlapped, a tick still costs ~250ms.
	var agents []claude.Agent
	var rosterErr error
	done := make(chan struct{})
	go func() { agents, rosterErr = claude.Agents(); close(done) }()

	// Asked before the roster is read rather than after, because on a machine without
	// maki the answer to every question below is "nothing" and that is not a fault (§17).
	var roster maki.Roster
	var makiErr error
	makiDone := make(chan struct{})
	go func() {
		if maki.Available() {
			roster, makiErr = maki.Read()
		}
		close(makiDone)
	}()

	// The third optional agent-adjacent tool, asked the same way and for the same reason:
	// on a machine without portless the answer is "no previews", which is not a fault
	// (§18). Its error is dropped rather than carried into Trouble — a missing preview
	// costs one glyph on one row, and the trouble line is ordered by how much of the board
	// a fact costs (§14). `doctor` is where this read is reported.
	// Read unconditionally rather than behind preview.Available(): that answers for portless
	// only, and a machine with no portless can still be running a Storybook (§18).
	var previews preview.Roster
	previewDone := make(chan struct{})
	go func() {
		previews, _ = preview.Read()
		close(previewDone)
	}()

	titles := cmux.TitlesByPid()
	<-done
	<-makiDone
	<-previewDone

	// Every directory either side of the link join, resolved to its worktree once. Both
	// sides go through the same function so the two answers are comparable — that is the
	// whole join (§18) — and the memo matters because a fleet routinely has several
	// sessions in one worktree.
	trees := map[string]string{}
	resolve := func(dir string) {
		if dir == "" {
			return
		}
		if _, seen := trees[dir]; !seen {
			trees[dir] = host.WorkTree(dir)
		}
	}
	for _, a := range agents {
		resolve(a.Cwd)
	}
	for _, rep := range roster.Reports {
		resolve(rep.Cwd)
	}
	for _, rt := range previews.Routes {
		resolve(rt.Dir)
	}
	for _, sb := range previews.Storybooks {
		resolve(sb.Dir)
	}

	clock := cmux.HookClock()
	jobs := map[string]string{}
	for _, a := range agents {
		// cmux's hook clock is the most accurate when present; the transcript mtime
		// covers the sessions it never registered.
		if clock[a.SessionID].IsZero() {
			if t := a.LastActivity(); !t.IsZero() {
				clock[a.SessionID] = t
			}
		}
		if l := a.JobLabel(); l != "" {
			jobs[a.SessionID] = l
		}
	}
	// maki sessions carry their own clock in the report, so there is nothing to fall back
	// to and nothing to prefer: cmux never hooked them, and maki's updated_at is what the
	// session itself says about when it last moved.
	for _, rep := range roster.Reports {
		for _, sess := range rep.Sessions {
			if t := sess.LastActivity(); !t.IsZero() {
				clock[sess.ID] = t
			}
		}
	}

	return Build(Snapshot{
		Agents:     agents,
		Titles:     titles,
		Clock:      clock,
		JobLabels:  jobs,
		Labels:     st.Labels,
		Todos:      st.Todos,
		Threshold:  st.Threshold(),
		Trees:      trees,
		Previews:   previews.Routes,
		Storybooks: previews.Storybooks,
		Maki:       roster,
		RosterErr:  rosterErr,
		MakiErr:    makiErr,
		// Asked here rather than in the callers, so every entry point learns about a
		// missing cmux from the same place and cannot word it differently (§9.26).
		NoCmux: !cmux.Available(),
	}, time.Now())
}

// Find returns the rows whose label or workspace contains q, case-insensitively.
func Find(rows []Row, q string) []Row {
	q = strings.ToLower(q)
	var hits []Row
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Label), q) ||
			strings.Contains(strings.ToLower(r.Workspace), q) {
			hits = append(hits, r)
		}
	}
	return hits
}
