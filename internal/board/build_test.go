package board

import (
	"testing"
	"time"

	"board/internal/claude"
	"board/internal/cmux"
)

var now = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

// snap builds a Snapshot around one agent, with a tab and a clock entry unless the
// test removes them.
func snap(a claude.Agent, mutate ...func(*Snapshot)) Snapshot {
	s := Snapshot{
		Agents:    []claude.Agent{a},
		Titles:    map[int]cmux.Titles{a.Pid: {ID: "S-1", Surface: "tab title", Workspace: "APP"}},
		Clock:     map[string]time.Time{a.SessionID: now.Add(-30 * time.Minute)},
		JobLabels: map[string]string{},
		Labels:    map[string]string{},
		Threshold: 45 * time.Minute,
	}
	for _, m := range mutate {
		m(&s)
	}
	return s
}

func interactive() claude.Agent {
	return claude.Agent{SessionID: "sess-1", Pid: 100, Kind: "interactive",
		Cwd: "/Users/x/work/repo-a", Status: "idle"}
}

func background() claude.Agent {
	return claude.Agent{SessionID: "sess-bg", ID: "bg1", Pid: 200, Kind: claude.Background,
		Cwd: "/Users/x/work/repo-b", State: "blocked"}
}

func one(t *testing.T, s Snapshot) Row {
	t.Helper()
	f := Build(s, now)
	if len(f.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(f.Rows))
	}
	return f.Rows[0]
}

// This board is a view of tabs: a row you cannot jump to is a row you cannot act
// on. Background agents are the deliberate exception — hiding blocked work to keep
// the table tidy would invert the point of the tool.
func TestRosterMembership(t *testing.T) {
	noTab := func(s *Snapshot) { s.Titles = map[int]cmux.Titles{} }

	if f := Build(snap(interactive(), noTab), now); len(f.Rows) != 0 {
		t.Errorf("interactive session with no cmux surface became a row: %+v", f.Rows)
	}
	r := one(t, snap(background(), noTab))
	if r.Workspace != "background" {
		t.Errorf("background agent workspace = %q, want background", r.Workspace)
	}
	if r.Jumpable() {
		t.Error("background agent reported as jumpable; it has no tab")
	}
	// Jumpable and selectable are different things: the key is the session, which every
	// row has, so a tab-less row can still be selected and lifted out of the collapsed
	// quiet tail (EVIDENCE.md §9.14).
	if r.Key != "sess-bg" {
		t.Errorf("background agent key = %q, want the session id", r.Key)
	}
	if got := one(t, snap(interactive())).Key; got != "sess-1" {
		t.Errorf("key = %q, want the session id", got)
	}
}

// Enter needs the surface behind a selection, and the selection carries a session key.
func TestByKey(t *testing.T) {
	f := Fleet{Rows: []Row{
		{Key: "K-1", Label: "one", Surface: "S-1"},
		{Key: "K-2", Label: "two"},
	}}
	if r, ok := f.ByKey("K-2"); !ok || r.Label != "two" || r.Jumpable() {
		t.Errorf("ByKey(K-2) = %+v ok=%v", r, ok)
	}
	if _, ok := f.ByKey("K-GONE"); ok {
		t.Error("ByKey found a session that is not in the fleet")
	}
	// A cleared selection must never resolve to the first row and jump somewhere.
	if _, ok := f.ByKey(""); ok {
		t.Error("an empty key resolved to a row")
	}
}

func TestStateAndRank(t *testing.T) {
	cases := []struct {
		name  string
		agent claude.Agent
		state string
		rank  int
	}{
		{"interactive waiting on a human", claude.Agent{SessionID: "s", Pid: 100, Status: "waiting"}, "blocked →", RankBlocked},
		{"background blocked", claude.Agent{SessionID: "s", Pid: 100, Kind: claude.Background, State: "blocked"}, "blocked →", RankBlocked},
		{"busy", claude.Agent{SessionID: "s", Pid: 100, Status: "busy"}, "running", RankWorking},
		{"idle at the prompt", claude.Agent{SessionID: "s", Pid: 100, Status: "idle"}, "done", RankQuiet},
		// needsInput-style states must never read as blocked; that mistake covered
		// 16 of 21 sessions once. See EVIDENCE.md §9.1.
		{"unknown status", claude.Agent{SessionID: "s", Pid: 100, Status: "needsInput"}, "done", RankQuiet},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := one(t, snap(c.agent))
			if r.State != c.state || r.Rank != c.rank {
				t.Errorf("state/rank = %q/%d, want %q/%d", r.State, r.Rank, c.state, c.rank)
			}
		})
	}
	if f := Build(snap(claude.Agent{SessionID: "s", Pid: 100, Status: "waiting"}), now); f.Blocked != 1 {
		t.Errorf("blocked count = %d, want 1", f.Blocked)
	}
}

func TestLabelPrecedence(t *testing.T) {
	withJob := func(s *Snapshot) { s.JobLabels["sess-1"] = "watch CI to green?" }
	withUser := func(s *Snapshot) { s.Labels["S-1"] = "merge app#1497" }
	noTitle := func(s *Snapshot) {
		s.Titles[100] = cmux.Titles{ID: "S-1", Workspace: "APP"}
	}

	cases := []struct {
		name string
		want string
		with []func(*Snapshot)
	}{
		{"user label wins", "merge app#1497", []func(*Snapshot){withUser, withJob}},
		{"then Claude's own needs string", "watch CI to green?", []func(*Snapshot){withJob}},
		{"then the tab title", "tab title", nil},
		{"then the directory", "repo-a", []func(*Snapshot){noTitle}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := one(t, snap(interactive(), c.with...)).Label; got != c.want {
				t.Errorf("label = %q, want %q", got, c.want)
			}
		})
	}
}

func TestIdleAndStale(t *testing.T) {
	clock := func(d time.Duration) func(*Snapshot) {
		return func(s *Snapshot) { s.Clock["sess-1"] = now.Add(d) }
	}

	t.Run("no clock reads as fresh, not as ancient", func(t *testing.T) {
		r := one(t, snap(interactive(), func(s *Snapshot) { s.Clock = map[string]time.Time{} }))
		if r.Idle != 0 || r.Stale {
			t.Errorf("idle = %v stale = %v, want 0/false", r.Idle, r.Stale)
		}
	})

	t.Run("a clock in the future clamps to zero", func(t *testing.T) {
		if r := one(t, snap(interactive(), clock(time.Hour))); r.Idle != 0 {
			t.Errorf("idle = %v, want 0", r.Idle)
		}
	})

	t.Run("past the threshold is stale", func(t *testing.T) {
		f := Build(snap(interactive(), clock(-46*time.Minute)), now)
		if !f.Rows[0].Stale || f.Stale != 1 {
			t.Errorf("stale = %v count = %d, want true/1", f.Rows[0].Stale, f.Stale)
		}
	})

	t.Run("a working session is never stale", func(t *testing.T) {
		busy := interactive()
		busy.Status = "busy"
		f := Build(snap(busy, clock(-50*time.Hour)), now)
		if f.Rows[0].Stale || f.Stale != 0 {
			t.Error("elapsed time on a working agent is progress, not rot")
		}
	})

	t.Run("oldest spans every band", func(t *testing.T) {
		f := Build(snap(interactive(), clock(-50*time.Hour)), now)
		if f.Oldest != 50*time.Hour {
			t.Errorf("oldest = %v, want 50h", f.Oldest)
		}
	})
}

// Band first, then the thing ignored longest at the top of its band.
func TestSortIsBandThenOldest(t *testing.T) {
	agent := func(id string, pid int, status string) claude.Agent {
		return claude.Agent{SessionID: id, Pid: pid, Status: status}
	}
	s := Snapshot{
		Agents: []claude.Agent{
			agent("fresh-quiet", 1, "idle"),
			agent("busy", 2, "busy"),
			agent("old-quiet", 3, "idle"),
			agent("blocked", 4, "waiting"),
		},
		Titles: map[int]cmux.Titles{
			1: {ID: "S1", Surface: "fresh-quiet"}, 2: {ID: "S2", Surface: "busy"},
			3: {ID: "S3", Surface: "old-quiet"}, 4: {ID: "S4", Surface: "blocked"},
		},
		Clock: map[string]time.Time{
			"fresh-quiet": now.Add(-time.Minute), "busy": now,
			"old-quiet": now.Add(-40 * time.Hour), "blocked": now.Add(-time.Hour),
		},
		Threshold: 45 * time.Minute,
	}
	var got []string
	for _, r := range Build(s, now).Rows {
		got = append(got, r.Label)
	}
	want := []string{"blocked", "old-quiet", "fresh-quiet", "busy"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestBandsPreserveSort(t *testing.T) {
	f := Fleet{Rows: []Row{
		{Label: "b", Rank: RankBlocked}, {Label: "q1", Rank: RankQuiet},
		{Label: "q2", Rank: RankQuiet}, {Label: "w", Rank: RankWorking},
	}}
	blocked, working, quiet := f.Bands()
	if len(blocked) != 1 || len(working) != 1 || len(quiet) != 2 {
		t.Fatalf("bands = %d/%d/%d, want 1/1/2", len(blocked), len(working), len(quiet))
	}
	if quiet[0].Label != "q1" {
		t.Errorf("quiet band reordered: %v", quiet)
	}
}

func TestFind(t *testing.T) {
	rows := []Row{
		{Label: "merge app#1497", Workspace: "APP"},
		{Label: "rotting thing", Workspace: "REVIEWS"},
		{Label: "another app row", Workspace: "TASKS"},
	}
	cases := []struct {
		q    string
		hits int
	}{
		{"merge", 1},
		{"APP", 2},    // matches one workspace and one label
		{"app", 2},    // case-insensitive
		{"REVIEW", 1}, // matches the workspace
		{"nothing", 0},
	}
	for _, c := range cases {
		if got := len(Find(rows, c.q)); got != c.hits {
			t.Errorf("Find(%q) = %d hits, want %d", c.q, got, c.hits)
		}
	}
}

// The header states the fleet's span, so the count must be of real tabs: background
// agents have no workspace and would otherwise inflate it by one shared bucket.
func TestWorkspaceSpan(t *testing.T) {
	agent := func(id string, pid int) claude.Agent {
		return claude.Agent{SessionID: id, Pid: pid, Status: "idle"}
	}
	s := Snapshot{
		Agents: []claude.Agent{agent("a", 1), agent("b", 2), agent("c", 3), background()},
		Titles: map[int]cmux.Titles{
			1: {ID: "S1", Surface: "a", Workspace: "APP"},
			2: {ID: "S2", Surface: "b", Workspace: "APP"},
			3: {ID: "S3", Surface: "c", Workspace: "REVIEWS"},
		},
		Threshold: 45 * time.Minute,
	}
	if got := Build(s, now).Workspaces; got != 2 {
		t.Errorf("workspaces = %d, want 2 (APP, REVIEWS; background is not a workspace)", got)
	}
	if got := Build(Snapshot{}, now).Workspaces; got != 0 {
		t.Errorf("empty fleet workspaces = %d, want 0", got)
	}
}
