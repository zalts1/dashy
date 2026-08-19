package cmux

import (
	"strings"
	"sync"
)

// State is what cmux's sidebar knows about one workspace: the pull request it has already
// correlated, the colour the user gave it, and the branch its directory is on.
//
// One dump, three facts. board was already making this call for the pull request alone (§18),
// so the colour and the branch cost no subprocess — which is the only reason grouping is
// affordable at all on a surface that refuses to poll anything (§2).
type State struct {
	PR     PR
	Colour string // the workspace's accent, "#RRGGBB", or "" when cmux says none
	Branch string // the branch the workspace's directory is on, without its cleanliness word
}

// WorkspaceStates asks cmux about each workspace and returns what its sidebar says.
//
// One call per workspace, run concurrently: each is ~140ms and a fleet has a handful of
// workspaces, so overlapped they cost about one call and hide behind `claude agents` (§9.3).
func WorkspaceStates(workspaces []string) map[string]State {
	out := map[string]State{}
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
			st := parseSidebarState(string(b))
			mu.Lock()
			out[ws] = st
			mu.Unlock()
		}(ws)
	}
	wg.Wait()
	return out
}

// parseSidebarState reads the three facts board wants out of one dump.
//
// Hand-parsed off flat lines rather than asked for as JSON, because sidebar-state has no JSON
// form — the same trade §2 makes for every upstream board reads (§18).
func parseSidebarState(dump string) State {
	var st State
	st.PR, _ = parseSidebarPR(dump)
	st.Colour = colour(field(dump, "color"))
	st.Branch = branch(field(dump, "git_branch"))
	return st
}

// field reads one top-level `key=value` line.
//
// **Top-level is load-bearing.** sidebar-state nests a `status_count` block whose lines are
// indented and carry a `color=` of their own — the agent's state badge, cmux blue for running
// and grey for idle. That is a fact board already derives itself, and taking it would paint
// every workspace the same colour and call it the user's choice.
func field(dump, key string) string {
	for _, line := range strings.Split(dump, "\n") {
		if line != strings.TrimLeft(line, " \t") {
			continue // indented: belongs to a nested block, not to the workspace
		}
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// colour normalises cmux's answer. "none" is most workspaces — three of the eight this was
// built against — and an absent colour is a group drawn without one, not a group drawn black.
func colour(v string) string {
	if !strings.HasPrefix(v, "#") || len(v) != 7 {
		return ""
	}
	return strings.ToUpper(v)
}

// branch drops the cleanliness word cmux appends. `git_branch=main clean` is two facts and
// board wants the first: whether a tree is dirty is not something this surface reports.
func branch(v string) string {
	if v == "" || v == "none" {
		return ""
	}
	return strings.Fields(v)[0]
}
