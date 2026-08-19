package view

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/zalts1/dashy/internal/board"
)

// wideFleet is the width-hostile case: labels longer than any column, workspace names
// longer than the labels. Real data looks like this: a workspace name of this length
// is what pushed rows past the right edge.
func wideFleet() board.Fleet {
	f := board.Fleet{Workspaces: 3, Blocked: 1, Stale: 1, Oldest: 52 * 24 * time.Hour}
	f.Rows = []board.Row{
		{Label: "decide: ignore noise, guard hook in config, or investigate cmux settings?",
			Repo: "platform-migration", Surface: "S-BLK", Idle: 52 * 24 * time.Hour,
			Rank: board.RankBlocked, Stale: true},
		{Label: "short", Repo: "background", Surface: "S-RUN", Rank: board.RankWorking},
		{Label: strings.Repeat("x", 140), Repo: strings.Repeat("w", 60),
			Surface: "S-OLD", Idle: 50 * time.Hour, Rank: board.RankQuiet, Stale: true},
	}
	for i := 0; i < 12; i++ {
		f.Rows = append(f.Rows, board.Row{Label: "filler", Repo: "platform-migration",
			Surface: "S-F", Idle: time.Duration(i) * time.Hour, Rank: board.RankQuiet})
	}
	return f
}

// A line wider than the terminal wraps, and a wrapped line makes the frame occupy more
// screen rows than height() counted: the fit arithmetic silently under-reports, the
// terminal scrolls, and the header is the first thing gone (EVIDENCE.md §9.10). So no
// line may exceed the width — at any width, in any state.
func TestNoLineExceedsTheTerminalWidth(t *testing.T) {
	states := []UI{{}, {Sel: "S-OLD", Paused: true}, {Notice: "cmux focus refused"},
		// A typed line is unbounded user input on the one line the layout does not size,
		// so it belongs in this matrix as much as any workspace name does.
		{Typing: true, Paused: true},
		{Typing: true, Paused: true, Input: strings.Repeat("reply to the ACME export request ", 12)},
		// The fold changes the legend's contents and the band's line, both of which the
		// clamp has to hold at every width.
		{QuietCollapsed: true}, {QuietCollapsed: true, Sel: "S-OLD", Paused: true}}
	for _, cols := range []int{40, 52, 58, 64, 72, 80, 90, 100, 118, 140, 200, 300} {
		for _, rows := range []int{12, 20, 24, 44, 60} {
			for _, u := range states {
				for _, f := range []board.Fleet{wideFleet(), goldenFleet(), fixture(), {}} {
					out := Frame(f, Screen{Now: frameNow, Interval: 10 * time.Second,
						Threshold: 4 * time.Hour, Rows: rows, Cols: cols}, u)
					for i, line := range strings.Split(out, "\n") {
						if w := plainW(line); w > cols {
							t.Fatalf("cols=%d rows=%d %+v: line %d is %d wide, %d too many:\n%q",
								cols, rows, u, i+1, w, w-cols, plain(line))
						}
					}
				}
			}
		}
	}
}

// The bars have to stay put as the window changes, or every row's quantity has to be
// re-read from scratch after a resize. The label column absorbs the change instead.
func TestResizeKeepsTheColumnsAligned(t *testing.T) {
	f := wideFleet()
	for _, cols := range []int{72, 90, 118, 200} {
		labelW, tailW, barW := columns(f, cols, true)
		if labelW < minLabelW {
			t.Errorf("cols=%d: label column %d is under the floor %d", cols, labelW, minLabelW)
		}
		// The whole line has to fit by construction, not by the clamp catching it:
		// the clamp is a backstop for absurd widths, not part of the layout.
		if w := rowChrome(barW, gutterFor(f)) + labelW + tailW; w > cols {
			t.Errorf("cols=%d: widest row is %d by arithmetic, %d too many", cols, w, w-cols)
		}
	}
	// A tail long enough to squeeze the label takes the truncation itself once the
	// label is at its floor — the label is the meaning, the workspace is context.
	labelW, tailW, barW := columns(f, 64, true)
	if labelW != minLabelW {
		t.Errorf("label column = %d at 64 cols, want the floor %d", labelW, minLabelW)
	}
	if rowChrome(barW, gutterFor(f))+labelW+tailW > 64 {
		t.Errorf("tail did not give way: %d + %d + %d > 64", rowChrome(barW, gutterFor(f)), labelW, tailW)
	}
}

func TestClampLine(t *testing.T) {
	cases := []struct {
		name, in string
		w        int
		want     string
	}{
		{"short line untouched", "abc", 10, "abc"},
		{"exact fit untouched", "abc", 3, "abc"},
		{"plain text cut", "abcdef", 3, "abc\033[0m"},
		// Escape codes cost no columns, so they must not be counted or the visible text
		// gets cut a fifth of the way in.
		{"escapes are not columns", fg(inkPrimary, "abcde"), 5, fg(inkPrimary, "abcde")},
		{"cut inside a painted run resets colour", fg(inkPrimary, "abcde"), 3,
			"\033[38;2;255;255;255mabc\033[0m"},
		{"zero width", fg(inkPrimary, "abc"), 0, "\033[38;2;255;255;255m\033[0m"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampLine(c.in, c.w); got != c.want {
				t.Errorf("clampLine(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
			}
		})
	}
	// A cut must never land inside an escape sequence: the tail of a code would print
	// as literal text.
	if got := clampLine(dim("abcdef"), 4); strings.Contains(plain(got), "[38") {
		t.Errorf("clamp split an escape sequence: %q", got)
	}
}

// The tail is a labelled column, so it never sizes below its own header: an empty fleet
// that drops the word WORKSPACE reads as a column that is missing, not one that is empty.
func TestTailKeepsRoomForItsOwnHeader(t *testing.T) {
	if _, tailW, _ := columns(board.Fleet{}, 118, true); tailW < runes(wsHeader) {
		t.Errorf("tail column = %d on an empty fleet, want at least %d", tailW, runes(wsHeader))
	}
	if !strings.Contains(Frame(board.Fleet{}, screen(44, 118), UI{}), wsHeader) {
		t.Error("empty frame lost the WORKSPACE column header")
	}
}

// A clipped bar is worse than no bar: it is the same glyph run saying a smaller number,
// and bar length is a quantity on an absolute scale. So the bar column is drawn only
// where it fits whole, and the narrow layout drops it — along with its legend, since a
// key for a scale that is not drawn is noise.
func TestNarrowLayoutDropsTheBarRatherThanCutIt(t *testing.T) {
	for _, cols := range []int{34, 40, 45, 50, 57, 58, 64, 72, 90, 118, 300} {
		out := Frame(wideFleet(), Screen{Now: frameNow, Interval: 10 * time.Second,
			Threshold: 4 * time.Hour, Rows: 44, Cols: cols}, UI{})
		for i, line := range strings.Split(out, "\n") {
			// bar() always emits its unfilled cells as spaces, so a complete bar column
			// is never the last thing on a line. A cut one is.
			if strings.HasSuffix(strings.TrimRight(plain(line), " "), "▇") {
				t.Errorf("cols=%d: line %d ends mid-bar, understating an idle time:\n%q",
					cols, i+1, plain(line))
			}
		}
		_, _, barW := columns(wideFleet(), cols, true)
		bars := barW > 0
		if bars != strings.Contains(out, "▇") {
			t.Errorf("cols=%d: bars=%v but the frame %s bar glyphs", cols, bars,
				map[bool]string{true: "has", false: "has no"}[strings.Contains(out, "▇")])
		}
		if !bars && strings.Contains(out, "elapsed") {
			t.Errorf("cols=%d: legend drawn for a scale that is not: %q", cols, plain(out))
		}
	}
}

// The IDLE header and a row's duration are one column read two ways, so they share a
// right edge. ASKED's wait used to sit one column left of the rows' (EVIDENCE.md §9.12),
// which is exactly the kind of drift a resize makes visible — the band is gone, but the
// column arithmetic it exposed is still what this pins.
func TestDurationColumnsShareARightEdge(t *testing.T) {
	lines := strings.Split(plain(Frame(goldenFleet(), screen(44, 118), UI{})), "\n")
	// Counted in runes: the bar glyph is three bytes wide and one column wide, so a
	// byte index reported two identical columns as 26 apart.
	rightOf := func(marker, field string) int {
		for _, l := range lines {
			if strings.Contains(l, marker) {
				if i := strings.Index(l, field); i >= 0 {
					return utf8.RuneCountInString(l[:i]) + runes(field)
				}
				t.Fatalf("field %q not found in %q", field, l)
			}
		}
		t.Fatalf("no line containing %q", marker)
		return 0
	}
	head := rightOf(sessionHeader, "IDLE")
	blocked := rightOf("watch CI to green", "52d00h")
	quiet := rightOf("rotting thing", "2d02h")
	if head != blocked || blocked != quiet {
		t.Errorf("right edges: IDLE=%d blocked=%d quiet=%d, want all equal", head, blocked, quiet)
	}
}

// The KPI strip is the glance layer, so it sheds cells whole instead of being cut
// mid-word: "ol" where "oldest 52d00h" was reads as a rendering fault, not as a
// narrow window. Blocked is the one cell that never goes.
func TestKPIStripShedsCellsWhole(t *testing.T) {
	// Whole cells, in the order they are shed from the right.
	cells := []string{"1 BLOCKED", "1 working", "1 quiet >4h", "oldest 52d00h"}
	gaps := regexp.MustCompile(` +`)
	norm := func(s string) string { return strings.TrimSpace(gaps.ReplaceAllString(s, " ")) }

	for _, cols := range []int{40, 46, 50, 58, 64, 72, 118} {
		out := Frame(wideFleet(), Screen{Now: frameNow, Interval: 10 * time.Second,
			Threshold: 4 * time.Hour, Rows: 44, Cols: cols}, UI{})
		var strip string
		for _, l := range strings.Split(plain(out), "\n") {
			if strings.Contains(l, "BLOCKED") && !strings.Contains(l, "decide") {
				strip = norm(l)
				break
			}
		}
		want := ""
		for k := len(cells); k > 0; k-- {
			if c := norm(strings.Join(cells[:k], " ")); c == strip {
				want = c
				break
			}
		}
		if want == "" {
			t.Errorf("cols=%d: strip is not a whole-cell prefix: %q", cols, strip)
		}
		if !strings.Contains(strip, "BLOCKED") {
			t.Errorf("cols=%d: blocked count shed: %q", cols, strip)
		}
	}
}
