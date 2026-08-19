// Package github answers one question: is there an open pull request for this worktree's branch.
//
// It is the one place board reaches the network, and it does so only when asked: the `github`
// config key defaults to false, and with it unset nothing here runs. That key is the whole of
// the trade — board is described as having no network of its own, and this is the exception
// somebody opts into (DESIGN.md §10.12).
//
// board does not make the request itself. `gh api graphql --cache` does, which matters three
// times over: gh already holds the credentials, gh owns the cache — so board still writes only
// ~/.board.json — and a cached answer costs ~40ms rather than ~580ms, so the poll interval and
// GitHub's rate limit are decoupled.
//
// **cmux already knows this and cannot be asked.** It polls api.github.com itself and holds the
// badge in the running process; `cmux sidebar-state` answers only for the tab it is called from,
// ignores `--tab` entirely, and returns nothing at all once cmux's env is stripped, which board
// always does (§9.8). Nothing is on disk: no cmux file, no sqlite row, no user default mentions a
// pull request. So this package exists to fetch a second time what one process on the machine has
// already fetched, which is worth fixing upstream rather than here (EVIDENCE.md §9.37, §9.42).
package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zalts1/dashy/internal/host"
)

// PR is what one worktree's branch has open, or nothing.
type PR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// Target is one question: which repository, and which branch inside it.
type Target struct {
	Tree   string // the worktree this answers for, and the key callers join on
	Owner  string
	Name   string
	Branch string
}

// defaultTTL is how long an answer is good for. A pull request's existence changes on a human
// timescale, and this is what keeps a 10s poll from becoming 360 requests an hour per worktree.
const defaultTTL = 3 * time.Minute

// maxConcurrent bounds the fan-out. A cold answer is a network round trip, and a fleet spread
// over a dozen worktrees should not open a dozen sockets at once on somebody's tethered laptop.
const maxConcurrent = 4

// Available reports whether board can ask at all. `gh` on PATH is the whole test — whether it is
// authenticated, whether the network is up and whether you can see the repository are answered by
// the call itself, and all of them mean the same thing here: no glyph, no complaint.
func Available() bool {
	_, err := host.Look("gh")
	return err == nil
}

// Targets turns worktrees into questions, reading the branch and the remote off disk. trees maps
// a worktree to the repository it belongs to, which is what board already resolved for the
// location column (§18).
//
// A worktree board cannot form a question about is silently absent from the result: a detached
// HEAD has no branch, a remote that is not GitHub has no pull request, and neither is a fault.
func Targets(trees map[string]string) []Target {
	var out []Target
	for tree, repo := range trees {
		if tree == "" || repo == "" {
			continue
		}
		branch := branchOf(tree)
		if branch == "" {
			continue
		}
		owner, name := parseRemote(originURL(readFile(filepath.Join(repo, ".git", "config"))))
		if owner == "" || name == "" {
			continue
		}
		out = append(out, Target{Tree: tree, Owner: owner, Name: name, Branch: branch})
	}
	return out
}

// Read asks about every target and returns the worktrees that have an open pull request. Errors
// are absences: a row without a glyph is the same outcome whether gh is missing, the network is
// down, or the branch simply has no PR — and `doctor` is where the difference is stated.
func Read(targets []Target) map[string]PR {
	out := map[string]PR{}
	if len(targets) == 0 {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)
	for _, t := range targets {
		wg.Add(1)
		go func(t Target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if pr, ok := ask(t); ok {
				mu.Lock()
				out[t.Tree] = pr
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	return out
}

// ask runs the one query. --cache is gh's own on-disk response cache, which is why this is safe
// to call on every tick: the first call in a TTL pays the round trip and the rest are file reads.
func ask(t Target) (PR, bool) {
	out, err := host.Output("gh", "api", "graphql",
		"--cache", ttl(),
		"-f", "query="+query(),
		"-F", "owner="+t.Owner,
		"-F", "name="+t.Name,
		"-F", "branch="+t.Branch)
	if err != nil {
		// Includes every failure that matters and none that need telling apart here: no gh, not
		// logged in, no network, no such repository, rate limited. All of them are "no glyph".
		return PR{}, false
	}
	return parseAnswer(out)
}

func ttl() string { return strconv.Itoa(int(defaultTTL.Seconds())) + "s" }

// query asks for one open pull request on one branch and nothing else. Deliberately not the
// review state: §10.12 recorded that "reviewed since you pushed" is not measurable — GitHub's
// pushedDate is deprecated and usually null — so the CTA it would drive was left out rather than
// approximated.
func query() string {
	return `query($owner: String!, $name: String!, $branch: String!) {
  repository(owner: $owner, name: $name) {
    pullRequests(headRefName: $branch, states: [OPEN], first: 1) {
      nodes { number url state }
    }
  }
}`
}

func parseAnswer(b []byte) (PR, bool) {
	var reply struct {
		Data struct {
			Repository *struct {
				PullRequests struct {
					Nodes []PR `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &reply) != nil || reply.Data.Repository == nil {
		return PR{}, false
	}
	nodes := reply.Data.Repository.PullRequests.Nodes
	if len(nodes) == 0 || nodes[0].URL == "" {
		return PR{}, false
	}
	return nodes[0], true
}

// branchOf reads the branch a worktree is on. A linked worktree's HEAD lives in the gitdir its
// `.git` file points at, not beside its files, so the pointer is followed first.
func branchOf(tree string) string {
	git := filepath.Join(tree, ".git")
	if fi, err := os.Stat(git); err == nil && fi.IsDir() {
		return parseHead(readFile(filepath.Join(git, "HEAD")))
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(readFile(git)), "gitdir:")
	if !ok {
		return ""
	}
	return parseHead(readFile(filepath.Join(strings.TrimSpace(rest), "HEAD")))
}

// parseHead names the branch, or nothing. A detached HEAD holds a sha, and a sha is not something
// a pull request is open against.
func parseHead(s string) string {
	ref, ok := strings.CutPrefix(strings.TrimSpace(s), "ref: refs/heads/")
	if !ok {
		return ""
	}
	return ref
}

// originURL finds origin's url in a git config. Hand-parsed rather than shelled out to
// `git config --get`, which is a fork per worktree per tick on a path that already reads files.
func originURL(cfg string) string {
	section := ""
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if section != `[remote "origin"]` {
			continue
		}
		if v, ok := strings.CutPrefix(line, "url"); ok {
			if v, ok := strings.CutPrefix(strings.TrimSpace(v), "="); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

// parseRemote pulls owner and repository out of a remote url, in any of the spellings git writes.
// A remote that is not GitHub answers nothing: a query built from a GitLab path would fail on
// every tick and the failure would look exactly like "no pull request".
func parseRemote(url string) (owner, name string) {
	u := strings.TrimSuffix(strings.TrimSpace(url), ".git")
	var path string
	switch {
	case strings.HasPrefix(u, "git@github.com:"):
		path = strings.TrimPrefix(u, "git@github.com:")
	case strings.HasPrefix(u, "ssh://git@github.com/"):
		path = strings.TrimPrefix(u, "ssh://git@github.com/")
	case strings.Contains(u, "github.com/"):
		// Covers https:// with or without a user, which is the form gh itself writes.
		_, path, _ = strings.Cut(u, "github.com/")
	default:
		return "", ""
	}
	owner, name, ok := strings.Cut(path, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", ""
	}
	return owner, name
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
