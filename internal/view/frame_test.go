package view

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
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
		{Key: "K-BLK", State: "blocked →", Label: "merge app#1497", Repo: "APP", Surface: "S-BLK", Idle: 3 * time.Hour, Rank: board.RankBlocked},
		{Key: "K-BG", State: "blocked →", Label: "no tab to jump to", Repo: "background", Idle: time.Hour, Rank: board.RankBlocked},
		{Key: "K-OLD", State: "done", Label: "rotting thing", Repo: "REVIEWS", Surface: "S-OLD", Idle: 50 * time.Hour, Rank: board.RankQuiet, Stale: true},
		{Key: "K-NEW", State: "done", Label: "fresh thing", Repo: "TASKS", Surface: "S-NEW", Idle: 5 * time.Minute, Rank: board.RankQuiet},
		{Key: "K-RUN", State: "running", Label: "busy thing", Repo: "KILL", Surface: "S-RUN", Rank: board.RankWorking},
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
			Repo: "W", Surface: "S-F", Idle: time.Duration(40-i) * time.Hour, Rank: board.RankQuiet})
	}
	f.Rows = append(f.Rows, board.Row{Key: "K-BGQ", Label: "background agent, no tab",
		Repo: "background", Idle: time.Minute, Rank: board.RankQuiet})
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
	// Matched on a prefix, because whether the label prints whole is the label column's
	// business and not this test's: 40 of these 46 rows are "filler", so the p90 column
	// sits at its floor and this row draws as "background agent,…" (§9.29).
	const tabless = "background agent"
	if out := Frame(f, screen(20, 130), UI{}); strings.Contains(out, tabless) {
		t.Fatal("fixture is not exercising the collapse; the tab-less row is already visible")
	}
	sel := Frame(f, screen(20, 130), UI{Sel: "K-BGQ", Paused: true})
	if !strings.Contains(sel, tabless) {
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
	wide := board.Fleet{Rows: []board.Row{{Label: strings.Repeat("x", 200), Repo: "APP"}}}
	// avail is what the elastic columns share once the fixed chrome and the right
	// margin are paid for.
	// Measured against a base-width bar: these are the cases where the label is what the
	// terminal is short of, so there is no surplus for the bar to have taken.
	avail := func(cols int) int { return cols - headMargin - rowChrome(barCells, gutterFor(wide)) }
	cases := []struct {
		name string
		f    board.Fleet
		cols int
		want int
	}{
		{"short labels do not shrink below the floor", fixture(), 130, minLabelW},
		// The tail keeps its floor even for a 3-letter repo, so the label gets
		// what is left of avail, not all of it.
		{"a long label is capped by the terminal", wide, 90, avail(90) - runes(wsHeader)},
		{"and by the absolute cap on a wide terminal", wide, 400, maxLabelW},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _, _ := columns(c.f, c.cols, true); got != c.want {
				t.Errorf("label column = %d, want %d", got, c.want)
			}
		})
	}
	// A narrow terminal may not push the label below its floor. Asserted as a floor rather than
	// as an exact width, which is what it always meant: how many columns the label gets at 50
	// depends on what the location column's own floor costs, and that changed when the header
	// became REPO (§9.39). The invariant is the floor, not the arithmetic that lands on it.
	if got, _, _ := columns(wide, 50, true); got < minLabelW {
		t.Errorf("label column = %d at 50 cols, below the floor %d", got, minLabelW)
	}

	// Sized to content: the longest label present, so bars sit next to the text.
	f := board.Fleet{Rows: []board.Row{{Label: strings.Repeat("x", 30)}}}
	if got, _, _ := columns(f, 130, true); got != 30 {
		t.Errorf("label column = %d, want 30 (the longest label)", got)
	}
	// And the tail is sized to the longest location, not to a guess: it used to have
	// no width of its own at all, which is what wrapped the rows.
	f = board.Fleet{Rows: []board.Row{{Label: "x", Repo: "platform-migration"}}}
	if _, got, _ := columns(f, 130, true); got != len("platform-migration") {
		t.Errorf("tail column = %d, want %d", got, len("platform-migration"))
	}
}

// The quiet band folds on a key, and the fold is the viewer's, not the fit loop's: it
// starts expanded, and what it costs when folded is one line. The count stays on that
// line because a band collapsed to its count still reports — nothing may leave the frame
// silently (§9.14).
func TestQuietFolds(t *testing.T) {
	f := fixture()
	open := Frame(f, screen(44, 130), UI{})
	folded := Frame(f, screen(44, 130), UI{QuietCollapsed: true})

	t.Run("expanded is the default", func(t *testing.T) {
		if !strings.Contains(open, "rotting thing") || !strings.Contains(open, "fresh thing") {
			t.Error("quiet rows are not drawn without the fold being asked for")
		}
		if strings.Contains(open, "collapsed") {
			t.Error("an unfolded band says it is collapsed")
		}
	})

	t.Run("folded draws the count and no rows", func(t *testing.T) {
		for _, label := range []string{"rotting thing", "fresh thing"} {
			if strings.Contains(folded, label) {
				t.Errorf("folded band still draws %q", label)
			}
		}
		if !strings.Contains(folded, "QUIET") {
			t.Error("the band header went with the rows")
		}
		// Two quiet rows in the fixture, and the fold must not lie about how many.
		if !regexp.MustCompile(`QUIET.*2`).MatchString(folded) {
			t.Error("folded band does not report its count")
		}
		if !strings.Contains(folded, "collapsed") {
			t.Error("folded band does not say the rows are hidden by choice")
		}
	})

	t.Run("folding buys screen rows back", func(t *testing.T) {
		if height(folded) >= height(open) {
			t.Errorf("folded frame is %d lines, open is %d — the fold bought nothing",
				height(folded), height(open))
		}
	})

	t.Run("the key is named in both states", func(t *testing.T) {
		// §9.14: a key that exists should say so, and it is the only way to get the rows
		// back once they are folded away.
		if !strings.Contains(open, "z fold") {
			t.Error("the fold key is not named while the band is open")
		}
		if !strings.Contains(folded, "z unfold") {
			t.Error("the unfold key is not named while the band is folded")
		}
	})

	t.Run("no quiet band, no key", func(t *testing.T) {
		bare := board.Fleet{Rows: []board.Row{
			{Key: "K-RUN", State: "running", Label: "busy thing", Repo: "KILL", Rank: board.RankWorking},
		}}
		if strings.Contains(Frame(bare, screen(44, 130), UI{}), "fold") {
			t.Error("a fold key offered for a band that is not there")
		}
	})
}

// Navigation follows the screen, so a folded row is not a stop. Stepping into a row the
// frame is not drawing would put the caret somewhere invisible and point Enter at it.
func TestFoldedRowsAreNotNavigationStops(t *testing.T) {
	f := fixture()
	open := DisplayOrder(f, UI{})
	folded := DisplayOrder(f, UI{QuietCollapsed: true})
	for _, k := range []string{"K-OLD", "K-NEW"} {
		if !contains(open, k) {
			t.Errorf("%s is not a stop while the band is open", k)
		}
		if contains(folded, k) {
			t.Errorf("%s is still a stop while the band is folded", k)
		}
	}
	// The other bands are untouched: folding QUIET must not cost the blocked rows.
	for _, k := range []string{"K-BLK", "K-BG", "K-RUN"} {
		if !contains(folded, k) {
			t.Errorf("folding QUIET dropped %s from the order", k)
		}
	}
}

func contains(s []string, k string) bool {
	for _, v := range s {
		if v == k {
			return true
		}
	}
	return false
}
