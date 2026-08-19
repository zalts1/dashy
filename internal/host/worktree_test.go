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

// Repository is the other half of WorkTree: a linked worktree's *repository*, so the column
// can say `app -> feature-a` rather than making the reader guess which repo a branch directory
// belongs to (§18).
func TestRepository(t *testing.T) {
	tmp := realTempDir(t)
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git", "worktrees", "feature-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(repo, ".claude", "worktrees", "feature-a")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	// A linked worktree's .git is a file naming the gitdir inside the repository it came from.
	// That pointer is the only thing on disk that connects the two.
	gitdir := "gitdir: " + filepath.Join(repo, ".git", "worktrees", "feature-a") + "\n"
	if err := os.WriteFile(filepath.Join(tree, ".git"), []byte(gitdir), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := Repository(tree); got != repo {
		t.Errorf("Repository(worktree) = %q, want the repository %q", got, repo)
	}
	// A main checkout is its own repository: .git is a directory, there is no pointer to read,
	// and answering itself is what lets the caller treat both the same way.
	if got := Repository(repo); got != repo {
		t.Errorf("Repository(main checkout) = %q, want itself", got)
	}
	// A directory in no repository at all answers itself too.
	loose := filepath.Join(tmp, "loose")
	if err := os.MkdirAll(loose, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Repository(loose); got != loose {
		t.Errorf("Repository(no repo) = %q, want itself", got)
	}
	// A .git file board cannot make sense of answers the worktree rather than guessing. A
	// wrong repository name in the column is worse than a missing one.
	odd := filepath.Join(tmp, "odd")
	if err := os.MkdirAll(odd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(odd, ".git"), []byte("something else entirely\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Repository(odd); got != odd {
		t.Errorf("Repository(unparseable .git) = %q, want itself", got)
	}
	if got := Repository(""); got != "" {
		t.Errorf("Repository(\"\") = %q, want empty", got)
	}
}
