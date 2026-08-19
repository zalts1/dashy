package host

import (
	"os"
	"path/filepath"
	"strings"
)

// WorkTree is the directory a session's work belongs to: the nearest ancestor holding a
// `.git` entry.
//
// For a linked worktree that is the worktree itself and not the repository it was made
// from, which is the whole point of walking rather than asking git. Claude Code creates
// its worktrees *inside* the main checkout (`.claude/worktrees/<branch>`), so a rule that
// answered the repository would make every worktree compare equal to the main checkout
// and to each other — and the two things joined on this answer, a preview and an editor
// folder, are exactly the two that are per-branch (§18).
//
// A `.git` file counts as much as a `.git` directory: a linked worktree's is a file
// pointing at the repository's gitdir, so a directory-only check walks straight past it.
//
// Two deliberate answers at the edges. A directory in no repository answers itself, so
// every session that has a directory has a folder to open. A directory that does not
// exist answers "", because offering to open a folder that is gone is worse than
// offering nothing. Symlinks are resolved, because the two sides of the join arrive by
// different routes — a shell's idea of the cwd, and what lsof resolved for a pid — and
// two spellings of one directory have to compare equal or nothing ever matches.
//
// No subprocess: `git rev-parse --show-toplevel` answers the same question and costs a
// fork per session, on a path that runs every tick. This is a handful of stat calls.
func WorkTree(dir string) string {
	if dir == "" {
		return ""
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return ""
	}
	for d := dir; ; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			// The filesystem root, with no repository anywhere above.
			return dir
		}
		d = parent
	}
}

// Repository is the repository a worktree belongs to. For a main checkout that is the worktree
// itself; for a linked one it is the repository it was created from.
//
// The only thing on disk connecting the two is the pointer in the worktree's `.git` file —
// `gitdir: /path/to/repo/.git/worktrees/<name>` — so the repository is that path with the
// gitdir suffix removed. `git rev-parse --path-format=absolute --git-common-dir` answers the
// same question and costs a fork per row per tick, which is the trade WorkTree already declined.
//
// Everything unexpected answers the worktree rather than guessing: a `.git` that is a directory
// (a main checkout, nothing to resolve), a `.git` file in a shape this does not recognise, or a
// path with no marker in it at all. A wrong repository name in the column would be worse than
// the redundant one it replaces (§18).
func Repository(tree string) string {
	if tree == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(tree, ".git"))
	if err != nil {
		return tree
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return tree
	}
	// The marker, not the last two path elements: a repository whose own directory is called
	// "worktrees" would break a positional rule and not this one.
	if i := strings.Index(strings.TrimSpace(rest), string(filepath.Separator)+".git"+
		string(filepath.Separator)+"worktrees"+string(filepath.Separator)); i > 0 {
		return strings.TrimSpace(rest)[:i]
	}
	return tree
}
