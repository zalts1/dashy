package cmux

import (
	"reflect"
	"testing"
)

// The shape sidebar-state prints. cmux's own states are open, merged and closed — it maps
// GitHub's `state` and nothing else, so a draft arrives as open (§10.13).
func TestParseSidebarPR(t *testing.T) {
	const openDump = `tab=F2F02077-AFC5-4064-9D4F-2D023772F13C
cwd=/Users/you/work/repo
git_branch=a-branch clean
pr=#21 open https://github.com/you/repo/pull/21
pr_label=PR
ports=none
`
	got, ok := parseSidebarPR(openDump)
	if !ok {
		t.Fatal("parseSidebarPR found nothing in a dump with a pull request in it")
	}
	want := PR{Number: 21, State: "open", URL: "https://github.com/you/repo/pull/21"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSidebarPR = %+v, want %+v", got, want)
	}
	if !got.Open() {
		t.Error("an open pull request does not report itself open")
	}

	merged, ok := parseSidebarPR("pr=#1709 merged https://github.com/you/repo/pull/1709\n")
	if !ok || merged.State != "merged" || merged.Number != 1709 {
		t.Errorf("merged = %+v, %v", merged, ok)
	}
	if merged.Open() {
		t.Error("a merged pull request reports itself open")
	}
	if closed, ok := parseSidebarPR("pr=#7 closed https://github.com/you/repo/pull/7\n"); !ok || closed.State != "closed" {
		t.Errorf("closed = %+v, %v", closed, ok)
	}

	// Everything that is not a pull request, and none of it an error: most tabs have none.
	for _, none := range []string{
		"pr=none\npr_label=none\n",
		"cwd=/Users/you/work/repo\nports=none\n",
		"pr=#21\n",                       // no state, no url
		"pr=#21 open\n",                  // no url
		"pr=21 open https://x/pull/21\n", // no # — not the shape this knows
		"pr=#abc open https://x/pull/1\n",
		"pr=#21 open notaurl\n",
		"",
	} {
		if pr, ok := parseSidebarPR(none); ok {
			t.Errorf("parseSidebarPR(%q) claimed a pull request: %+v", none, pr)
		}
	}
}

// Asking about nothing costs nothing: a fleet whose rows have no tabs makes no calls.
func TestPullRequestsWithNoWorkspaces(t *testing.T) {
	if got := PullRequests(nil); len(got) != 0 {
		t.Errorf("PullRequests(nil) = %+v, want empty", got)
	}
	if got := PullRequests([]string{"", ""}); len(got) != 0 {
		t.Errorf("PullRequests of empty ids = %+v, want empty", got)
	}
}
