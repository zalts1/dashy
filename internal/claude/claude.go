// Package claude reads the session roster and state from Claude Code itself.
//
// `claude agents --json` is authoritative: it knows every live session and reports
// idle/busy/waiting for interactive ones and blocked for background ones. That
// replaces the state board used to derive from cmux's audit log, which was wrong
// twice (EVIDENCE.md §9.1) and never saw background agents at all.
//
// It costs ~250ms, so callers run it concurrently with the cmux query.
package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/host"
)

// The three ways the roster fails to arrive, kept separate because they are three
// different repairs: install claude, find out why the query failed, or find out what
// shape it answers in now. Callers name the kind rather than reporting "something
// broke" (EVIDENCE.md §9.26).
var (
	ErrNotInstalled = errors.New("claude not found")
	ErrQueryFailed  = errors.New("claude agents failed")
	ErrUnreadable   = errors.New("claude agents answered in an unknown shape")
)

type Agent struct {
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

const Background = "background"

// Agents reads the roster. The error is half the answer: no agents and no error is a
// quiet fleet, no agents and an error is a fleet board cannot see, and returning nil
// for both is what let a broken machine look like an idle one (EVIDENCE.md §9.26).
func Agents() ([]Agent, error) {
	b, err := host.Output("claude", "agents", "--json")
	if err != nil {
		// Not installed is worth its own answer: it is the one failure with an obvious
		// repair, and the likeliest on a machine board has just been copied to.
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrNotInstalled
		}
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	return parseAgents(b)
}

func parseAgents(b []byte) ([]Agent, error) {
	var out []Agent
	if err := json.Unmarshal(b, &out); err != nil {
		// The shape is undocumented and has changed before (§9.1, §9.3), so this is the
		// failure most likely to arrive without anyone touching board.
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return out, nil
}

// Blocked reports whether this session is waiting on a human. One field now,
// where it used to be a log heuristic that shipped wrong twice.
func (a Agent) Blocked() bool { return a.Status == "waiting" || a.State == "blocked" }

func (a Agent) Running() bool { return a.Status == "busy" }

func (a Agent) IsBackground() bool { return a.Kind == Background }

// JobLabel is the extra context Claude Code keeps for a background agent. `needs`
// is the actual open question, which beats a slug: "watch CI to green, or leave it
// here?" tells you what to do; the job name does not. Empty for interactive agents.
func (a Agent) JobLabel() string {
	if a.ID == "" {
		return ""
	}
	b, err := os.ReadFile(host.Home(".claude", "jobs", a.ID, "state.json"))
	if err != nil {
		return ""
	}
	var st struct {
		Needs string `json:"needs"`
		Name  string `json:"name"`
	}
	if json.Unmarshal(b, &st) != nil {
		return ""
	}
	if st.Needs != "" {
		return st.Needs
	}
	return st.Name
}

// LastActivity is the fallback idle clock for sessions cmux never hooked. The
// transcript is appended on every turn, so its mtime is the last activity.
func (a Agent) LastActivity() time.Time {
	p := host.Home(".claude", "projects", strings.ReplaceAll(a.Cwd, "/", "-"), a.SessionID+".jsonl")
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
