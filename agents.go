package main

// Session roster and state, from Claude Code itself.
//
// `claude agents --json` is authoritative: it knows every live session and reports
// idle/busy/waiting for interactive ones and blocked for background ones. That
// replaces the state this tool used to derive from cmux's audit log, which was
// wrong twice (see DESIGN.md §12) and which never saw background agents at all.
//
// It costs ~250ms, so it runs concurrently with the cmux query.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type agent struct {
	SessionID  string `json:"sessionId"`
	ID         string `json:"id"` // short id for background agents; names the jobs dir
	Kind       string `json:"kind"`
	Cwd        string `json:"cwd"`
	Pid        int    `json:"pid"`
	Name       string `json:"name"`
	Status     string `json:"status"` // interactive: idle | busy | waiting
	State      string `json:"state"`  // background: blocked | done
	WaitingFor string `json:"waitingFor"`
}

func claudeAgents() []agent {
	b, err := run("claude", "agents", "--json")
	if err != nil {
		return nil
	}
	var out []agent
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// blocked reports whether this session is waiting on a human.
func (a agent) blocked() bool {
	return a.Status == "waiting" || a.State == "blocked"
}

func (a agent) running() bool { return a.Status == "busy" }

// jobDetail pulls the extra context Claude Code keeps for a background agent:
// `needs` is the actual open question, which is a far better label than a slug.
func (a agent) jobDetail() (needs, name string) {
	if a.ID == "" {
		return "", ""
	}
	b, err := os.ReadFile(home(".claude", "jobs", a.ID, "state.json"))
	if err != nil {
		return "", ""
	}
	var st struct {
		Needs string `json:"needs"`
		Name  string `json:"name"`
	}
	json.Unmarshal(b, &st)
	return st.Needs, st.Name
}

// lastActivity is the fallback idle clock for sessions cmux never hooked. The
// transcript is appended on every turn, so its mtime is the last activity.
func (a agent) lastActivity() time.Time {
	p := home(".claude", "projects", strings.ReplaceAll(a.Cwd, "/", "-"), a.SessionID+".jsonl")
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

func (a agent) label(fallback string) string {
	if needs, name := a.jobDetail(); needs != "" || name != "" {
		if needs != "" {
			return needs
		}
		return name
	}
	if fallback != "" {
		return fallback
	}
	return filepath.Base(a.Cwd)
}

// cmuxUpdatedAt maps session id -> last hook activity. cmux's hook file misses
// some sessions entirely, so this is an enrichment rather than the roster.
func cmuxUpdatedAt() map[string]time.Time {
	out := map[string]time.Time{}
	b, err := os.ReadFile(home(".cmuxterm", "claude-hook-sessions.json"))
	if err != nil {
		return out
	}
	var file struct {
		Sessions map[string]session `json:"sessions"`
	}
	if json.Unmarshal(b, &file) != nil {
		return out
	}
	for _, s := range file.Sessions {
		if t := time.Unix(int64(s.UpdatedAt), 0); t.After(out[s.SessionID]) {
			out[s.SessionID] = t
		}
	}
	return out
}
