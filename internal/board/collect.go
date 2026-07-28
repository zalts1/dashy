package board

import (
	"strings"
	"time"

	"board/internal/claude"
	"board/internal/cmux"
	"board/internal/config"
)

// Collect gathers one Snapshot and builds a Fleet from it. This is the only impure
// part of the package: everything it reads is read-only, and all the judgement
// lives in Build.
func Collect() Fleet {
	st := config.Load()

	// The two subprocess calls are independent and neither is cheap: claude agents
	// ~250ms, cmux top ~50ms. Overlapped, a tick costs ~250ms rather than ~300.
	var agents []claude.Agent
	done := make(chan struct{})
	go func() { agents = claude.Agents(); close(done) }()
	titles := cmux.TitlesByPid()
	<-done

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

	return Build(Snapshot{
		Agents:    agents,
		Titles:    titles,
		Clock:     clock,
		JobLabels: jobs,
		Labels:    st.Labels,
		Threshold: st.Threshold(),
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
