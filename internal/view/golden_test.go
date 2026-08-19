package view

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// The scheme every golden renders with. Fixed, like the clock: which editor is installed on
// the machine running the suite must not change the pinned frame (§18).
const goldenScheme = "vscode"

// The golden files were captured from the pre-package-split renderer. They pin the
// whole frame — layout, escape codes, bar lengths, collapse behaviour — so any
// change to rendering has to be an intentional one that re-blesses them.
//
// Re-bless deliberately, and only with a reason: BLESS=1 go test ./internal/view
var update = os.Getenv("BLESS") != ""

func TestGoldenFrames(t *testing.T) {
	now := time.Date(2026, 7, 28, 14, 30, 0, 0, time.UTC)
	cases := []struct {
		file string
		f    board.Fleet
		s    Screen
		u    UI
	}{
		{"frame-wide.txt", goldenFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme}, UI{}},
		{"frame-narrow.txt", goldenFleet(),
			Screen{now, 30 * time.Second, 4 * time.Hour, 24, 90, goldenScheme}, UI{}},
		{"frame-sel.txt", goldenFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme},
			UI{Sel: "K-OLD", Paused: true, Notice: "cmux focus refused"}},
		{"frame-empty.txt", board.Fleet{},
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme}, UI{}},
		// The todo band, pinned with a selection on one of its rows: the caret and the
		// pause are what make `d` safe to press (DESIGN.md §12).
		{"frame-todo.txt", goldenTodoFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme},
			UI{Sel: "todo:t1", Paused: true}},
		// The quiet band folded, on the fleet whose tail is long enough for it to matter:
		// 32 rows to one line, and the rows the fold buys back go to the list (§9.21).
		{"frame-folded.txt", goldenTodoFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme},
			UI{QuietCollapsed: true}},
		// The capture mode, pinned in the slot the legend occupies when ambient.
		{"frame-typing.txt", goldenTodoFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme},
			UI{Typing: true, Paused: true, Input: "reply to the security questionnaire"}},
		// A world board could not fully read: the trouble in the span, and the one row that
		// still arrived counted beside it. This is what a machine with no cmux actually
		// shows — the background agent has no tab to lose (§9.26).
		{"frame-trouble.txt", goldenTroubleFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme}, UI{}},
		// The link cell, pinned with the escapes in it: one row pointing at both a preview
		// and a folder, one at only a folder, one at neither, and a todo — which has no
		// process and so no directory. Nothing else in the suite would catch a hyperlink
		// that stopped being closed (§18, §9.34).
		{"frame-links.txt", goldenLinkFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme}, UI{}},
		// A tall tab, so the frame takes the airy form: a blank line between rows. Pinned because
		// the spacing is a fit decision, and a fit decision that is not pinned is one that drifts
		// (§9.44). frame-wide.txt is the same fleet on a tab too short for it, i.e. compact.
		{"frame-airy.txt", goldenLinkFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 120, 118, goldenScheme}, UI{}},
		// The legend under `?`: the scale's rungs and all three glyphs by name, on the line the
		// keys were on. Pinned because it is the one line whose whole job is to be read (§9.38).
		{"frame-help.txt", goldenLinkFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118, goldenScheme}, UI{Help: true}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			got := Frame(c.f, c.s, c.u)
			path := "testdata/" + c.file
			if update {
				os.WriteFile(path, []byte(got), 0o600)
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("frame differs from golden %s\n--- got ---\n%q\n--- want ---\n%q",
					c.file, got, string(want))
			}
		})
	}
}

// goldenLinkFleet is the awkward fleet with links on it. The directories are deliberately
// the two shapes the join has to tell apart: a repository, and a linked worktree inside it
// (§18).
func goldenLinkFleet() board.Fleet {
	f := goldenFleet()
	f.TodoCap = 10
	// All three on one row, so the full cell is pinned and not just its combinations.
	f.Rows[0].Folder = "/Users/you/work/repo"
	f.Rows[0].Preview = "https://app.localhost"
	f.Rows[0].Storybook = "http://localhost:6006"
	f.Rows[0].PR = "https://github.com/you/repo/pull/1497"
	f.Rows[0].PRState = "open"
	f.Rows[2].Folder = "/Users/you/work/repo/.claude/worktrees/csv export"
	f.Rows[2].Storybook = "http://localhost:6007"
	f.Rows[3].Folder, f.Rows[3].Preview = "/Users/you/work/other", "https://api.localhost:8443"
	f.Rows[3].PR = "https://github.com/you/repo/pull/1502"
	f.Rows[3].PRState = "merged"
	// A branch somebody abandoned, so all three states are pinned.
	f.Rows[4].PR, f.Rows[4].PRState = "https://github.com/you/repo/pull/1301", "closed"
	f.Rows[4].Storybook = "http://localhost:6007"
	f.Rows = append(f.Rows, board.Row{Key: "todo:t0", State: "todo",
		Label: "book the quarterly review", Idle: 26 * time.Hour, Rank: board.RankTodo})
	return f
}

// goldenTroubleFleet is a partially-read world: cmux is gone, so the interactive
// sessions were dropped for having no tab and only the background agent survives. The
// trouble and the count are both true at once, which is the case worth pinning — a
// header that reported only one of them was the bug.
func goldenTroubleFleet() board.Fleet {
	return board.Fleet{
		Trouble: "cmux not found · board doctor",
		Rows: []board.Row{
			{Key: "K-BG", State: "blocked →", Label: "watch CI to green, or leave it here?",
				Repo: "background", Idle: 52 * 24 * time.Hour, Rank: board.RankBlocked, Stale: true},
		},
		Blocked: 1,
		Stale:   1,
		Oldest:  52 * 24 * time.Hour,
		TodoCap: 10,
	}
}

// goldenTodoFleet adds the todo list to the awkward fleet: a fortnight-old note, a
// day-old one, a fresh one, and a label wider than the column.
func goldenTodoFleet() board.Fleet {
	f := goldenFleet()
	f.TodoCap = 10
	for i, td := range []struct {
		text string
		age  time.Duration
	}{
		{"reply to the ACME csv export request before Thursday", 12 * 24 * time.Hour},
		{"review the export PR", 26 * time.Hour},
		{"book the quarterly review", 9 * time.Minute},
	} {
		f.Rows = append(f.Rows, board.Row{Key: fmt.Sprintf("todo:t%d", i), State: "todo",
			Label: td.text, Idle: td.age, Rank: board.RankTodo})
	}
	return f
}

// goldenFleet is deliberately awkward: a background agent with no surface, a label
// wider than the column, a 52-day idle at the top of the scale, and enough filler to
// force the QUIET tail to collapse.
func goldenFleet() board.Fleet {
	f := board.Fleet{
		Rows: []board.Row{
			{Key: "K-BLK", State: "blocked →", Label: "merge app#1497", Repo: "APP", Surface: "S-BLK", Idle: 3 * time.Hour, Rank: board.RankBlocked},
			{Key: "K-BG", State: "blocked →", Label: "watch CI to green, or leave it here?", Repo: "background", Surface: "", Idle: 52 * 24 * time.Hour, Rank: board.RankBlocked, Stale: true},
			{Key: "K-RUN", State: "running", Label: "busy thing", Repo: "KILL", Surface: "S-RUN", Idle: 0, Rank: board.RankWorking},
			{Key: "K-OLD", State: "done", Label: "rotting thing", Repo: "app", Tree: "csv-export", Surface: "S-OLD", Idle: 50 * time.Hour, Rank: board.RankQuiet, Stale: true},
			{Key: "K-NEW", State: "done", Label: "fresh thing", Repo: "TASKS", Surface: "S-NEW", Idle: 5 * time.Minute, Rank: board.RankQuiet},
		},
		Blocked:    2,
		Stale:      2,
		Workspaces: 5, // APP, KILL, REVIEWS, TASKS, W — background is not one
		Oldest:     52 * 24 * time.Hour,
	}
	for i := 0; i < 30; i++ {
		f.Rows = append(f.Rows, board.Row{Key: fmt.Sprintf("K-F%d", i), State: "done",
			Label: "filler", Repo: "W", Surface: "S-F",
			Idle: time.Duration(30-i) * time.Hour, Rank: board.RankQuiet})
	}
	return f
}
