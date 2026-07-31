package view

import (
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// Both renderers report trouble, and both read it off the same Fleet field. These tests
// exist to keep them from drifting apart: the whole point of deriving it in `board` is
// that the frame and the table cannot disagree about what is wrong (§3).

const screenCols = 118

func troubleScreen() Screen {
	return Screen{Now: time.Date(2026, 7, 28, 14, 30, 0, 0, time.UTC),
		Interval: 10 * time.Second, Threshold: 45 * time.Minute, Rows: 44, Cols: screenCols}
}

// The words board used for an empty fleet were "no sessions", which is a claim about
// the fleet. An unreadable roster is a claim about board, and saying the first when the
// second is true is the bug (EVIDENCE.md §9.26).
func TestHeaderSaysTheRosterCouldNotBeReadInsteadOfNoSessions(t *testing.T) {
	f := board.Fleet{Trouble: "claude not found · board doctor"}
	got := plain(Frame(f, troubleScreen(), UI{}))
	if !strings.Contains(got, "claude not found") {
		t.Errorf("the frame does not report the trouble:\n%s", firstLines(got, 4))
	}
	if strings.Contains(got, "no sessions") {
		t.Errorf("the frame still claims an empty fleet while the roster is unreadable:\n%s",
			firstLines(got, 4))
	}
}

// Nothing wrong, nothing said. "no sessions" is the right answer to a readable but
// empty world and must survive untouched.
func TestHeaderStillSaysNoSessionsWhenNothingIsWrong(t *testing.T) {
	got := plain(Frame(board.Fleet{}, troubleScreen(), UI{}))
	if !strings.Contains(got, "no sessions") {
		t.Errorf("an empty readable fleet lost its span:\n%s", firstLines(got, 4))
	}
}

// Rows that did arrive still get counted. With no cmux the background agents come
// through, and a header that reported only the trouble would hide the fleet it can see.
func TestHeaderKeepsTheCountWhenSomeRowsArrived(t *testing.T) {
	f := spanFleet(2, 1)
	f.Trouble = "cmux not found · board doctor"
	got := plain(Frame(f, troubleScreen(), UI{}))
	if !strings.Contains(got, "cmux not found") {
		t.Errorf("the trouble is missing:\n%s", firstLines(got, 4))
	}
	if !strings.Contains(got, "2 sessions") {
		t.Errorf("the rows that arrived are no longer counted:\n%s", firstLines(got, 4))
	}
}

// Trouble is painted with the validated warning colour. Not statusCritical: bare
// #d03b3b measures 2.91 against a dark background, which is why blocked is a filled
// badge and nothing else may use it as text (§6, §9.4).
func TestTroubleUsesTheValidatedWarningColour(t *testing.T) {
	f := board.Fleet{Trouble: "roster unreadable · board doctor"}
	got := Frame(f, troubleScreen(), UI{})
	if !strings.Contains(got, fg(statusWarning, "roster unreadable · board doctor")) {
		t.Error("the trouble is not painted with statusWarning")
	}
	if strings.Contains(got, fg(statusCritical, "roster unreadable · board doctor")) {
		t.Error("the trouble is painted in bare statusCritical, which fails contrast as text")
	}
}

// The one-shot table is piped and pasted, so the trouble leads: a reader who takes the
// first line, or `head`s the output, must not miss that everything below it is partial.
func TestTableLeadsWithTheTrouble(t *testing.T) {
	f := fixture()
	f.Trouble = "roster unavailable · board doctor"
	lines := strings.Split(strings.TrimRight(Table(f, 45*time.Minute), "\n"), "\n")
	if !strings.Contains(lines[0], "roster unavailable") {
		t.Errorf("first line = %q, want the trouble before the rows", lines[0])
	}
}

// No trouble, no line. A healthy fleet's table must keep exactly the shape the existing
// tests pin, or every piped capture gains a blank row.
func TestTableAddsNoLineWhenNothingIsWrong(t *testing.T) {
	f := fixture()
	lines := strings.Split(strings.TrimRight(Table(f, 45*time.Minute), "\n"), "\n")
	if len(lines) != 1+len(f.Rows)+2 {
		t.Errorf("got %d lines, want %d — a healthy table grew a line:\n%s",
			len(lines), 1+len(f.Rows)+2, Table(f, 45*time.Minute))
	}
}

// The two renderers must say the same thing, verbatim. This is the test that would fail
// if either grew its own phrasing for a fact the domain already named.
func TestBothRenderersReportTheSameTrouble(t *testing.T) {
	f := fixture()
	f.Trouble = "claude not found · board doctor"
	frame := plain(Frame(f, troubleScreen(), UI{}))
	table := Table(f, 45*time.Minute)
	for name, out := range map[string]string{"frame": frame, "table": table} {
		if !strings.Contains(out, f.Trouble) {
			t.Errorf("%s does not carry the fleet's own words %q:\n%s", name, f.Trouble, out)
		}
	}
}

// A trouble phrase is text in the header, so it is subject to the same invariant as
// everything else: the frame fits, whatever the width (§6, §9.10, §9.12).
func TestTroubleNeverWidensTheFrame(t *testing.T) {
	f := wideFleet()
	f.Trouble = "claude agents answered in a shape board does not know · board doctor"
	for _, cols := range []int{60, 80, 118} {
		s := troubleScreen()
		s.Cols = cols
		for i, l := range strings.Split(Frame(f, s, UI{}), "\n") {
			if w := plainW(l); w > cols {
				t.Errorf("cols=%d line %d is %d wide:\n%q", cols, i, w, plain(l))
			}
		}
	}
}
