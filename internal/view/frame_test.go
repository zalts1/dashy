package view

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"board/internal/board"
)

var frameNow = time.Date(2026, 7, 28, 14, 30, 0, 0, time.UTC)

func screen(rows, cols int) Screen {
	return Screen{Now: frameNow, Interval: 10 * time.Second,
		Threshold: 4 * time.Hour, Rows: rows, Cols: cols}
}

// fixture is the small navigation fleet: one of each band, plus a background agent
// with no tab.
func fixture() board.Fleet {
	return board.Fleet{Workspaces: 4, Rows: []board.Row{
		{Key: "K-BLK", State: "blocked →", Label: "merge app#1497", Workspace: "APP", Surface: "S-BLK", Idle: 3 * time.Hour, Rank: board.RankBlocked},
		{Key: "K-BG", State: "blocked →", Label: "no tab to jump to", Workspace: "background", Idle: time.Hour, Rank: board.RankBlocked},
		{Key: "K-OLD", State: "done", Label: "rotting thing", Workspace: "REVIEWS", Surface: "S-OLD", Idle: 50 * time.Hour, Rank: board.RankQuiet, Stale: true},
		{Key: "K-NEW", State: "done", Label: "fresh thing", Workspace: "TASKS", Surface: "S-NEW", Idle: 5 * time.Minute, Rank: board.RankQuiet},
		{Key: "K-RUN", State: "running", Label: "busy thing", Workspace: "KILL", Surface: "S-RUN", Rank: board.RankWorking},
	}}
}

func TestSelectionIsMarkedAndPauseIsVisible(t *testing.T) {
	plain := Frame(fixture(), screen(44, 130), UI{})
	if strings.Contains(plain, "▸") {
		t.Error("caret drawn with no selection")
	}
	if strings.Contains(plain, "paused") {
		t.Error("paused shown when live")
	}
	sel := Frame(fixture(), screen(44, 130), UI{Sel: "K-OLD", Paused: true})
	if !strings.Contains(sel, "▸") {
		t.Error("no caret for selection")
	}
	// A stale frame that does not say it is stale looks live, and Enter would jump to
	// whatever moved under the cursor.
	if !strings.Contains(sel, "paused") {
		t.Error("paused not surfaced")
	}
}

// A failed focus must be reported inside the frame: when the jump happens the board
// tab is typically no longer the visible one, so stderr goes unseen.
func TestNoticeRendersInHeader(t *testing.T) {
	out := Frame(fixture(), screen(44, 130), UI{Notice: "cmux focus refused"})
	if !strings.Contains(out, "cmux focus refused") {
		t.Error("notice not rendered in header")
	}
}

// collapsing is the fixture for the collapse rules: enough quiet rows to overflow a
// short tab, including a fresh background row that has no tab to jump to.
func collapsing() board.Fleet {
	f := fixture()
	for i := 0; i < 40; i++ {
		f.Rows = append(f.Rows, board.Row{Key: fmt.Sprintf("K-F%d", i), Label: "filler",
			Workspace: "W", Surface: "S-F", Idle: time.Duration(40-i) * time.Hour, Rank: board.RankQuiet})
	}
	f.Rows = append(f.Rows, board.Row{Key: "K-BGQ", Label: "background agent, no tab",
		Workspace: "background", Idle: time.Minute, Rank: board.RankQuiet})
	return f
}

func TestQuietTailCollapses(t *testing.T) {
	f := collapsing()
	out := Frame(f, screen(20, 130), UI{})
	// The count must stay visible: a band that collapses silently reads as a band
	// with nothing in it.
	if !collapseCount.MatchString(out) {
		t.Error("collapsed tail did not report how many rows it hid")
	}
	// A selection in the hidden tail must still be drawn, or the cursor is invisible.
	sel := Frame(f, screen(20, 130), UI{Sel: "K-NEW", Paused: true})
	if !strings.Contains(sel, "fresh thing") {
		t.Error("selected row was collapsed out of view")
	}
}

var collapseCount = regexp.MustCompile(`\+\d+ quiet`)

// The collapse line reports a count; it is not a control. A disclosure chevron said
// otherwise for a while, and there is no key that expands the band — the honest ways
// to see a hidden row are to select it or to make the tab taller (EVIDENCE.md §9.14).
func TestCollapseLineOffersNothingItCannotDo(t *testing.T) {
	out := Frame(collapsing(), screen(20, 130), UI{})
	if strings.Contains(out, "⌄") {
		t.Error("collapse line draws a disclosure chevron, promising an expand key that does not exist")
	}
	if !collapseCount.MatchString(out) {
		t.Errorf("collapse line does not read as a count of hidden rows")
	}
}

// The hidden tail is reachable one row at a time, so a row that cannot be selected
// can be counted and never drawn. Background agents have no surface, which used to
// keep them out of the navigation order entirely.
func TestATabLessRowInTheTailCanBeBroughtIntoView(t *testing.T) {
	f := collapsing()
	if out := Frame(f, screen(20, 130), UI{}); strings.Contains(out, "background agent, no tab") {
		t.Fatal("fixture is not exercising the collapse; the tab-less row is already visible")
	}
	sel := Frame(f, screen(20, 130), UI{Sel: "K-BGQ", Paused: true})
	if !strings.Contains(sel, "background agent, no tab") {
		t.Error("a tab-less row in the hidden tail cannot be brought on screen at all")
	}
}

func TestEmptyBandsStillRender(t *testing.T) {
	out := Frame(board.Fleet{}, screen(44, 130), UI{})
	// NEEDS YOU is the band you look at first; it must exist even when empty, or its
	// absence is indistinguishable from not having looked.
	if !strings.Contains(out, "NEEDS YOU") || !strings.Contains(out, "nothing blocked") {
		t.Error("empty NEEDS YOU band missing")
	}
	if !strings.Contains(out, "elapsed") {
		t.Error("scale legend missing; the bars become decoration without it")
	}
}

// The bar is time you owe a session. A working agent's elapsed time is progress,
// so it gets no bar.
func TestBarsOnlyWhereTimeIsOwed(t *testing.T) {
	f := board.Fleet{Rows: []board.Row{
		{Label: "busy thing", Surface: "S-RUN", Idle: 50 * time.Hour, Rank: board.RankWorking},
	}}
	if strings.Contains(Frame(f, screen(44, 130), UI{}), "▇▇") {
		t.Error("working row drew an idle bar")
	}
	f.Rows[0].Rank = board.RankQuiet
	if !strings.Contains(Frame(f, screen(44, 130), UI{}), "▇▇") {
		t.Error("quiet row drew no idle bar")
	}
}

func TestColumnWidths(t *testing.T) {
	wide := board.Fleet{Rows: []board.Row{{Label: strings.Repeat("x", 200), Workspace: "APP"}}}
	// avail is what the elastic columns share once the fixed chrome and the right
	// margin are paid for.
	avail := func(cols int) int { return cols - headMargin - rowChrome }
	cases := []struct {
		name string
		f    board.Fleet
		cols int
		want int
	}{
		{"short labels do not shrink below the floor", fixture(), 130, minLabelW},
		// The tail keeps its floor even for a 3-letter workspace, so the label gets
		// what is left of avail, not all of it.
		{"a long label is capped by the terminal", wide, 90, avail(90) - runes(wsHeader)},
		{"and by the absolute cap on a wide terminal", wide, 400, maxLabelW},
		{"a narrow terminal cannot push it below the floor", wide, 50, minLabelW},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _, _ := columns(c.f, c.cols); got != c.want {
				t.Errorf("label column = %d, want %d", got, c.want)
			}
		})
	}
	// Sized to content: the longest label present, so bars sit next to the text.
	f := board.Fleet{Rows: []board.Row{{Label: strings.Repeat("x", 30)}}}
	if got, _, _ := columns(f, 130); got != 30 {
		t.Errorf("label column = %d, want 30 (the longest label)", got)
	}
	// And the tail is sized to the longest workspace, not to a guess: it used to have
	// no width of its own at all, which is what wrapped the rows.
	f = board.Fleet{Rows: []board.Row{{Label: "x", Workspace: "platform-migration"}}}
	if _, got, _ := columns(f, 130); got != len("platform-migration") {
		t.Errorf("tail column = %d, want %d", got, len("platform-migration"))
	}
}
