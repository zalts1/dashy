package host

import (
	"os"
	"path/filepath"
	"testing"
)

// The join in board.Build is by worktree, so this is what decides whether a preview
// belongs to a row. The case that matters is the linked worktree: `.claude/worktrees/x`
// sits *inside* the main checkout's tree but is a different branch, and a walk that
// answered the repository would put the wrong preview on the wrong row (§18).
func TestWorkTree(t *testing.T) {
	// Resolved up front: on darwin t.TempDir() hands back a path under /var, which is
	// itself a symlink to /private/var, so an unresolved expectation would fail on the
	// very behaviour the next test asserts.
	tmp := realTempDir(t)
	// A repository, a subdirectory of it, and a linked worktree nested inside it — the
	// shape Claude Code creates under .claude/worktrees.
	repo := filepath.Join(tmp, "repo")
	sub := filepath.Join(repo, "app", "src")
	tree := filepath.Join(repo, ".claude", "worktrees", "feature")
	treeSub := filepath.Join(tree, "app")
	for _, d := range []string{sub, treeSub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A linked worktree's .git is a *file* pointing at the repository's gitdir, not a
	// directory. Checking only for a directory would walk straight past it.
	if err := os.WriteFile(filepath.Join(tree, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(tmp, "loose")
	if err := os.MkdirAll(loose, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ in, want string }{
		{repo, repo},
		{sub, repo},
		{tree, tree},    // the worktree answers itself, not repo
		{treeSub, tree}, // and so does anything under it
		{loose, loose},  // no repository above it: it answers itself
		{filepath.Join(tmp, "gone"), ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := WorkTree(c.in); got != c.want {
			t.Errorf("WorkTree(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Symlinks are resolved, because the two sides of the join arrive by different routes:
// a session's cwd is whatever the shell reported, a preview's is what lsof resolved.
// Two spellings of one directory have to compare equal or nothing ever matches.
func TestWorkTreeResolvesSymlinks(t *testing.T) {
	tmp := realTempDir(t)
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	if got := WorkTree(link); got != repo {
		t.Errorf("WorkTree(symlink) = %q, want the real %q", got, repo)
	}
}

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
