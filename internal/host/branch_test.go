package host

import (
	"os"
	"path/filepath"
	"testing"
)

// A main checkout keeps HEAD in its own `.git` directory.
func TestBranchOfAMainCheckout(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, ".git"))
	write(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")
	if got := Branch(dir); got != "main" {
		t.Errorf("Branch = %q, want %q", got, "main")
	}
}

// A linked worktree's `.git` is a *file* pointing at the repository's gitdir, and its HEAD
// lives there — which is the whole reason this cannot just read `<tree>/.git/HEAD`. Getting
// this wrong is what puts one branch's pull request on another branch's row (§18).
func TestBranchOfALinkedWorktree(t *testing.T) {
	repo := t.TempDir()
	gitdir := filepath.Join(repo, ".git", "worktrees", "pla-1013")
	mkdir(t, gitdir)
	write(t, filepath.Join(gitdir, "HEAD"), "ref: refs/heads/pla-1013-dataview-system-refactor\n")

	tree := filepath.Join(repo, ".claude", "worktrees", "pla-1013")
	mkdir(t, tree)
	write(t, filepath.Join(tree, ".git"), "gitdir: "+gitdir+"\n")

	if got := Branch(tree); got != "pla-1013-dataview-system-refactor" {
		t.Errorf("Branch = %q, want the worktree's own branch", got)
	}
}

// A branch name may contain slashes, and only the `refs/heads/` prefix comes off.
func TestBranchKeepsSlashes(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, ".git"))
	write(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/feat/nested/name\n")
	if got := Branch(dir); got != "feat/nested/name" {
		t.Errorf("Branch = %q, want the whole name below refs/heads/", got)
	}
}

// Everything unreadable answers "", and "" is what the join reads as "do not claim a branch".
// A detached HEAD is a raw sha and is not a branch a pull request can be matched on.
func TestBranchAnswersNothingWhenItCannotTell(t *testing.T) {
	if got := Branch(""); got != "" {
		t.Errorf("Branch(\"\") = %q, want empty", got)
	}
	if got := Branch(t.TempDir()); got != "" {
		t.Errorf("Branch of a directory with no git = %q, want empty", got)
	}
	detached := t.TempDir()
	mkdir(t, filepath.Join(detached, ".git"))
	write(t, filepath.Join(detached, ".git", "HEAD"), "9fceb02aab1f4c5e8b3c1d2e7a6b5c4d3e2f1a0b\n")
	if got := Branch(detached); got != "" {
		t.Errorf("Branch of a detached HEAD = %q, want empty", got)
	}
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
