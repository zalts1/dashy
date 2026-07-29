package view

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"board/internal/board"
)

// withTodos returns the standard fixture plus todo rows, oldest first as Build sorts
// them.
func withTodos(f board.Fleet, texts ...string) board.Fleet {
	f.TodoCap = 10
	for i, text := range texts {
		f.Rows = append(f.Rows, board.Row{
			Key:   fmt.Sprintf("todo:t%d", i),
			State: "todo",
			Label: text,
			Idle:  time.Duration(len(texts)-i) * 24 * time.Hour,
			Rank:  board.RankTodo,
		})
	}
	return f
}

// The list follows the fleet: it is not a state a session can be in, and drawing it
// between WORKING and QUIET put it inside a ranking of process states. This is also the
// rank order — RankTodo is last (DESIGN.md §12, EVIDENCE.md §9.19).
func TestTodoBandFollowsTheWholeFleet(t *testing.T) {
	out := Frame(withTodos(fixture(), "ACME csv export"), screen(44, 130), UI{})
	iWork, iQuiet, iTodo := strings.Index(out, "WORKING"), strings.Index(out, "QUIET"), strings.Index(out, "TODO")
	if iTodo < 0 {
		t.Fatal("no TODO band drawn")
	}
	if !(iWork < iQuiet && iQuiet < iTodo) {
		t.Errorf("band order = WORKING@%d QUIET@%d TODO@%d, want the list last",
			iWork, iQuiet, iTodo)
	}
	if !strings.Contains(out, "ACME csv export") {
		t.Error("todo text not drawn")
	}
}

// A bar means rot on the idle scale. A session's idle time is a gap that resets; a
// todo's age is a lifetime that only grows. Drawing both as the same glyph at the same
// scale made "0m" read as "active this minute" (§9.19). WORKING rows already have no bar
// for the same class of reason.
func TestTodoRowsCarryNoBarAndNoWorkspace(t *testing.T) {
	f := withTodos(fixture(), "ACME csv export")
	row := todoLineOf(t, Frame(f, screen(44, 130), UI{}), "ACME csv export")
	if strings.Contains(row, "▇") {
		t.Errorf("todo row draws an idle bar:\n%q", row)
	}
	// Its age is stated as a lifetime, not as a gap in the IDLE column's vocabulary.
	if !strings.Contains(row, "ago") {
		t.Errorf("todo row does not say how long ago:\n%q", row)
	}
	if strings.Contains(row, "1d00h") {
		t.Errorf("todo row uses the idle format:\n%q", row)
	}
	// Session rows keep theirs: this is a change to one band, not to the scale.
	if session := todoLineOf(t, Frame(f, screen(44, 130), UI{}), "rotting thing"); !strings.Contains(session, "▇") {
		t.Errorf("a quiet session lost its bar:\n%q", session)
	}
}

// The cap is stated where you watch it climb, so the refusal at 11 is not a surprise.
func TestTodoBandStatesTheCap(t *testing.T) {
	f := withTodos(fixture(), "one", "two", "three")
	f.TodoCap = 10
	out := Frame(f, screen(44, 130), UI{})
	if !strings.Contains(out, "3 of 10") {
		t.Errorf("the band does not state the cap:\n%s", bandOf(out, "TODO"))
	}
}

// The list absorbs a squeeze the way the quiet tail does — a floor plus a count — so
// its position is a reading decision rather than a way of surviving a short tab.
func TestTodoBandCollapsesToItsOwnFloorAfterQuiet(t *testing.T) {
	f := fixture()
	for i := range 20 {
		f.Rows = append(f.Rows, board.Row{Key: fmt.Sprintf("K-F%d", i), State: "done",
			Label: "filler", Workspace: "W", Surface: "S-F",
			Idle: time.Duration(20-i) * time.Hour, Rank: board.RankQuiet})
	}
	f = withTodos(f, "t-one", "t-two", "t-three", "t-four", "t-five", "t-six")

	out := plain(Frame(f, screen(26, 110), UI{}))
	if !strings.Contains(out, "TODO") {
		t.Fatalf("the list vanished on a 26-row tab:\n%s", out)
	}
	// The quiet tail gives way first, all the way to its count: a band collapsed to
	// "QUIET · 22" still reports, which is why shedding its rows beats losing the list.
	if strings.Contains(out, "○ filler") {
		t.Error("quiet rows survived while the list was being cut; the order is wrong")
	}
	if !strings.Contains(out, "QUIET · 22") {
		t.Errorf("the quiet count stopped reporting:\n%s", out)
	}
	// The oldest todos are kept and the newest counted, exactly as the tail does (§9.14).
	if !strings.Contains(out, "t-one") {
		t.Error("the oldest todo was collapsed away; the collapse cuts from the wrong end")
	}
	if !strings.Contains(out, "+4 todo") {
		t.Errorf("nothing reports the hidden todos:\n%s", bandOf(out, "TODO"))
	}

	// Squeezed further, the rows go and both counts stay. That is the floor of honesty:
	// nothing disappears without saying it did.
	tighter := plain(Frame(f, screen(22, 110), UI{}))
	for _, want := range []string{"QUIET · 22", "TODO · 6 of 10", "+6 todo"} {
		if !strings.Contains(tighter, want) {
			t.Errorf("a 22-row tab lost %q:\n%s", want, tighter)
		}
	}
	for _, rows := range []int{26, 22, 18, 14} {
		if h := height(Frame(f, screen(rows, 110), UI{})); h > rows {
			t.Errorf("frame is %d lines in a %d-row tab", h, rows)
		}
	}
}

// todoLineOf returns the rendered line carrying label, with colour stripped.
func todoLineOf(t *testing.T, frame, label string) string {
	t.Helper()
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(plain(l), label) {
			return plain(l)
		}
	}
	t.Fatalf("no line carrying %q", label)
	return ""
}

// bandOf returns the frame from a band header down, for readable failures.
func bandOf(frame, header string) string {
	if i := strings.Index(plain(frame), header); i >= 0 {
		return plain(frame)[i:]
	}
	return plain(frame)
}

// A band earns its lines by exception (EVIDENCE.md §9.13): with nothing on the list
// there is nothing to report, so there is no band and no placeholder.
func TestNoTodosDrawsNoBand(t *testing.T) {
	if out := Frame(fixture(), screen(44, 130), UI{}); strings.Contains(out, "TODO") {
		t.Error("an empty todo list still drew a band")
	}
}

// The trim order §12 settles: the quiet tail gives way first, so an ordinary short tab
// still shows the whole list.
func TestTodosSurviveTheCollapse(t *testing.T) {
	f := fixture()
	for i := range 30 {
		f.Rows = append(f.Rows, board.Row{Key: fmt.Sprintf("K-F%d", i), State: "done",
			Label: "filler", Workspace: "W", Surface: "S-F",
			Idle: time.Duration(30-i) * time.Hour, Rank: board.RankQuiet})
	}
	out := Frame(withTodos(f, "reply to the questionnaire", "ACME csv export"), screen(24, 110), UI{})
	if !strings.Contains(out, "+") || !strings.Contains(out, "quiet") {
		t.Fatal("the fleet did not collapse; this test is not exercising the trim")
	}
	for _, want := range []string{"reply to the questionnaire", "ACME csv export"} {
		if !strings.Contains(out, want) {
			t.Errorf("collapse dropped the todo %q — the reminder vanished when the screen got busy", want)
		}
	}
}

// Navigation follows the screen, so the list is the last stop — walking down past the
// quiet tail arrives at the todos (§9.19).
func TestDisplayOrderEndsWithTheTodoList(t *testing.T) {
	got := DisplayOrder(withTodos(fixture(), "a todo"))
	want := []string{"K-BLK", "K-BG", "K-RUN", "K-OLD", "K-NEW", "todo:t0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("display order: got %v want %v", got, want)
	}
}

// §9.14 in reverse: a frame must not promise a key it does not have, and `d` only
// exists while a todo is selected — so it is advertised then and not before.
func TestTheDoneKeyIsAdvertisedOnlyWhenItWorks(t *testing.T) {
	f := withTodos(fixture(), "ACME csv export")
	if out := Frame(f, screen(44, 130), UI{}); strings.Contains(out, "d done") {
		t.Error("the done key is advertised with nothing selected, where it does nothing")
	}
	if out := Frame(f, screen(44, 130), UI{Sel: "K-OLD", Paused: true}); strings.Contains(out, "d done") {
		t.Error("the done key is advertised on a session row, where it must never fire")
	}
	out := Frame(f, screen(44, 130), UI{Sel: "todo:t0", Paused: true})
	if !strings.Contains(out, "d done") {
		t.Error("a selected todo does not advertise the key that removes it")
	}
}

// Typing takes over the legend's line rather than adding one, so entering the mode
// cannot change the frame's height — a taller frame would collapse the quiet tail under
// the typist, and the fit loop would be doing it for a mode that lasts three seconds
// (§12).
func TestTypingReplacesTheLegendInItsOwnSlot(t *testing.T) {
	f := withTodos(fixture(), "ACME csv export")
	ambient := Frame(f, screen(44, 130), UI{})
	typing := Frame(f, screen(44, 130), UI{Typing: true, Input: "reply to the quest", Paused: true})

	if got, want := height(typing), height(ambient); got != want {
		t.Errorf("typing frame is %d lines, ambient is %d: the mode moved the layout", got, want)
	}
	if !strings.Contains(typing, "new todo") {
		t.Error("no prompt drawn while typing")
	}
	if !strings.Contains(typing, "reply to the quest") {
		t.Error("the typed text is not shown; a prompt you cannot read is worse than none")
	}
	if strings.Contains(typing, "ctrl-c to exit") {
		t.Error("the legend is still drawn while typing; the prompt is meant to replace it")
	}
}

// The header is where mode changes go, and the two ways out have to be named there:
// nothing else on screen says how to escape a mode.
func TestTypingIsNamedInTheHeaderWithBothExits(t *testing.T) {
	out := firstLines(Frame(fixture(), screen(44, 130), UI{Typing: true, Paused: true}), 3)
	for _, want := range []string{"typing", "enter", "esc"} {
		if !strings.Contains(out, want) {
			t.Errorf("header does not name %q:\n%s", want, out)
		}
	}
	// The clock has to go: it would be stating a freshness that stopped being true when
	// the refresh paused.
	if strings.Contains(out, "every") {
		t.Errorf("the cadence is still claimed while typing:\n%s", out)
	}
}

// An empty prompt still has to look like a prompt, or the first keystroke lands with no
// evidence the mode is on.
func TestAnEmptyPromptStillReadsAsOne(t *testing.T) {
	out := Frame(fixture(), screen(44, 130), UI{Typing: true, Paused: true})
	if !strings.Contains(out, "new todo") {
		t.Error("an empty input drew no prompt")
	}
}

// `a` is always available, unlike `d`, so it is always named (§9.14 cuts both ways).
func TestTheAddKeyIsAdvertised(t *testing.T) {
	if out := Frame(fixture(), screen(44, 130), UI{}); !strings.Contains(out, "a new todo") {
		t.Error("the add key is never advertised; nothing else on screen would teach it")
	}
}

// The one-shot list is the capture surface's receipt: it has to show the cap, because
// the cap is the part that will bite (§12).
func TestTodoListShowsAgeAndTheCap(t *testing.T) {
	out := Todos(withTodos(board.Fleet{}, "reply to ACME", "review the export PR"))
	for _, want := range []string{"reply to ACME", "review the export PR", "2d ago", "2 of 10"} {
		if !strings.Contains(out, want) {
			t.Errorf("list is missing %q:\n%s", want, out)
		}
	}
	// Sessions are not todos: this surface lists the list, nothing else.
	if full := Todos(withTodos(fixture(), "a todo")); strings.Contains(full, "rotting thing") {
		t.Errorf("session rows leaked into the todo list:\n%s", full)
	}
	empty := Todos(board.Fleet{})
	if !strings.Contains(empty, "no todos") {
		t.Errorf("empty list = %q, want it to say so", empty)
	}
}

// The header states the size of the fleet. Todos are not the fleet: three reminders
// must not read as three more sessions running (§12).
func TestHeaderCountsSessionsOnly(t *testing.T) {
	out := Frame(withTodos(fixture(), "one", "two", "three"), screen(44, 130), UI{})
	if !strings.Contains(out, "5 sessions") {
		t.Errorf("header does not report 5 sessions:\n%s", firstLines(out, 3))
	}
	// All todos and no sessions is an honest "no sessions" plus a TODO band, not a
	// fleet of three.
	only := Frame(withTodos(board.Fleet{}, "one"), screen(44, 130), UI{})
	if !strings.Contains(only, "no sessions") {
		t.Errorf("a todo-only fleet does not read as no sessions:\n%s", firstLines(only, 3))
	}
	if !strings.Contains(only, "TODO") {
		t.Error("a todo-only fleet drew no todo band")
	}
}

func firstLines(s string, n int) string {
	if lines := strings.SplitN(s, "\n", n+1); len(lines) > n {
		return strings.Join(lines[:n], "\n")
	}
	return s
}

// The table's headline count is a count of sessions. Folding todos into it would report
// a fleet larger than the one running (§12).
func TestTableCountsTodosApartFromSessions(t *testing.T) {
	out := Table(withTodos(fixture(), "ACME csv export", "review Jeff's PR"), 45*time.Minute)
	if !strings.Contains(out, "5 sessions") {
		t.Errorf("summary does not report 5 sessions:\n%s", out)
	}
	if !strings.Contains(out, "2 todo") {
		t.Errorf("summary does not report the todos:\n%s", out)
	}
	if !strings.Contains(out, "ACME csv export") {
		t.Error("todo missing from the table body")
	}
	// No todos, no cell: the same exception rule the band follows.
	if plain := Table(fixture(), 45*time.Minute); strings.Contains(plain, "todo") {
		t.Errorf("empty todo list still drew a cell:\n%s", plain)
	}
}

// Walking to the list means stepping past every quiet row, which on a real fleet is
// fifteen presses to reach a note. `t` goes straight there (DESIGN.md §12).
func TestFirstTodoIsTheListsEntryPoint(t *testing.T) {
	if got := FirstTodo(withTodos(fixture(), "oldest", "newer")); got != "todo:t0" {
		t.Errorf("FirstTodo = %q, want the oldest todo — the top of the list", got)
	}
	// Nothing to jump to is empty, not the first row of something else: an empty string
	// is the ambient selection, so the caller can report instead of moving the cursor.
	if got := FirstTodo(fixture()); got != "" {
		t.Errorf("FirstTodo on a fleet with no todos = %q, want empty", got)
	}
}

// §9.14: never name a key that does nothing. `t` exists only when the list does.
func TestTheListKeyIsAdvertisedOnlyWhenThereIsAList(t *testing.T) {
	if out := Frame(fixture(), screen(44, 130), UI{}); strings.Contains(out, "t list") {
		t.Error("the list key is advertised with no todos to jump to")
	}
	if out := Frame(withTodos(fixture(), "a todo"), screen(44, 130), UI{}); !strings.Contains(out, "t list") {
		t.Error("the list key is not advertised when there is a list")
	}
}
