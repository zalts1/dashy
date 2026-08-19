package board

import (
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/github"
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
	repos := map[string]string{}
	resolve := func(dir string) {
		if dir == "" {
			return
		}
		if _, seen := trees[dir]; !seen {
			t := host.WorkTree(dir)
			trees[dir] = t
			// Memoised on the worktree rather than on the directory: several sessions in one
			// worktree resolve the repository once, and the read is a single file (§18).
			if t != "" {
				if _, seen := repos[t]; !seen {
					repos[t] = host.Repository(t)
				}
			}
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

	// The pull requests, and the only read here that leaves the machine. Asked last because it
	// needs the worktrees the loop above resolved, and asked at all only when the `github` key is
	// set: unset — the default — nothing runs and no row carries the glyph (§10.12).
	prs := map[string]string{}
	if st.Config.GitHub {
		for tree, pr := range github.Read(github.Targets(repos)) {
			prs[tree] = pr.URL
		}
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
		Repos:      repos,
		PRs:        prs,
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

// Find returns the rows whose label, location or cmux workspace contains q, case-insensitively.
//
// The location is matched because it is what the reader can see: the column shows a repository
// and a worktree now, so `board jump dashy` has to reach the row whose visible location is
// exactly that. The workspace stays matchable even though it is no longer drawn — it was
// matchable before, and taking a search term away is a worse surprise than an invisible one
// that still works (§18).
func Find(rows []Row, q string) []Row {
	q = strings.ToLower(q)
	var hits []Row
	for _, r := range rows {
		for _, field := range []string{r.Label, r.Where(), r.Workspace} {
			if strings.Contains(strings.ToLower(field), q) {
				hits = append(hits, r)
				break
			}
		}
	}
	return hits
}
