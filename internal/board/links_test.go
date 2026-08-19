package board

import (
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/maki"
	"github.com/zalts1/dashy/internal/preview"
)

// The three directories the join has to tell apart: a repository, a linked worktree
// inside it, and a second worktree. Claude Code creates worktrees under the main
// checkout, so "under the same path" is not the same question as "the same branch".
const (
	repo  = "/Users/x/work/repo"
	treeA = repo + "/.claude/worktrees/feature-a"
	treeB = repo + "/.claude/worktrees/feature-b"
)

// trees is what Collect resolves impurely: every directory either side of the join,
// mapped to the worktree it belongs to. Build is pure over it (§11).
func trees() map[string]string {
	return map[string]string{
		repo:           repo,
		repo + "/app":  repo,
		treeA:          treeA,
		treeA + "/app": treeA,
		treeB:          treeB,
	}
}

// A row's folder is the worktree its session is working in, not the directory the
// session happens to be sitting in: that is what shows the branch's changed files.
func TestFolderIsTheWorktree(t *testing.T) {
	a := interactive()
	a.Cwd = repo + "/app"
	r := one(t, snap(a, func(s *Snapshot) { s.Trees = trees() }))
	if r.Folder != repo {
		t.Errorf("folder = %q, want the worktree %q", r.Folder, repo)
	}

	a.Cwd = treeA + "/app"
	r = one(t, snap(a, func(s *Snapshot) { s.Trees = trees() }))
	if r.Folder != treeA {
		t.Errorf("worktree session folder = %q, want %q", r.Folder, treeA)
	}

	// A cwd board could not resolve — deleted since the session started — gets no folder
	// rather than a guess. Offering to open a directory that is gone is worse than
	// offering nothing.
	a.Cwd = "/Users/x/work/gone"
	if r := one(t, snap(a, func(s *Snapshot) { s.Trees = trees() })); r.Folder != "" {
		t.Errorf("unresolvable cwd got folder %q, want none", r.Folder)
	}
}

// The preview belongs to the session whose worktree it serves. A dev server started in
// one worktree must never surface on a row working in another — including the main
// checkout the worktree sits inside, which is what a plain path-prefix rule would get
// wrong.
func TestPreviewJoinsOnTheWorktree(t *testing.T) {
	routes := []preview.Route{
		{URL: "https://feature-a.localhost", Dir: treeA},
		{URL: "https://main.localhost", Dir: repo + "/app"},
	}
	withPreviews := func(s *Snapshot) { s.Trees, s.Previews = trees(), routes }

	a := interactive()
	a.Cwd = treeA
	if got := one(t, snap(a, withPreviews)).Preview; got != "https://feature-a.localhost" {
		t.Errorf("worktree row preview = %q, want feature-a", got)
	}

	a.Cwd = repo
	if got := one(t, snap(a, withPreviews)).Preview; got != "https://main.localhost" {
		t.Errorf("main checkout preview = %q, want main", got)
	}

	// A worktree with nothing running gets no link, even though a preview is up in the
	// repository it was made from.
	a.Cwd = treeB
	if got := one(t, snap(a, withPreviews)).Preview; got != "" {
		t.Errorf("worktree with no dev server got preview %q, want none", got)
	}
}

// Two previews inside one worktree — a monorepo running two dev servers — resolve to the
// one nearest the session's own directory, and resolve the same way on every tick.
func TestPreviewPicksTheNearest(t *testing.T) {
	tr := trees()
	tr[repo+"/api"] = repo
	routes := []preview.Route{
		{URL: "https://api.localhost", Dir: repo + "/api"},
		{URL: "https://web.localhost", Dir: repo + "/app"},
	}
	a := interactive()
	a.Cwd = repo + "/app"
	r := one(t, snap(a, func(s *Snapshot) { s.Trees, s.Previews = tr, routes }))
	if r.Preview != "https://web.localhost" {
		t.Errorf("preview = %q, want the one in the session's own directory", r.Preview)
	}
}

// Both links reach a maki row too. maki sessions are rows like any other (§17), and the
// report carries the cwd the join needs.
func TestMakiRowsGetLinks(t *testing.T) {
	s := Snapshot{
		Titles:   map[int]cmux.Titles{700: {ID: "S-M", Surface: "maki", Workspace: "APP"}},
		Clock:    map[string]time.Time{},
		Trees:    trees(),
		Previews: []preview.Route{{URL: "https://feature-a.localhost", Dir: treeA}},
		Maki: maki.Roster{
			Pids:    []int{700},
			Reports: map[string]maki.Report{"S-M": {Cwd: treeA, Sessions: []maki.Session{{ID: "m-1", Title: "a maki session"}}}},
		},
		Threshold: 45 * time.Minute,
	}
	r := one(t, s)
	if r.Folder != treeA {
		t.Errorf("maki folder = %q, want %q", r.Folder, treeA)
	}
	if r.Preview != "https://feature-a.localhost" {
		t.Errorf("maki preview = %q, want feature-a", r.Preview)
	}
}

// A todo has no process, so it has no directory and nothing to point at. The same rule
// that keeps it out of the counters keeps it out of the links (§12).
func TestTodosHaveNoLinks(t *testing.T) {
	a := interactive()
	a.Cwd = repo
	s := snap(a, func(s *Snapshot) {
		s.Trees = trees()
		s.Previews = []preview.Route{{URL: "https://main.localhost", Dir: repo}}
		s.Todos = []config.Todo{{ID: "t1", Text: "reply to the questionnaire", Created: now}}
	})
	f := Build(s, now)
	for _, r := range f.Rows {
		if r.Rank != RankTodo {
			continue
		}
		if r.Folder != "" || r.Preview != "" {
			t.Errorf("todo row carries links: folder %q preview %q", r.Folder, r.Preview)
		}
	}
}

// A Storybook joins on the worktree exactly as a preview does, and the two are independent:
// a row can carry both, either, or neither. They are found by different mechanisms — a
// portless route and a bounded port scan — and the join is the one thing they share (§18).
func TestStorybookJoinsLikeAPreview(t *testing.T) {
	withBoth := func(s *Snapshot) {
		s.Trees = trees()
		s.Previews = []preview.Route{{URL: "https://feature-a.localhost", Dir: treeA}}
		s.Storybooks = []preview.Route{
			{URL: "http://localhost:6006", Dir: treeA},
			{URL: "http://localhost:6007", Dir: repo},
		}
	}

	a := interactive()
	a.Cwd = treeA
	r := one(t, snap(a, withBoth))
	if r.Preview != "https://feature-a.localhost" || r.Storybook != "http://localhost:6006" {
		t.Errorf("worktree row: preview %q storybook %q", r.Preview, r.Storybook)
	}

	// The main checkout has a Storybook and no dev server: one glyph, not both, and not the
	// worktree's Storybook either.
	a.Cwd = repo
	r = one(t, snap(a, withBoth))
	if r.Preview != "" {
		t.Errorf("main checkout got a worktree's preview: %q", r.Preview)
	}
	if r.Storybook != "http://localhost:6007" {
		t.Errorf("main checkout storybook = %q, want the one in its own worktree", r.Storybook)
	}

	// A third worktree with nothing running in it gets neither, though both are up elsewhere
	// in the same repository.
	a.Cwd = treeB
	r = one(t, snap(a, withBoth))
	if r.Preview != "" || r.Storybook != "" {
		t.Errorf("idle worktree got links: preview %q storybook %q", r.Preview, r.Storybook)
	}
}

// A todo has no directory, so it has none of the three links (§12).
func TestTodosHaveNoStorybook(t *testing.T) {
	a := interactive()
	a.Cwd = repo
	s := snap(a, func(s *Snapshot) {
		s.Trees = trees()
		s.Storybooks = []preview.Route{{URL: "http://localhost:6006", Dir: repo}}
		s.Todos = []config.Todo{{ID: "t1", Text: "book the quarterly review", Created: now}}
	})
	for _, r := range Build(s, now).Rows {
		if r.Rank == RankTodo && r.Storybook != "" {
			t.Errorf("todo row carries a storybook: %q", r.Storybook)
		}
	}
}
