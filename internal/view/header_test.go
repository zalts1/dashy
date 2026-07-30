package view

import (
	"regexp"
	"strings"
	"testing"

	"github.com/zalts1/dashy/internal/board"
)

var ansi = regexp.MustCompile(`\033\[[0-9;]*m`)

// plainW is the printed width of a rendered string: escape codes cost no columns, so
// len() would overstate every measurement in this file by a factor of five.
func plainW(s string) int { return len([]rune(ansi.ReplaceAllString(s, ""))) }

func plain(s string) string { return ansi.ReplaceAllString(s, "") }

func spanFleet(sessions, workspaces int) board.Fleet {
	f := board.Fleet{Workspaces: workspaces}
	for i := 0; i < sessions; i++ {
		f.Rows = append(f.Rows, board.Row{Label: "x", Workspace: "WS"})
	}
	return f
}

// The header states the fleet's span: how much work exists and how far it is spread.
// The workspace count is the one fact no band below carries.
func TestHeaderStatesTheSpan(t *testing.T) {
	cases := []struct {
		name string
		f    board.Fleet
		want string
	}{
		{"plural", spanFleet(31, 6), "31 sessions in 6 workspaces"},
		{"singular", spanFleet(1, 1), "1 session in 1 workspace"},
		// All background: there are sessions but no tabs, so there is no span to state.
		{"no workspaces", spanFleet(4, 0), "4 sessions"},
		{"empty fleet", board.Fleet{}, "no sessions"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := plain(header(c.f, screen(44, 118), UI{}))
			if !strings.Contains(got, c.want) {
				t.Errorf("header = %q, want it to state %q", got, c.want)
			}
			if c.name == "no workspaces" && strings.Contains(got, "workspace") {
				t.Errorf("header = %q, invented a workspace span", got)
			}
		})
	}
}

// Freshness sits on the right edge, mirroring the left indent. Two anchors are what
// make the line read as a title bar instead of a sentence trailing off into dead space.
func TestHeaderFreshnessIsRightAligned(t *testing.T) {
	for _, cols := range []int{90, 118, 200} {
		got := header(spanFleet(31, 6), screen(44, cols), UI{})
		if w := plainW(got); w != cols-2 {
			t.Errorf("cols=%d: header width %d, want %d (2-column right margin)", cols, w, cols-2)
		}
		if !strings.HasSuffix(plain(got), "every 10s") {
			t.Errorf("cols=%d: header does not end in the refresh interval: %q", cols, plain(got))
		}
	}
}

// A wrapped header is a frame one line taller than measured, which scrolls the whole
// thing (EVIDENCE.md §9.10). It must fit any width, shedding chrome before meaning.
func TestHeaderNeverWraps(t *testing.T) {
	for _, cols := range []int{20, 34, 46, 58, 70, 90, 118, 300} {
		for _, u := range []UI{{}, {Paused: true}, {Notice: "cmux focus refused"}} {
			got := header(spanFleet(31, 6), screen(44, cols), u)
			if strings.Contains(got, "\n") {
				t.Fatalf("cols=%d: header is more than one line", cols)
			}
			if w := plainW(got); w > cols {
				t.Errorf("cols=%d %+v: header is %d wide, %d too many", cols, u, w, w-cols)
			}
		}
	}
	// Order of sacrifice: the interval is config, the span is context, the count is the
	// point. A narrow tab keeps the count.
	narrow := plain(header(spanFleet(31, 6), screen(44, 46), UI{}))
	if strings.Contains(narrow, "every") {
		t.Errorf("narrow header kept the refresh interval over the span: %q", narrow)
	}
	if !strings.Contains(narrow, "31 sessions") {
		t.Errorf("narrow header dropped the session count: %q", narrow)
	}
}

// While a selection is live the data stops refreshing, so a clock would be stating a
// time that is no longer true. The mode takes the clock's place, not a slot beside it.
func TestHeaderPausedReplacesTheClock(t *testing.T) {
	got := plain(header(spanFleet(31, 6), screen(44, 118), UI{Paused: true}))
	if strings.Contains(got, "14:30:00") {
		t.Errorf("paused header still shows a clock: %q", got)
	}
	if !strings.Contains(got, "esc to resume") {
		t.Errorf("paused header does not say how to get out: %q", got)
	}
}

// The board tab is usually hidden by the time a jump fails, so the notice has to be in
// the frame — and legible. Bare #d03b3b measures 2.91 and is reserved for the badge
// fill (EVIDENCE.md §9.4).
func TestHeaderNoticeIsLegible(t *testing.T) {
	got := header(spanFleet(31, 6), screen(44, 118), UI{Notice: "cmux focus refused", Paused: true})
	if !strings.Contains(plain(got), "cmux focus refused") {
		t.Error("notice missing from the header")
	}
	if strings.Contains(got, fg(statusCritical, "cmux focus refused")) {
		t.Error("notice painted in bare critical red, which fails contrast at 2.91")
	}
}

func TestHeaderTitleStaysTheAnchor(t *testing.T) {
	got := header(spanFleet(31, 6), screen(44, 118), UI{})
	if !strings.HasPrefix(plain(got), "  BOARD") {
		t.Errorf("header does not lead with the title: %q", plain(got))
	}
	// Exactly one element is allowed to shout, and it is BLOCKED — so the title is
	// bright ink, never a fill.
	if strings.Contains(got, "\033[48;") {
		t.Error("header painted a background fill")
	}
	if !strings.Contains(got, fg(inkPrimary, "BOARD")) {
		t.Error("title is not in primary ink")
	}
}
