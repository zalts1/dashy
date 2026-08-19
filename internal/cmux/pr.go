package cmux

import (
	"strconv"
	"strings"
	"sync"
)

// The pull request cmux has already correlated for a tab.
//
// cmux polls GitHub itself and shows the result in its sidebar, one badge per tab. `sidebar-state`
// hands the same thing over per workspace, so board reads it instead of asking GitHub a second
// time — the badge is enrichment from cmux exactly as tab titles and the idle clock are (§3).
//
// This was very nearly built as a networked read against `gh`. It is not, and the reason is worth
// knowing: the flag is `--workspace`, not `--tab`. `--tab` is not a flag `sidebar-state` accepts,
// so passing it is silently ignored and the app answers about the *selected* tab — which made
// seven different tab UUIDs return one tab's data and looked exactly like an addressing dead end
// (EVIDENCE.md §9.42).
const prPrefix = "pr="

// PR is a tab's pull request: where it is, and what state cmux found it in.
type PR struct {
	Number int
	State  string // open | merged | closed, cmux's own vocabulary
	URL    string
}

// Open reports whether this is a pull request somebody still has to do something about. The
// frame draws it differently on that basis (§18).
func (p PR) Open() bool { return p.State == "open" }

// PullRequests asks cmux about each workspace and returns the ones that have a pull request.
//
// One call per workspace, run concurrently: each is ~140ms and a fleet has a handful of
// workspaces, so overlapped they cost about one call and hide behind `claude agents` (§9.3).
func PullRequests(workspaces []string) map[string]PR {
	out := map[string]PR{}
	if len(workspaces) == 0 {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ws := range workspaces {
		if ws == "" {
			continue
		}
		wg.Add(1)
		go func(ws string) {
			defer wg.Done()
			// The env is stripped by host.Output as always (§9.8), and it does not matter here:
			// --workspace names the target explicitly, so there is nothing to inherit wrongly.
			b, err := output("sidebar-state", "--workspace", ws)
			if err != nil {
				return
			}
			if pr, ok := parseSidebarPR(string(b)); ok {
				mu.Lock()
				out[ws] = pr
				mu.Unlock()
			}
		}(ws)
	}
	wg.Wait()
	return out
}

// parseSidebarPR reads the `pr=` line out of a sidebar-state dump. The shape is
// `pr=#21 open https://github.com/owner/name/pull/21`, and `pr=none` for a tab without one.
//
// Hand-parsed off one line rather than asked for as JSON, because sidebar-state has no JSON form.
// The same trade §2 makes for every upstream board reads: the surface is undocumented, so the
// parse is small and its failure is visible immediately — a missing glyph.
func parseSidebarPR(dump string) (PR, bool) {
	for _, line := range strings.Split(dump, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), prPrefix)
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		// Three fields exactly: the number, the state, the url. Fewer is `pr=none` or a shape
		// this does not know, and a half-read pull request is not one board will draw.
		if len(fields) < 3 || !strings.HasPrefix(fields[0], "#") {
			return PR{}, false
		}
		n, err := strconv.Atoi(strings.TrimPrefix(fields[0], "#"))
		if err != nil || !strings.HasPrefix(fields[2], "http") {
			return PR{}, false
		}
		return PR{Number: n, State: strings.ToLower(fields[1]), URL: fields[2]}, true
	}
	return PR{}, false
}
