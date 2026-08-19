// Package maki reads the session roster and state from maki.
//
// maki has no `agents --json`. Its roster lives inside the running process — the UI
// event loop owns every live session — and the only way in is a Lua plugin, so board
// installs one (`board install-hooks`, internal/hooks/maki.go). On every turn and every
// status change it writes what `maki.session.live()` answered to
// ~/.board/maki/<cmux surface id>.json.
//
// This package reads those reports. It is deliberately two reads and not one: a report
// outlives the process that wrote it — maki fires no shutdown event to delete one on —
// so the running maki processes are what say which reports still describe something
// alive. Board joins the two on pid, which is the same join the claude roster uses.
//
// DESIGN.md §17 is why it is shaped this way; EVIDENCE.md §9.32 is what the install cost.
package maki

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/host"
)

// The three statuses maki reports, spelled as `SessionStatus::as_str` spells them.
// needs_input is the one worth naming: unlike Claude Code's needsInput it is
// `permission_prompt.is_open() || pending_input`, so it means a human's turn and not
// "sitting at the prompt" — the distinction §9.2 had to work around for claude.
const (
	Working    = "working"
	NeedsInput = "needs_input"
	Idle       = "idle"
)

// The two ways the roster fails to arrive, kept separate because they are two different
// repairs: find out why the process query failed, or find out what shape the plugin is
// writing now. Naming the kind is what lets a caller report the fact instead of
// "something broke" (EVIDENCE.md §9.26).
var (
	ErrQueryFailed = errors.New("maki process query failed")
	ErrUnreadable  = errors.New("maki reports answered in an unknown shape")
)

// Session is one live maki session as the plugin found it.
type Session struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	// UpdatedAt is float64 rather than int64 because it arrives through Lua, where every
	// number is a double: 1754145000 and 1.754145e9 are the same second written two ways
	// and board must accept both.
	UpdatedAt float64 `json:"updated_at"`
}

// Blocked reports whether this session is waiting on a human.
func (s Session) Blocked() bool { return s.Status == NeedsInput }

func (s Session) Running() bool { return s.Status == Working }

// LastActivity is the idle clock: maki's own updated_at for the session. No file mtime
// is consulted, because a session idle for hours writes nothing and its report still
// carries the second it last moved. Zero stays zero — Build reads that as "no clock"
// and shows no idle time, where an epoch timestamp would show fifty-odd years.
func (s Session) LastActivity() time.Time {
	if s.UpdatedAt <= 0 {
		return time.Time{}
	}
	return time.Unix(int64(s.UpdatedAt), 0)
}

// Report is one maki process's answer: the tab it runs in and every session open in it.
type Report struct {
	Surface  string    `json:"surface"` // cmux surface id, from CMUX_SURFACE_ID
	Cwd      string    `json:"cwd"`
	Sessions []Session `json:"sessions"`
}

// Roster is the two reads that together say which maki sessions are live.
type Roster struct {
	Pids    []int             // maki processes running now
	Reports map[string]Report // by cmux surface id
}

// Available reports whether maki is installed. Asked before anything else is made of a
// silent maki: on a machine without it, no reports and no processes is the correct and
// uninteresting answer, not a fault to report (§14).
func Available() bool {
	_, err := exec.LookPath("maki")
	return err == nil
}

// RosterDir is where the plugin writes and board reads. Under board's own dot directory
// rather than maki's state dir: board is the one that defines this shape, and resolving
// maki's XDG-or-legacy state directory from Go would be a second copy of somebody
// else's precedence rules.
func RosterDir() string { return host.Home(".board", "maki") }

// Read gathers the roster. Whatever arrived is returned alongside the error, because a
// failed process query and an unreadable report are both partial answers and the caller
// reports the fact rather than discarding the half it got.
func Read() (Roster, error) {
	r := Roster{}
	pids, err := readPids()
	r.Pids = pids
	if err != nil {
		return r, err
	}
	reports, err := readReports()
	r.Reports = reports
	return r, err
}

// readPids asks which maki processes are running. pgrep, because Go's standard library
// cannot enumerate processes and the answer is needed on every tick: it is read-only,
// it costs a few milliseconds beside the claude roster's ~250ms, and an exact-name match
// is the whole query.
func readPids() ([]int, error) {
	b, err := host.Output("pgrep", "-x", "maki")
	if err != nil {
		// pgrep exits 1 for "nothing matched", which is the common answer on a machine
		// where maki is installed and not running — an answer, not a failure.
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	return parsePids(b), nil
}

// parsePids keeps the list ascending. Build walks it to make rows, and two rows that tie
// on band and idle time would otherwise swap places between ticks.
func parsePids(b []byte) []int {
	var out []int
	for _, line := range strings.Fields(string(b)) {
		if pid, err := strconv.Atoi(line); err == nil {
			out = append(out, pid)
		}
	}
	sort.Ints(out)
	return out
}

// readReports reads every report in the roster directory, keyed by the tab it names.
//
// A missing directory is the fresh answer — maki has never reported here — and not a
// failure. One unparseable file is: the shape is written by a plugin board ships, so a
// file board cannot read means the two have gone out of step, and skipping it would hide
// a live session behind a silence (§9.26).
func readReports() (map[string]Report, error) {
	dir := RosterDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	out := map[string]Report{}
	for _, e := range entries {
		// The directory is board's, but a stray file in it must not take the roster down.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		r, err := parseReport(b)
		if err != nil {
			return nil, err
		}
		// A report naming no tab cannot be joined to one. That is a maki started outside
		// cmux: a real session with nowhere for board to send you, so it is not a row —
		// the same rule interactive claude sessions with no surface are held to.
		if r.Surface == "" {
			continue
		}
		out[r.Surface] = r
	}
	return out, nil
}

func parseReport(b []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(b, &r); err != nil {
		return Report{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return r, nil
}
