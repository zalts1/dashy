package host

import (
	"os"
	"path/filepath"
	"strings"
)

// Branch is the branch a worktree has checked out, or "" when board cannot tell.
//
// It exists to settle which pull request belongs to a row. cmux reports one pull request per
// *workspace*, keyed off that workspace's own directory — but a session in a linked worktree is
// on a different branch to the workspace it was launched from, so the workspace's answer is
// another branch's pull request. Comparing branches is what stops board drawing it (§18).
//
// A linked worktree's `.git` is a file pointing at the repository's gitdir, and HEAD lives
// *there*, not in the worktree — the same indirection Repository reads for its own purpose, and
// the reason this cannot simply read `<tree>/.git/HEAD`.
//
// No subprocess, for the reason WorkTree gives: `git branch --show-current` answers the same
// question and costs a fork per row per tick. This is one file read, memoised per worktree.
//
// A detached HEAD answers "" rather than its sha. A sha is not a branch, and the one caller
// compares branches — an answer that can never match is better spelled as no answer.
func Branch(tree string) string {
	if tree == "" {
		return ""
	}
	head, err := os.ReadFile(filepath.Join(gitDir(tree), "HEAD"))
	if err != nil {
		return ""
	}
	ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: refs/heads/")
	if !ok {
		return "" // detached, or a shape this does not know
	}
	return ref
}

// gitDir is where a worktree keeps HEAD: its own `.git` directory for a main checkout, and the
// repository's `worktrees/<name>` directory for a linked one.
func gitDir(tree string) string {
	dot := filepath.Join(tree, ".git")
	b, err := os.ReadFile(dot)
	if err != nil {
		return dot // a directory, not a file: a main checkout, and HEAD is inside it
	}
	if rest, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:"); ok {
		return strings.TrimSpace(rest)
	}
	return dot
}
