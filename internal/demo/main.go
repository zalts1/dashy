// Command demo renders the fleet the README's GIF is recorded from: board's own
// renderer over a fixture, and nothing else.
//
// It exists because board reads real sessions. A recording of a live fleet would
// publish labels, workspace names and todos — work data — into a README, so the
// demo supplies a synthetic world instead of a synthetic screen: the frames below
// come from view.Frame by way of board.Build, the same pure join production uses
// (DESIGN.md §3), so this cannot show a fleet board could not produce.
//
// It is not a board command. It lives under internal/ so `go install` cannot reach
// it, and it is built only by demo/record.sh.
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/board"
	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/view"
)

const (
	interval  = 10 * time.Second
	threshold = 45 * time.Minute
	// Fixed like the clock below, and for the same reason: which editor is installed on the
	// machine doing the recording must not change the frames it records (§16, §18). The
	// demo fleet carries no folders yet, so nothing renders from it — see §10.11.
	demoScheme = "vscode"
)

// start is fixed rather than time.Now: the header carries a clock, so a wall clock
// would make every recording a different file for no reason.
var start = time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)

// session is one tab in the demo fleet. The label is the cmux tab title, which is
// where board reads it from when the user has not set one of their own.
type session struct {
	pid       int
	id        string
	label     string
	workspace string
	status    string // interactive: idle | busy | waiting
	idle      time.Duration
}

// The fleet is small enough to read at a glance and varied enough to show every band:
// one blocked, one working, and a quiet tail with the ⚠ waterline inside it.
var sessions = []session{
	{501, "s-app", "merge app#1497 before branching", "APP", "waiting", 3 * time.Hour},
	{502, "s-api", "build the csv export endpoint", "API", "busy", 0},
	{503, "s-auth", "migrate auth handlers to v2", "AUTH", "idle", 9 * time.Minute},
	{504, "s-infra", "bump the staging image", "INFRA", "idle", 26 * time.Minute},
	{505, "s-rev", "Review pipeline PR #541", "REVIEWS", "idle", 2*time.Hour + 48*time.Minute},
	{506, "s-docs", "answer the ACME security questionnaire", "DOCS", "idle", 28 * time.Hour},
	{508, "s-web", "ship the pricing page copy", "WEB", "idle", 55 * time.Minute},
	{509, "s-cli", "fold the quiet band", "TOOLS", "idle", 4*time.Hour + 5*time.Minute},
	{510, "s-data", "backfill the events table", "DATA", "idle", 7*time.Hour + 20*time.Minute},
	{511, "s-ops", "rotate the staging credentials", "OPS", "idle", 3*24*time.Hour + 2*time.Hour},
}

// repoOf is the fixture's stand-in for host.Repository: a path under `.claude/worktrees` belongs
// to the checkout above it, anything else is its own repository.
func repoOf(cwd string) string {
	if i := strings.Index(cwd, "/.claude/worktrees/"); i > 0 {
		return cwd[:i]
	}
	return cwd
}

// snapshot is one tick of the demo world. blocked names a session that has just come
// back to ask something; extra is a todo that has just been added.
func snapshot(now time.Time, blocked, extra string) board.Snapshot {
	s := board.Snapshot{
		Titles:    map[int]cmux.Titles{},
		Clock:     map[string]time.Time{},
		JobLabels: map[string]string{"s-bg": "hunt the flaky payment test"},
		Labels:    map[string]string{},
		Threshold: threshold,
		// The location column reads these (§18). Supplied by the fixture like everything else
		// here: the demo must not depend on there being git repositories on the machine doing
		// the recording, and one session is put in a linked worktree so the recording shows the
		// `repo -> worktree` form as well as the plain one.
		Trees: map[string]string{},
		Repos: map[string]string{},
		Todos: []config.Todo{
			{ID: "2483b5", Text: "reply to the ACME csv export request", Created: now.Add(-12 * 24 * time.Hour)},
			{ID: "9f1c22", Text: "book the quarterly review", Created: now.Add(-26 * time.Hour)},
		},
	}
	if extra != "" {
		s.Todos = append(s.Todos, config.Todo{ID: "c07e41", Text: extra, Created: now})
	}
	for _, x := range sessions {
		status := x.status
		if x.id == blocked {
			status = "waiting"
		}
		cwd := "/Users/you/work/" + strings.ToLower(x.workspace)
		if x.id == "s-docs" {
			// One session in a linked worktree, so the recording shows both forms of the column.
			cwd = "/Users/you/work/docs/.claude/worktrees/acme-questionnaire"
		}
		s.Agents = append(s.Agents, claude.Agent{SessionID: x.id, Pid: x.pid, Kind: "interactive",
			Cwd: cwd, Status: status})
		s.Titles[x.pid] = cmux.Titles{ID: "S-" + x.id, Surface: x.label, Workspace: x.workspace}
		s.Clock[x.id] = now.Add(-x.idle)
		s.Trees[cwd] = cwd
		s.Repos[cwd] = repoOf(cwd)
	}
	// One background agent, because they are the rows people are surprised board has:
	// no tab, so no title and no workspace, and its label is the question Claude Code
	// recorded for it.
	s.Agents = append(s.Agents, claude.Agent{SessionID: "s-bg", ID: "bg1", Pid: 507,
		Kind: claude.Background, Cwd: "/Users/you/work/pay", State: "done"})
	s.Clock["s-bg"] = now.Add(-5 * time.Hour)
	return s
}

type beat struct {
	fleet board.Fleet
	now   time.Time
	ui    view.UI
	hold  time.Duration
}

// The todo typed on camera. Long enough to read as real work, short enough to finish
// inside the recording.
const typed = "ask sam about the ACME questionnaire"

// script is the demo: an ambient board, a session coming back to ask something, a
// todo captured without leaving the tab, and the quiet band folded away.
func script() []beat {
	var bs []beat
	add := func(off, hold time.Duration, blocked, extra string, ui view.UI) {
		now := start.Add(off)
		bs = append(bs, beat{board.Build(snapshot(now, blocked, extra), now), now, ui, hold})
	}

	add(0, 3400*time.Millisecond, "", "", view.UI{})
	// AUTH comes back with a question: the count in the header moves and the row lifts
	// out of the quiet tail into NEEDS YOU. That movement is the whole point of the tool.
	add(12*time.Second, 3600*time.Millisecond, "s-auth", "", view.UI{})

	// `a` opens the prompt on the legend's line. Typed word by word rather than
	// character by character: it reads the same and costs a sixth of the GIF frames.
	words := strings.Split(typed, " ")
	for i := range words {
		partial := strings.Join(words[:i+1], " ")
		if i < len(words)-1 {
			partial += " "
		}
		hold := 220 * time.Millisecond
		if i == len(words)-1 {
			hold = 1200 * time.Millisecond
		}
		add(20*time.Second, hold, "s-auth", "",
			view.UI{Typing: true, Input: partial, Paused: true})
	}

	add(24*time.Second, 3600*time.Millisecond, "s-auth", typed, view.UI{})
	// `z`, and the rows the quiet band gives back go to the bands where something is
	// happening.
	add(34*time.Second, 2600*time.Millisecond, "s-auth", typed,
		view.UI{QuietCollapsed: true})
	// Unfolded again, both because `z` is reversible and because a loop that restarts
	// from a folded board opens on the one frame with nothing in the bottom half.
	add(44*time.Second, 3*time.Second, "s-auth", typed, view.UI{})
	return bs
}

func main() {
	rows, cols := envInt("LINES", 40), envInt("COLUMNS", 118)
	out := os.Stdout

	// `demo table` prints the one-shot table for the same fleet, which is where the
	// README's plain-text block comes from. Two examples of two different worlds at the
	// top of a README is a small lie that costs a reader real time.
	if len(os.Args) > 1 && os.Args[1] == "table" {
		now := start
		fmt.Fprint(out, view.Table(board.Build(snapshot(now, "", ""), now), threshold))
		return
	}

	// Before the alternate screen, so the recording shows the command that was typed.
	time.Sleep(700 * time.Millisecond)
	fmt.Fprint(out, "\033[?1049h\033[?25l")
	defer fmt.Fprint(out, "\033[?25h\033[?1049l")

	for _, b := range script() {
		render(out, view.Frame(b.fleet, view.Screen{Now: b.now, Interval: interval,
			Threshold: threshold, Rows: rows, Cols: cols, EditorScheme: demoScheme}, b.ui))
		time.Sleep(b.hold)
	}
}

// render is watch.render: cursor home, overwrite line by line, clear below. Kept
// identical because a demo that painted differently would animate differently.
func render(out *os.File, s string) {
	var b bytes.Buffer
	b.WriteString("\033[H")
	for _, line := range strings.Split(s, "\n") {
		b.WriteString(line + "\033[K\n")
	}
	b.WriteString("\033[J")
	out.Write(b.Bytes())
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}
