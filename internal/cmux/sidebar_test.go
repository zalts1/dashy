package cmux

import "testing"

// The real dump for a coloured workspace, verbatim from `cmux sidebar-state --workspace`.
// Three facts board wants live in it, and board already makes this call for the pull request.
const coloured = `tab=0EFAC982-5897-4124-A4B2-E9BA424DB8A3
color=#880E4F
cwd=/Users/you/repo/date-invite
focused_cwd=/Users/you/repo/date-invite
git_branch=main clean
pr=none
pr_label=none
ports=none
status_count=1
  claude_code=Idle icon=pause.circle.fill color=#8E8E93
`

func TestParseSidebarColour(t *testing.T) {
	if got := parseSidebarState(coloured).Colour; got != "#880E4F" {
		t.Errorf("Colour = %q, want %q", got, "#880E4F")
	}
	// Most workspaces have no colour — three of eight on the fleet this was built against.
	// "none" is cmux's way of saying so and must not become a colour board tries to draw.
	if got := parseSidebarState("color=none\n").Colour; got != "" {
		t.Errorf("colour of an uncoloured workspace = %q, want empty", got)
	}
	if got := parseSidebarState("cwd=/x\n").Colour; got != "" {
		t.Errorf("colour of a dump with no colour line = %q, want empty", got)
	}
	// The nested status line carries a colour too — the agent's state badge, not the
	// workspace's. It is indented, and taking it would paint every workspace cmux blue.
	if got := parseSidebarState("  claude_code=Idle icon=pause.circle.fill color=#8E8E93\n").Colour; got != "" {
		t.Errorf("colour = %q, want empty: the indented status colour is not the workspace's", got)
	}
}

func TestParseSidebarBranch(t *testing.T) {
	// The value carries a cleanliness word board has no use for. The branch is the first field.
	if got := parseSidebarState(coloured).Branch; got != "main" {
		t.Errorf("Branch = %q, want %q", got, "main")
	}
	if got := parseSidebarState("git_branch=pla-138-final-rollout-tweak clean\n").Branch; got != "pla-138-final-rollout-tweak" {
		t.Errorf("Branch = %q, want the branch without its state word", got)
	}
	if got := parseSidebarState("git_branch=none\n").Branch; got != "" {
		t.Errorf("Branch of a workspace with no git = %q, want empty", got)
	}
}

// The pull request still parses out of the same dump, unchanged.
func TestSidebarStateCarriesThePullRequest(t *testing.T) {
	const withPR = `color=#7D6608
git_branch=a-branch clean
pr=#21 open https://github.com/you/repo/pull/21
`
	st := parseSidebarState(withPR)
	if st.PR.Number != 21 || st.PR.State != "open" {
		t.Errorf("PR = %+v, want #21 open", st.PR)
	}
	if st.Colour != "#7D6608" || st.Branch != "a-branch" {
		t.Errorf("state = %+v, want the colour and branch beside the pull request", st)
	}
	if parseSidebarState(coloured).PR.Number != 0 {
		t.Error("a dump with pr=none produced a pull request")
	}
}

// Asking about nothing costs nothing: a fleet whose rows have no tabs makes no calls.
func TestWorkspaceStatesWithNoWorkspaces(t *testing.T) {
	if got := WorkspaceStates(nil); len(got) != 0 {
		t.Errorf("WorkspaceStates(nil) = %+v, want empty", got)
	}
	if got := WorkspaceStates([]string{"", ""}); len(got) != 0 {
		t.Errorf("WorkspaceStates of empty ids = %+v, want empty", got)
	}
}
