package cmux

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Focus brings a session's tab to the front. This is board's only write action
// against cmux, and it is the whole reason board exists alongside `claude agents`:
// attach opens a transcript, focus brings back the tab with its splits and
// scrollback. See DESIGN.md §7 Interaction.
//
// It takes two calls, because cmux keeps "which tab is showing" and "who has the
// keyboard" as separate state (DESIGN.md §9.15).
func Focus(id string) error {
	if id == "" {
		return fmt.Errorf("session has no surface id")
	}
	// Raises the window and selects the workspace. The parameter really is
	// surface_id; passing "surface" returns invalid_params (§9.8).
	out, err := output("rpc", "surface.focus", fmt.Sprintf(`{"surface_id":%q}`, id))
	if err != nil {
		return fmt.Errorf("cmux focus failed: %w", err)
	}
	if strings.Contains(string(out), "error") {
		return fmt.Errorf("cmux focus refused: %s", strings.TrimSpace(string(out)))
	}
	return selectTab(id)
}

// tabSlot is where a surface sits in its pane's tab strip, expressed as the sibling
// it sits next to rather than as an index. cmux shifts every later index when a tab
// closes, and board reads the tree up to a tick before it writes: an index that went
// stale in that window would silently reorder the user's tabs.
type tabSlot struct {
	Surface   string
	Workspace string
	After     string // put it back after this sibling...
	Before    string // ...or ahead of this one, when it is first in the strip
	Selected  bool   // already the front tab, so there is nothing to do
}

// selectTab brings the surface to the front of its pane.
//
// surface.focus does not do this. It selects the workspace and focuses the pane, and
// the pane then re-asserts whichever tab it last showed — so a jump landed on the
// right workspace and the wrong session (DESIGN.md §9.15). cmux has no select verb:
// `tab-action` covers rename, close and pin but nothing that fronts a tab. Reordering
// a surface onto the slot it already occupies, with focus, is the narrowest call that
// does, and is a no-op on the tab strip itself.
func selectTab(id string) error {
	slot, ok := findSlot(tree(), id)
	if !ok {
		return fmt.Errorf("cmux knows no tab strip for surface %s", id)
	}
	if slot.Selected {
		return nil
	}
	if err := slot.apply(); err != nil {
		return err
	}
	// Confirm against the tree, not against the reply. This whole defect hid for as
	// long as it did because surface.focus answers with a success payload naming the
	// very surface it declined to select.
	if after, ok := findSlot(tree(), id); !ok || !after.Selected {
		return fmt.Errorf("cmux did not bring the tab forward")
	}
	return nil
}

func (s tabSlot) apply() error {
	p := map[string]any{
		"surface_id":   s.Surface,
		"workspace_id": s.Workspace,
		"focus":        true,
	}
	if s.After != "" {
		p["after_surface_id"] = s.After
	} else {
		p["before_surface_id"] = s.Before
	}
	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("cmux select failed: %w", err)
	}
	out, err := output("rpc", "surface.reorder", string(b))
	if err != nil {
		return fmt.Errorf("cmux select failed: %w", err)
	}
	if strings.Contains(string(out), "error") {
		return fmt.Errorf("cmux select refused: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// findSlot locates a surface in the tree and names the sibling to place it against.
// Pure over the bytes: the placement is what would reorder somebody's tabs if it were
// wrong, so it is pinned by a fixture rather than read for correctness.
func findSlot(b []byte, surface string) (tabSlot, bool) {
	nodes := parseNodes(b)
	var me node
	for _, n := range nodes {
		if n.ID == surface {
			me = n
			break
		}
	}
	// No pane means no strip to place within. Treating paneless surfaces as one
	// group would move tabs between unrelated panes.
	if me.ID == "" || me.Pane == "" {
		return tabSlot{}, false
	}
	slot := tabSlot{Surface: me.ID, Workspace: me.Ws, Selected: me.Selected}
	if slot.Selected {
		return slot, true
	}

	var strip []node
	for _, n := range nodes {
		if n.Pane == me.Pane {
			strip = append(strip, n)
		}
	}
	// Array order is not tab order; index_in_pane is.
	sort.Slice(strip, func(i, j int) bool { return strip[i].Index < strip[j].Index })
	for i, n := range strip {
		if n.ID != me.ID {
			continue
		}
		if i > 0 {
			slot.After = strip[i-1].ID
		} else if i+1 < len(strip) {
			slot.Before = strip[i+1].ID
		}
		break
	}
	// Unselected and alone in its pane is a contradiction: cmux is mid-change, and
	// there is no neighbour to anchor against.
	if slot.After == "" && slot.Before == "" {
		return tabSlot{}, false
	}
	return slot, true
}

// WorkspaceTitle resolves a workspace UUID to its display name.
func WorkspaceTitle(id string) string {
	if id == "" {
		return ""
	}
	b, err := output("workspace", "list", "--json", "--id-format", "both")
	if err != nil {
		return ""
	}
	var f struct {
		Workspaces []struct{ ID, Title string } `json:"workspaces"`
	}
	json.Unmarshal(b, &f)
	for _, w := range f.Workspaces {
		if strings.EqualFold(w.ID, id) {
			return w.Title
		}
	}
	return ""
}
