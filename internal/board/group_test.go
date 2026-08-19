package board

import (
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
)

// The mis-join this fixture is built from, observed live: a session working in a linked
// worktree, inside a workspace whose own directory is the main checkout on another branch.
// cmux reports one pull request per workspace, so without a branch check the row claims a
// pull request belonging to a branch it is not on (§18).
func worktreeSnap(sessionBranch, workspaceBranch string) Snapshot {
	a := interactive()
	a.Cwd = treeA
	return Snapshot{
		Agents: []claude.Agent{a},
		Titles: map[int]cmux.Titles{a.Pid: {
			ID: "S-1", Surface: "Align dataview styles", Workspace: "Align dataview styles", WorkspaceID: "W-1",
		}},
		Clock:     map[string]time.Time{a.SessionID: now.Add(-30 * time.Minute)},
		Threshold: 45 * time.Minute,
		Trees:     trees(),
		Repos:     map[string]string{treeA: repo, repo: repo},
		Branches:  map[string]string{treeA: sessionBranch},
		Spaces: map[string]cmux.State{"W-1": {
			PR:     cmux.PR{Number: 1709, State: "merged", URL: "https://github.com/x/app/pull/1709"},
			Branch: workspaceBranch,
		}},
	}
}

func TestPullRequestDropsWhenTheBranchIsNotTheRows(t *testing.T) {
	r := one(t, worktreeSnap("pla-1013-dataview-refactor", "pla-138-final-rollout-tweak"))
	if r.PR != "" || r.PRState != "" {
		t.Errorf("PR = %q %q, want none: the workspace's pull request is another branch's", r.PR, r.PRState)
	}
}

func TestPullRequestStaysWhenTheBranchIsTheRows(t *testing.T) {
	r := one(t, worktreeSnap("pla-138-final-rollout-tweak", "pla-138-final-rollout-tweak"))
	if r.PR != "https://github.com/x/app/pull/1709" || r.PRState != "merged" {
		t.Errorf("PR = %q %q, want the workspace's pull request", r.PR, r.PRState)
	}
}

// A wrong link is worse than no link (§18), so an unverifiable branch draws nothing.
// A detached HEAD and a workspace with no git both arrive here as "".
func TestPullRequestDropsWhenTheBranchIsUnknown(t *testing.T) {
	if r := one(t, worktreeSnap("", "pla-138")); r.PR != "" {
		t.Errorf("PR = %q, want none when the row's branch is unreadable", r.PR)
	}
	if r := one(t, worktreeSnap("pla-138", "")); r.PR != "" {
		t.Errorf("PR = %q, want none when the workspace's branch is unknown", r.PR)
	}
}

// fleetSnap is n sessions spread over the workspaces named, with the idle minutes given.
func fleetSnap(spec []struct {
	label, ws, wsID, colour string
	idleMin                 int
}) Snapshot {
	s := Snapshot{
		Titles:    map[int]cmux.Titles{},
		Clock:     map[string]time.Time{},
		Spaces:    map[string]cmux.State{},
		Threshold: 45 * time.Minute,
	}
	for i, x := range spec {
		a := claude.Agent{SessionID: x.label, Pid: 100 + i, Kind: "interactive", Cwd: repo, Status: "idle"}
		s.Agents = append(s.Agents, a)
		s.Titles[a.Pid] = cmux.Titles{ID: "S" + x.label, Surface: x.label, Workspace: x.ws, WorkspaceID: x.wsID}
		s.Clock[a.SessionID] = now.Add(-time.Duration(x.idleMin) * time.Minute)
		s.Spaces[x.wsID] = cmux.State{Colour: x.colour}
	}
	return s
}

// The whole point: sessions sharing a workspace sit together, and the group is named once.
func TestSessionsClusterByWorkspace(t *testing.T) {
	f := Build(fleetSnap([]struct {
		label, ws, wsID, colour string
		idleMin                 int
	}{
		{"refunds", "Payments rework", "W-pay", "#7D6608", 5},
		{"dataview", "Align dataview", "W-dv", "#006B6B", 10},
		{"webhooks", "Payments rework", "W-pay", "#7D6608", 20},
	}), now)

	_, _, _, quiet := f.Bands()
	groups := Groups(quiet)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}
	// Payments rework holds the newest session (5m), so its group leads.
	if groups[0].Name != "Payments rework" || len(groups[0].Rows) != 2 {
		t.Errorf("first group = %q with %d rows, want Payments rework with 2", groups[0].Name, len(groups[0].Rows))
	}
	// Newest first *within* the group, which is the sort the bands already used.
	if groups[0].Rows[0].Label != "refunds" || groups[0].Rows[1].Label != "webhooks" {
		t.Errorf("rows = %q, %q, want refunds then webhooks", groups[0].Rows[0].Label, groups[0].Rows[1].Label)
	}
	if groups[1].Name != "Align dataview" || len(groups[1].Rows) != 1 {
		t.Errorf("second group = %q with %d rows", groups[1].Name, len(groups[1].Rows))
	}
	if groups[0].Colour != "#7D6608" {
		t.Errorf("colour = %q, want the workspace's", groups[0].Colour)
	}
}

// Variant B: a group earns a header only when there is something to group. On a fleet with
// one session per workspace — the way this tool's author works — grouping costs nothing.
func TestOnlyAGroupWithMoreThanOneSessionEarnsAHeader(t *testing.T) {
	f := Build(fleetSnap([]struct {
		label, ws, wsID, colour string
		idleMin                 int
	}{
		{"solo", "Wizard copy", "W-wiz", "#880E4F", 5},
		{"a", "Payments", "W-pay", "", 10},
		{"b", "Payments", "W-pay", "", 20},
	}), now)
	_, _, _, quiet := f.Bands()
	groups := Groups(quiet)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
	if groups[0].Header() {
		t.Error("a workspace with one session drew a header; it should spend no line")
	}
	if !groups[1].Header() {
		t.Error("a workspace with two sessions drew no header")
	}
}

// The colour is the workspace's, and it reaches the row so the gutter bar can be drawn on
// every member — not only on the one under a header.
func TestRowsCarryTheirGroupColour(t *testing.T) {
	f := Build(fleetSnap([]struct {
		label, ws, wsID, colour string
		idleMin                 int
	}{{"solo", "Wizard copy", "W-wiz", "#880E4F", 5}}), now)
	if got := f.Rows[0].GroupColour; got != "#880E4F" {
		t.Errorf("GroupColour = %q, want the workspace's", got)
	}
	if got := f.Rows[0].Group; got != "Wizard copy" {
		t.Errorf("Group = %q, want the workspace title", got)
	}
}

// Background agents have no tab and so no workspace: they group with nothing and wear no bar.
func TestBackgroundAgentsAreNotGrouped(t *testing.T) {
	r := one(t, snap(background(), func(s *Snapshot) { s.Titles = map[int]cmux.Titles{} }))
	if r.Group != "" || r.GroupColour != "" {
		t.Errorf("background agent grouped as %q/%q, want neither", r.Group, r.GroupColour)
	}
}

// Todos have no process and no workspace, so they never join a group (§12).
func TestTodosAreNotGrouped(t *testing.T) {
	f := Build(fleetSnap(nil), now)
	_ = f
	groups := Groups([]Row{{Key: todoKey + "1", Rank: RankTodo, Label: "a note"}})
	if len(groups) != 1 || groups[0].Header() || groups[0].Name != "" {
		t.Errorf("todo grouped as %+v, want one unnamed headerless group", groups)
	}
}
