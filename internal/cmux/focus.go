package cmux

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Focus brings a session's tab to the front. This is board's only write action
// against cmux, and it is the whole reason board exists alongside `claude agents`:
// attach opens a transcript, focus brings back the tab with its splits and
// scrollback. See DESIGN.md §7 Interaction.
//
// The parameter really is surface_id; passing "surface" returns invalid_params.
func Focus(id string) error {
	if id == "" {
		return fmt.Errorf("session has no surface id")
	}
	out, err := output("rpc", "surface.focus", fmt.Sprintf(`{"surface_id":%q}`, id))
	if err != nil {
		return fmt.Errorf("cmux focus failed: %w", err)
	}
	if strings.Contains(string(out), "error") {
		return fmt.Errorf("cmux focus refused: %s", strings.TrimSpace(string(out)))
	}
	return nil
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
