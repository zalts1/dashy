package cmux

import (
	"encoding/json"
	"strings"
)

// Titles is what cmux knows about one session's tab.
type Titles struct {
	ID        string // surface UUID, the handle Focus needs
	Surface   string // tab title
	Workspace string
	// WorkspaceID is the workspace UUID, which is what `sidebar-state --workspace` is addressed
	// by — and which cmux calls a *tab* in that command's output. Same identity, two names (§18).
	WorkspaceID string
}

// node is one surface plus the context its ancestors carry. cmux puts the pane id on
// the surface itself but the workspace id only on the workspace node above it, so the
// walk has to thread that down. Both the pid map and the tab-selection lookup are
// derived from this one pass: the tree shape has changed before, and a second
// hand-written walk is a second thing to get wrong.
type node struct {
	ID       string
	Title    string
	Pane     string
	Ws       string // workspace UUID
	WsTitle  string
	Index    int  // position in the pane's tab strip
	Selected bool // the pane's front tab
	Pids     []int
}

// TitlesByPid maps agent pid -> its surface and workspace titles. cmux surface
// nodes carry no UUID in the tree, so pid is the join key; cmux_process_pids is 1:1.
func TitlesByPid() map[int]Titles {
	return parseTop(tree())
}

// tree is the one read every cmux lookup shares. It asks about the whole world
// because host.Output blanks the cmux env: an unscoped query would otherwise
// resolve against whatever workspace happened to be selected (EVIDENCE.md §9.8).
func tree() []byte {
	b, err := output("--id-format", "both", "top", "--all", "--json")
	if err != nil {
		return nil
	}
	return b
}

func parseTop(b []byte) map[int]Titles {
	out := map[int]Titles{}
	for _, n := range parseNodes(b) {
		for _, pid := range n.Pids {
			out[pid] = Titles{n.ID, StripSpinner(n.Title), n.WsTitle, n.Ws}
		}
	}
	return out
}

func parseNodes(b []byte) []node {
	var root any
	if json.Unmarshal(b, &root) != nil {
		return nil
	}
	var out []node
	walk(root, "", "", &out)
	return out
}

// walk descends the whole tree rather than a known path: cmux nests surfaces
// under panels under tabs under workspaces, and that shape has changed before.
func walk(n any, ws, wsTitle string, out *[]node) {
	switch v := n.(type) {
	case map[string]any:
		kind, _ := v["kind"].(string)
		if kind == "workspace" {
			ws, _ = v["id"].(string)
			wsTitle, _ = v["title"].(string)
		}
		if kind == "surface" {
			*out = append(*out, surfaceNode(v, ws, wsTitle))
		}
		for _, c := range v {
			walk(c, ws, wsTitle, out)
		}
	case []any:
		for _, c := range v {
			walk(c, ws, wsTitle, out)
		}
	}
}

func surfaceNode(v map[string]any, ws, wsTitle string) node {
	n := node{Ws: ws, WsTitle: wsTitle}
	n.ID, _ = v["id"].(string)
	n.Title, _ = v["title"].(string)
	n.Pane, _ = v["pane_id"].(string)
	n.Selected, _ = v["selected_in_pane"].(bool)
	if f, ok := v["index_in_pane"].(float64); ok {
		n.Index = int(f)
	}
	pids, _ := v["cmux_process_pids"].([]any)
	for _, p := range pids {
		if f, ok := p.(float64); ok {
			n.Pids = append(n.Pids, int(f))
		}
	}
	return n
}

// StripSpinner drops the leading activity glyph an agent puts in a tab title so labels stay
// stable between renders. A rotating frame in a label means the label changes on every redraw,
// which is the whole reason this exists.
//
// Three families, because three have turned up: ✳ while a turn is queued, a braille frame, and
// the quarter-circle rotation ◐◑◒◓ — the last of which was missing and shipped a visible defect
// for as long as it was (EVIDENCE.md §9.40). Any new spinner is one more range here, so the
// symptom to recognise is a label that flickers between ticks.
//
// Only the *leading* glyph is activity. A quarter circle mid-label is somebody's text.
func StripSpinner(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > 0 && isSpinner(r[0]) {
		return strings.TrimSpace(string(r[1:]))
	}
	return string(r)
}

func isSpinner(r rune) bool {
	switch {
	case r == '✳':
		return true
	case r >= 0x2800 && r <= 0x28FF: // braille, any frame
		return true
	case r >= 0x25D0 && r <= 0x25D3: // ◐ ◑ ◒ ◓
		return true
	}
	return false
}
