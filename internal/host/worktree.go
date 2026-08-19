package host

import (
	"os"
	"path/filepath"
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
