package maki

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The shape the plugin writes: one file per cmux tab, holding what
// `maki.session.live()` answered. Several sessions per tab is the normal case — one
// maki process holds every session you have opened in that tab.
const reportJSON = `{
  "surface": "S-1",
  "cwd": "/w/api",
  "sessions": [
    {"id":"m1","title":"build the csv export endpoint","status":"working","updated_at":1754145000},
    {"id":"m2","title":"rotate the staging credentials","status":"needs_input","updated_at":1754140000},
    {"id":"m3","title":"New session","status":"idle","updated_at":1754100000}
  ]
}`

func TestParseReport(t *testing.T) {
	got, err := parseReport([]byte(reportJSON))
	if err != nil {
		t.Fatalf("well-formed report did not parse: %v", err)
	}
	if got.Surface != "S-1" || got.Cwd != "/w/api" {
		t.Errorf("surface/cwd = %q/%q, want S-1//w/api", got.Surface, got.Cwd)
	}
	if len(got.Sessions) != 3 {
		t.Fatalf("parsed %d sessions, want 3", len(got.Sessions))
	}
	want := []struct{ blocked, running bool }{
		{false, true},  // working
		{true, false},  // needs_input: a permission prompt or a question is a human's turn
		{false, false}, // idle
	}
	for i, w := range want {
		s := got.Sessions[i]
		if s.Blocked() != w.blocked || s.Running() != w.running {
			t.Errorf("%s: blocked/running = %v/%v, want %v/%v",
				s.ID, s.Blocked(), s.Running(), w.blocked, w.running)
		}
	}
}

// The clock is the session's own updated_at, which is why no file mtime is consulted:
// a session idle for three hours writes nothing, and its report still says when it
// last moved.
func TestLastActivityReadsTheSessionsOwnClock(t *testing.T) {
	got, err := parseReport([]byte(reportJSON))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Unix(1754145000, 0); !got.Sessions[0].LastActivity().Equal(want) {
		t.Errorf("LastActivity = %v, want %v", got.Sessions[0].LastActivity(), want)
	}
	// Zero must stay zero rather than becoming 1970: Build reads a zero clock as "no
	// clock" and shows no idle time, where an epoch timestamp would show 56 years.
	if got := (Session{}).LastActivity(); !got.IsZero() {
		t.Errorf("LastActivity with no updated_at = %v, want the zero time", got)
	}
}

// Same rule as the claude roster (§9.26): unreadable and empty are different facts, and
// the error is half the answer.
func TestParseReportSaysWhenItCouldNotRead(t *testing.T) {
	_, err := parseReport([]byte("not json"))
	if err == nil {
		t.Fatal("garbage parsed without error; an unreadable report reads as an absent one")
	}
	if !errors.Is(err, ErrUnreadable) {
		t.Errorf("error = %v, want it to wrap ErrUnreadable", err)
	}
}

// A number Lua wrote as a float must still land on the right second. maki's Lua encodes
// updated_at through a double, so 1.754145e9 is a shape board has to accept.
func TestParseReportAcceptsAFloatingPointClock(t *testing.T) {
	got, err := parseReport([]byte(`{"surface":"S-1","sessions":[{"id":"m1","updated_at":1.754145e9}]}`))
	if err != nil {
		t.Fatalf("a float clock did not parse: %v", err)
	}
	if want := time.Unix(1754145000, 0); !got.Sessions[0].LastActivity().Equal(want) {
		t.Errorf("LastActivity = %v, want %v", got.Sessions[0].LastActivity(), want)
	}
}

func TestParsePids(t *testing.T) {
	got := parsePids([]byte("501\n733\n\n"))
	if len(got) != 2 || got[0] != 501 || got[1] != 733 {
		t.Errorf("parsePids = %v, want [501 733]", got)
	}
	// Ascending, because Build walks this slice to make rows and two rows that tie on
	// band and idle time must not swap between ticks.
	if got := parsePids([]byte("733\n501\n")); got[0] != 501 {
		t.Errorf("parsePids = %v, want it sorted", got)
	}
	if got := parsePids([]byte("")); len(got) != 0 {
		t.Errorf("parsePids on no output = %v, want nothing", got)
	}
}

// readReports is the directory half. A missing directory is the fresh answer — maki has
// never reported on this machine — and must not read as a failure, or every machine
// without maki would cry trouble.
func TestReadReportsWithNoDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := readReports()
	if err != nil {
		t.Errorf("a missing roster directory reported an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d reports from a machine with none", len(got))
	}
}

func TestReadReportsKeysBySurface(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".board", "maki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("S-1.json", reportJSON)
	// A report naming no tab cannot be joined to one, so it is not a row and not an
	// error: a maki started outside cmux is a session board has nowhere to send you.
	write("nowhere.json", `{"surface":"","sessions":[{"id":"m9"}]}`)
	// Not ours, and not a report. The directory belongs to board, but a stray file in it
	// must not take the whole roster down with it.
	write("notes.txt", "hello")

	got, err := readReports()
	if err != nil {
		t.Fatalf("readReports: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d reports, want only the one naming a tab: %v", len(got), got)
	}
	if _, ok := got["S-1"]; !ok {
		t.Errorf("reports keyed %v, want the surface id as the key", got)
	}
}

// One unreadable file fails the read rather than being skipped. A report board cannot
// parse is a session it cannot see, and quietly dropping it is exactly the silence
// §9.26 was about — the shape here is written by a plugin board ships, so a parse
// failure means the two have gone out of step and the reader needs to know.
func TestReadReportsRefusesAnUnreadableFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".board", "maki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "S-1.json"), []byte("{oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReports(); !errors.Is(err, ErrUnreadable) {
		t.Errorf("error = %v, want it to wrap ErrUnreadable", err)
	}
}
