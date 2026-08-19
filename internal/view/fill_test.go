package view

import (
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// narrowFleet is content that cannot fill a large monitor: few rows, short labels,
// short workspace names. Every column below is sized to what is there, so the frame
// stops well short of the right edge and the surplus is the thing under test.
func narrowFleet() board.Fleet {
	return board.Fleet{Workspaces: 2, Stale: 1, Oldest: 30 * time.Hour, Rows: []board.Row{
		{Key: "K-1", Label: "PT8", Repo: "CAP", Surface: "S-1",
			Idle: 30 * time.Hour, Rank: board.RankQuiet, Stale: true},
		{Key: "K-2", Label: "ASAF WH", Repo: "REVIEWS", Surface: "S-2",
			Idle: 4 * time.Hour, Rank: board.RankQuiet},
	}}
}

// widest is the printed width of the frame's widest line, and of its header, which is
// always the second line — Frame opens with a blank one.
func widest(frame string) (head, body int) {
	for i, l := range strings.Split(frame, "\n") {
		if i == 1 {
			head = plainW(l)
			continue
		}
		body = max(body, plainW(l))
	}
	return head, body
}

// The header spanned the terminal while the table stopped at its content, so on a wide
// monitor the clock floated a hundred columns from the nearest thing it described. The
// frame is one block: the header's right edge lands on the widest line below it,
// wherever that is. Now that the bars spend the surplus the two usually meet at the
// terminal's edge — but they meet because the header follows the frame, not because
// both happen to be measured against the same terminal (§9.29).
func TestHeaderEndsWhereTheFrameEnds(t *testing.T) {
	for _, cols := range []int{90, 140, 206, 300} {
		head, body := widest(Frame(narrowFleet(), screen(57, cols), UI{}))
		if head != body {
			t.Errorf("cols=%d: header ends at %d, the frame below it at %d", cols, head, body)
		}
	}
}

// The frame's width is a ceiling for the header, never a squeeze below what it has to
// say: a fleet too small to fill the line must not cost the clock.
func TestHeaderKeepsItsOwnWidthOverANarrowBody(t *testing.T) {
	got := Frame(board.Fleet{}, screen(57, 200), UI{Notice: "cmux focus refused"})
	head, _ := widest(got)
	line := plain(strings.Split(got, "\n")[1])
	if !strings.HasSuffix(line, "every 10s") {
		t.Errorf("header shed the interval against an empty fleet (%d wide): %q", head, line)
	}
	if !strings.Contains(line, "cmux focus refused") {
		t.Errorf("header shed the notice against an empty fleet: %q", line)
	}
}

// The header may narrow to the frame, but it may never outgrow the terminal: that is
// the wrap that scrolls the whole frame away (EVIDENCE.md §9.10).
func TestHeaderNeverExceedsTheTerminalWhateverTheBody(t *testing.T) {
	for _, cols := range []int{20, 40, 64, 90, 118} {
		head, _ := widest(Frame(wideFleet(), screen(44, cols), UI{Notice: "cmux focus refused"}))
		if head > cols {
			t.Errorf("cols=%d: header is %d wide", cols, head)
		}
	}
}

// outlierFleet is nine short labels and one long one: the shape that made the label
// column a statistic one row could set for the other nine.
func outlierFleet() board.Fleet {
	f := board.Fleet{Workspaces: 1}
	for range 9 {
		f.Rows = append(f.Rows, board.Row{Label: "twelve chars", Repo: "WS", Rank: board.RankQuiet})
	}
	f.Rows = append(f.Rows, board.Row{Label: strings.Repeat("x", 60), Repo: "WS", Rank: board.RankQuiet})
	return f
}

// Labels first, and in full while there is room for them: truncating is what the layout
// does under pressure, never something it does to a window with columns to spare.
func TestLabelColumnTakesTheWholeLabelWhenThereIsRoom(t *testing.T) {
	if labelW, _, _ := columns(outlierFleet(), 300, true); labelW != 60 {
		t.Errorf("label column = %d, want 60: the window can afford the whole label", labelW)
	}
}

// Only once the row stops fitting does one long title stop setting the column for the
// nine rows that would otherwise cross it as padding.
func TestLabelColumnFallsBackToP90WhenItDoesNotFit(t *testing.T) {
	labelW, _, _ := columns(outlierFleet(), 80, true)
	if labelW >= 60 {
		t.Errorf("label column = %d: the outlier still sets it at a width that cannot hold it", labelW)
	}
	if labelW < minLabelW {
		t.Errorf("label column = %d, under the floor %d", labelW, minLabelW)
	}
}

// A fleet whose labels are all long gets a column that holds them at any width that can
// afford it — the fallback is outlier resistance, not a policy of truncating.
func TestLabelColumnStillFitsAFleetOfLongLabels(t *testing.T) {
	f := board.Fleet{Workspaces: 1}
	for range 10 {
		f.Rows = append(f.Rows, board.Row{Label: strings.Repeat("x", 60), Repo: "WS", Rank: board.RankQuiet})
	}
	if labelW, _, _ := columns(f, 300, true); labelW != 60 {
		t.Errorf("label column = %d, want 60: every row needs it", labelW)
	}
}

// What the labels do not use goes to the bars, because that is where the gap was: the
// bar column sat at a fixed 12 cells and the surplus piled up to the right of the tail.
func TestBarsSpendWhatTheLabelsDoNotUse(t *testing.T) {
	f := narrowFleet()
	_, _, narrow := columns(f, 60, true)
	_, _, wide := columns(f, 200, true)
	// At its narrowest the bar is at least its base width, and the surplus only grows it. Not an
	// equality: how much surplus exists at 60 columns depends on what the rest of the chrome
	// costs, and the gutter stopped being a constant when it became elastic (§9.41). The
	// invariant is that a bar is never *cut* below its base — a shorter run on an absolute scale
	// reports a smaller number.
	if narrow < barCells {
		t.Errorf("bar is %d cells at 60 columns, below the base %d", narrow, barCells)
	}
	if wide <= narrow {
		t.Errorf("bar is %d cells at 200 columns and %d at 60: the surplus went nowhere", wide, narrow)
	}
}

// The row reaches the frame's right edge rather than stopping short of it — until the
// bar hits its cap, which is the one thing allowed to leave width unspent. Surplus that
// nothing wants is margin; surplus the bar could have taken is the bug (§9.29).
func TestARowSpendsTheWidthItIsGiven(t *testing.T) {
	for _, f := range []board.Fleet{narrowFleet(), wideFleet(), outlierFleet()} {
		for _, cols := range []int{70, 90, 120, 149, 200, 300} {
			labelW, tailW, barW := columns(f, cols, true)
			w := rowChrome(barW, gutterFor(f)) + labelW + tailW
			if w == cols-headMargin || barW == barMaxCells {
				continue
			}
			t.Errorf("cols=%d: the widest row is %d, leaving %d columns unspent with the bar at %d of %d",
				cols, w, cols-headMargin-w, barW, barMaxCells)
		}
	}
}

// A bar wide enough to be read is the point; a bar wide enough to be the row's loudest
// mark is not. Exactly one element shouts and it is BLOCKED (DESIGN.md §6).
func TestTheBarStopsGrowing(t *testing.T) {
	for _, cols := range []int{200, 300, 400} {
		if _, _, barW := columns(narrowFleet(), cols, true); barW != barMaxCells {
			t.Errorf("cols=%d: bar is %d cells, want the cap %d", cols, barW, barMaxCells)
		}
	}
}

// A tab too narrow for a whole bar still gets none: a cut bar is the same glyph run
// reporting a smaller number, on a scale that is supposed to be absolute.
func TestANarrowTabStillGetsNoBarRatherThanACutOne(t *testing.T) {
	if _, _, barW := columns(wideFleet(), 50, true); barW != 0 {
		t.Errorf("bar is %d cells at 50 columns, want none at all", barW)
	}
}
