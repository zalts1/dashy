package board

import (
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/maki"
)

// maki sessions are rows like any other, joined by the same key claude's are: the pid,
// which cmux turns into a tab. What is different is where the state comes from and how
// liveness is settled, and those are the rules pinned here.

// makiSnap is a fleet of one maki process in one tab, holding the given sessions.
func makiSnap(sessions ...maki.Session) Snapshot {
	s := Snapshot{
		Titles: map[int]cmux.Titles{700: {ID: "S-M", Surface: "maki tab", Workspace: "API"}},
		Clock:  map[string]time.Time{},
		Labels: map[string]string{},
		Maki: maki.Roster{
			Pids:    []int{700},
			Reports: map[string]maki.Report{"S-M": {Surface: "S-M", Cwd: "/Users/x/work/api", Sessions: sessions}},
		},
		Threshold: 45 * time.Minute,
	}
	for _, sess := range sessions {
		if t := sess.LastActivity(); !t.IsZero() {
			s.Clock[sess.ID] = t
		}
	}
	return s
}

func session(id, title, status string, idle time.Duration) maki.Session {
	return maki.Session{ID: id, Title: title, Status: status,
		UpdatedAt: float64(now.Add(-idle).Unix())}
}

func TestMakiStatusesBecomeTheSameThreeBands(t *testing.T) {
	f := Build(makiSnap(
		session("m1", "wire the webhook", maki.NeedsInput, time.Hour),
		session("m2", "port the parser", maki.Working, 0),
		session("m3", "read the changelog", maki.Idle, 2*time.Hour),
	), now)
	if len(f.Rows) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(f.Rows), f.Rows)
	}
	want := map[string]int{"m1": RankBlocked, "m2": RankWorking, "m3": RankQuiet}
	for _, r := range f.Rows {
		if got := want[r.Key]; r.Rank != got {
			t.Errorf("%s rank = %d, want %d", r.Key, r.Rank, got)
		}
	}
	if f.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1 — needs_input is a human's turn", f.Blocked)
	}
	// The state text is the fleet's, not the agent's: two agents reporting the same
	// state must read the same, or the board has two vocabularies.
	for _, r := range f.Rows {
		if r.Key == "m1" && r.State != "blocked →" {
			t.Errorf("blocked state = %q, want the same words a claude row gets", r.State)
		}
	}
}

// The tab is where the row is jumpable to, and the workspace and idle clock come from
// the same join claude's rows use.
func TestMakiRowsCarryTheirTab(t *testing.T) {
	r := Build(makiSnap(session("m1", "wire the webhook", maki.Idle, 30*time.Minute)), now).Rows[0]
	if r.Surface != "S-M" || !r.Jumpable() {
		t.Errorf("surface = %q, jumpable = %v; a maki tab is a tab", r.Surface, r.Jumpable())
	}
	if r.Workspace != "API" {
		t.Errorf("workspace = %q, want API", r.Workspace)
	}
	if r.Idle != 30*time.Minute {
		t.Errorf("idle = %v, want 30m from the session's own updated_at", r.Idle)
	}
}

// The label chain is the one every row shares, with maki's session title standing where
// Claude Code's job label stands: what the user called it, then what the agent calls it,
// then the tab title, then the directory.
func TestMakiLabelPrefersTheMostSpecificName(t *testing.T) {
	titled := makiSnap(session("m1", "wire the webhook", maki.Idle, 0))
	if got := Build(titled, now).Rows[0].Label; got != "wire the webhook" {
		t.Errorf("label = %q, want maki's own session title", got)
	}

	labelled := makiSnap(session("m1", "wire the webhook", maki.Idle, 0))
	labelled.Labels = map[string]string{"S-M": "mine"}
	if got := Build(labelled, now).Rows[0].Label; got != "mine" {
		t.Errorf("label = %q, want the user's label to win", got)
	}

	untitled := makiSnap(session("m1", "", maki.Idle, 0))
	if got := Build(untitled, now).Rows[0].Label; got != "maki tab" {
		t.Errorf("label = %q, want the cmux tab title", got)
	}

	bare := makiSnap(session("m1", "", maki.Idle, 0))
	bare.Titles = map[int]cmux.Titles{700: {ID: "S-M"}}
	if got := Build(bare, now).Rows[0].Label; got != "api" {
		t.Errorf("label = %q, want the working directory's base", got)
	}
}

// A report outlives the process that wrote it — maki fires no shutdown event — so the
// running processes are what say which reports still describe something alive. Without
// this, a quit maki leaves a row on the board for good.
func TestAStaleReportWithNoProcessIsNotARow(t *testing.T) {
	s := makiSnap(session("m1", "wire the webhook", maki.Working, 0))
	s.Maki.Pids = nil
	if f := Build(s, now); len(f.Rows) != 0 {
		t.Errorf("got %d rows from a report whose maki has exited: %+v", len(f.Rows), f.Rows)
	}
}

// The mirror image: a maki running in no cmux tab is a session board has nowhere to send
// you, which is the rule interactive claude sessions with no surface are held to.
func TestAMakiOutsideCmuxIsNotARow(t *testing.T) {
	s := makiSnap(session("m1", "wire the webhook", maki.Working, 0))
	s.Titles = map[int]cmux.Titles{}
	f := Build(s, now)
	if len(f.Rows) != 0 {
		t.Errorf("got %d rows for a maki with no tab: %+v", len(f.Rows), f.Rows)
	}
	if f.Trouble != "" {
		t.Errorf("Trouble = %q; a maki outside cmux is not a fault", f.Trouble)
	}
}

// A maki that is running and saying nothing is the shape of a missing hook, and it is the
// one maki failure a reader can fix. Silence here is what §9.26 is about: without it the
// tab simply never appears and nothing says why.
func TestAMakiThatIsRunningAndNotReportingIsTrouble(t *testing.T) {
	s := makiSnap()
	s.Maki.Reports = nil
	f := Build(s, now)
	if !strings.Contains(f.Trouble, "maki") {
		t.Errorf("Trouble = %q, want it to name maki", f.Trouble)
	}
	if !strings.Contains(f.Trouble, "doctor") {
		t.Errorf("Trouble = %q, want it to name the command that explains it", f.Trouble)
	}
}

// The other side of that line. cmux counts a surface's whole process tree, so a `maki
// --print` a script starts inside a tab is a maki with no report of its own — and on a
// machine that is otherwise reporting fine, saying so every ten seconds is how a signal
// stops being read.
func TestAnExtraMakiOnAReportingMachineIsNotTrouble(t *testing.T) {
	s := makiSnap(session("m1", "wire the webhook", maki.Working, 0))
	s.Titles[701] = cmux.Titles{ID: "S-OTHER", Surface: "some script", Workspace: "OPS"}
	s.Maki.Pids = []int{700, 701}
	f := Build(s, now)
	if f.Trouble != "" {
		t.Errorf("Trouble = %q; the machine is reporting and one stray maki is not a fault", f.Trouble)
	}
	if len(f.Rows) != 1 {
		t.Errorf("got %d rows, want only the reported session", len(f.Rows))
	}
}

// The same finding biting the other way. cmux lists a surface's whole process tree, so one
// maki in one tab routinely resolves through two pids — and a walk over pids would then
// emit that tab's sessions twice, with two rows sharing one key and every counter doubled.
func TestOneTabIsOneSetOfRowsHoweverManyPidsResolveToIt(t *testing.T) {
	s := makiSnap(
		session("m1", "wire the webhook", maki.Working, 0),
		session("m2", "port the parser", maki.NeedsInput, time.Hour),
	)
	// A parent and its child, both on the tab, which is what `pgrep -x maki` returns for
	// one maki started from a shell inside it.
	s.Titles[701] = s.Titles[700]
	s.Maki.Pids = []int{700, 701}
	f := Build(s, now)
	if len(f.Rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(f.Rows), f.Rows)
	}
	if f.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1 — the counters were doubled too", f.Blocked)
	}
	keys := map[string]int{}
	for _, r := range f.Rows {
		keys[r.Key]++
		if keys[r.Key] > 1 {
			t.Errorf("row key %q appears twice; selection would be ambiguous", r.Key)
		}
	}
}

// An unreadable report is a different repair from a missing one, so it gets its own words.
func TestUnreadableMakiReportsAreTheirOwnTrouble(t *testing.T) {
	s := Snapshot{MakiErr: maki.ErrUnreadable}
	if got := Build(s, now).Trouble; !strings.Contains(got, "unreadable") {
		t.Errorf("Trouble = %q, want it to say unreadable", got)
	}
}

// The claude roster outranks maki: it is the bulk of most fleets, and one line has room
// for the more fundamental fact. `doctor` lists both.
func TestTheClaudeRosterOutranksMaki(t *testing.T) {
	s := makiSnap()
	s.Maki.Reports = nil
	s.RosterErr = maki.ErrUnreadable // any roster failure will do
	if got := Build(s, now).Trouble; strings.Contains(got, "maki") {
		t.Errorf("Trouble = %q, want the claude roster failure to win", got)
	}
}

// Two agents, one fleet. The counts a header states are about sessions, not about who
// is running them.
func TestBothAgentsLandInOneFleet(t *testing.T) {
	s := makiSnap(session("m1", "wire the webhook", maki.NeedsInput, time.Hour))
	c := interactive()
	s.Agents = append(s.Agents, c)
	s.Titles[c.Pid] = cmux.Titles{ID: "S-C", Surface: "claude tab", Workspace: "APP"}
	s.Clock[c.SessionID] = now.Add(-2 * time.Hour)
	f := Build(s, now)
	if f.Sessions() != 2 {
		t.Fatalf("Sessions() = %d, want both agents' sessions", f.Sessions())
	}
	if f.Workspaces != 2 {
		t.Errorf("Workspaces = %d, want 2", f.Workspaces)
	}
	// Band first, then oldest within the band — the claude session is the older one and
	// the maki session is the blocked one, so they must not be ordered by agent.
	if f.Rows[0].Key != "m1" {
		t.Errorf("first row = %q, want the blocked maki session", f.Rows[0].Key)
	}
}

// Ordering must not come out of a map. One tab with several sessions, built twice, has to
// produce the same rows in the same order or the cursor moves on its own.
func TestMakiRowOrderIsStable(t *testing.T) {
	s := makiSnap(
		session("m1", "a", maki.Idle, time.Hour),
		session("m2", "b", maki.Idle, time.Hour),
		session("m3", "c", maki.Idle, time.Hour),
	)
	first := keysOf(Build(s, now).Rows)
	for range 20 {
		if got := keysOf(Build(s, now).Rows); got != first {
			t.Fatalf("row order changed between builds: %q then %q", first, got)
		}
	}
}

func keysOf(rows []Row) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.Key + " ")
	}
	return b.String()
}
