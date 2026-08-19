package board

import (
	"path/filepath"
	"strings"

	"github.com/zalts1/dashy/internal/preview"
)

// The link join: which local preview belongs to which row. One concern, one file — the rule
// below is the one that would have been wrong silently, so it is worth being able to read on
// its own (DESIGN.md §18, EVIDENCE.md §9.34).

// nearest picks the one URL a row points at from a set of candidates: something serving the
// same git worktree the session is working in. Used for both the preview and the Storybook,
// which are found by different mechanisms and joined by the same rule.
//
// The worktree is the join and a path prefix is not, because Claude Code puts its
// worktrees *inside* the main checkout. `.claude/worktrees/feature` is under the repo's
// own directory while being a different branch, so "the route's directory is below the
// session's" would put a feature branch's preview on the main checkout's row — and a
// preview link on the wrong row is worse than no link at all (§18).
//
// Within one worktree there can be several: a monorepo running a dev server per app. The
// nearest to the session's own directory wins, measured in shared leading path
// components, and ties resolve to the first route — which arrives URL-sorted, so the
// answer is the same on every tick rather than drifting with map order.
func nearest(candidates []preview.Route, trees map[string]string, tree, cwd string) string {
	if tree == "" {
		return ""
	}
	best, bestShared := "", -1
	for _, rt := range candidates {
		if trees[rt.Dir] != tree {
			continue
		}
		if n := sharedPath(rt.Dir, cwd); n > bestShared {
			best, bestShared = rt.URL, n
		}
	}
	return best
}

// sharedPath counts the leading path components two directories have in common. Only the
// tie-break depends on it, so an unresolved spelling of one of them costs a preference
// and never a wrong answer: both candidates are already known to be in one worktree.
func sharedPath(a, b string) int {
	as, bs := strings.Split(a, string(filepath.Separator)), strings.Split(b, string(filepath.Separator))
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}
