package view

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

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
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118}, UI{}},
		{"frame-narrow.txt", goldenFleet(),
			Screen{now, 30 * time.Second, 4 * time.Hour, 24, 90}, UI{}},
		{"frame-sel.txt", goldenFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118},
			UI{Sel: "K-OLD", Paused: true, Notice: "cmux focus refused"}},
		{"frame-empty.txt", board.Fleet{},
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118}, UI{}},
		// The todo band, pinned with a selection on one of its rows: the caret and the
		// pause are what make `d` safe to press (DESIGN.md §12).
		{"frame-todo.txt", goldenTodoFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118},
			UI{Sel: "todo:t1", Paused: true}},
		// The quiet band folded, on the fleet whose tail is long enough for it to matter:
		// 32 rows to one line, and the rows the fold buys back go to the list (§9.21).
		{"frame-folded.txt", goldenTodoFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118},
			UI{QuietCollapsed: true}},
		// The capture mode, pinned in the slot the legend occupies when ambient.
		{"frame-typing.txt", goldenTodoFleet(),
			Screen{now, 10 * time.Second, 45 * time.Minute, 44, 118},
			UI{Typing: true, Paused: true, Input: "reply to the security questionnaire"}},
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
			{Key: "K-BLK", State: "blocked →", Label: "merge app#1497", Workspace: "APP", Surface: "S-BLK", Idle: 3 * time.Hour, Rank: board.RankBlocked},
			{Key: "K-BG", State: "blocked →", Label: "watch CI to green, or leave it here?", Workspace: "background", Surface: "", Idle: 52 * 24 * time.Hour, Rank: board.RankBlocked, Stale: true},
			{Key: "K-RUN", State: "running", Label: "busy thing", Workspace: "KILL", Surface: "S-RUN", Idle: 0, Rank: board.RankWorking},
			{Key: "K-OLD", State: "done", Label: "rotting thing", Workspace: "REVIEWS", Surface: "S-OLD", Idle: 50 * time.Hour, Rank: board.RankQuiet, Stale: true},
			{Key: "K-NEW", State: "done", Label: "fresh thing", Workspace: "TASKS", Surface: "S-NEW", Idle: 5 * time.Minute, Rank: board.RankQuiet},
		},
		Blocked:    2,
		Stale:      2,
		Workspaces: 5, // APP, KILL, REVIEWS, TASKS, W — background is not one
		Oldest:     52 * 24 * time.Hour,
	}
	for i := 0; i < 30; i++ {
		f.Rows = append(f.Rows, board.Row{Key: fmt.Sprintf("K-F%d", i), State: "done",
			Label: "filler", Workspace: "W", Surface: "S-F",
			Idle: time.Duration(30-i) * time.Hour, Rank: board.RankQuiet})
	}
	return f
}
