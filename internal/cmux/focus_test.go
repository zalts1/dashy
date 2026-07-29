package cmux

import "testing"

// The real shape of `top --all --json`: surfaces sit in panes, and the pane's tab
// strip is ordered by index_in_pane, not by array order — so the array here is
// deliberately shuffled. The workspace id lives only on the ancestor workspace node;
// the pane id lives on the surface itself.
const slotJSON = `{
  "windows": [
    {"kind": "window", "id": "W-1", "workspaces": [
      {"kind": "workspace", "id": "WS-1", "title": "REVIEWS", "panes": [
        {"kind": "pane", "id": "P-1", "surfaces": [
          {"kind": "surface", "id": "S-2", "pane_id": "P-1", "index_in_pane": 1, "selected_in_pane": true,  "title": "front"},
          {"kind": "surface", "id": "S-3", "pane_id": "P-1", "index_in_pane": 2, "selected_in_pane": false, "title": "third"},
          {"kind": "surface", "id": "S-1", "pane_id": "P-1", "index_in_pane": 0, "selected_in_pane": false, "title": "first"}
        ]},
        {"kind": "pane", "id": "P-2", "surfaces": [
          {"kind": "surface", "id": "S-4", "pane_id": "P-2", "index_in_pane": 0, "selected_in_pane": true, "title": "alone in a split"}
        ]}
      ]},
      {"kind": "workspace", "id": "WS-2", "title": "BOARD", "panes": [
        {"kind": "pane", "id": "P-3", "surfaces": [
          {"kind": "surface", "id": "S-5", "pane_id": "P-3", "index_in_pane": 0, "selected_in_pane": false, "title": "background"},
          {"kind": "surface", "id": "S-6", "pane_id": "P-3", "index_in_pane": 1, "selected_in_pane": true,  "title": "front"}
        ]}
      ]}
    ]}
  ]
}`

func TestFindSlot(t *testing.T) {
	cases := []struct {
		surface string
		want    tabSlot
	}{
		// Mid-strip: placed back after the sibling it already follows.
		{"S-3", tabSlot{Surface: "S-3", Workspace: "WS-1", After: "S-2"}},
		// First in its pane has no predecessor, so it anchors on its successor.
		{"S-1", tabSlot{Surface: "S-1", Workspace: "WS-1", Before: "S-2"}},
		// Already the front tab: nothing to do, and no neighbour is needed.
		{"S-2", tabSlot{Surface: "S-2", Workspace: "WS-1", Selected: true}},
		// Only tab in a split. Selected, so the reorder never fires.
		{"S-4", tabSlot{Surface: "S-4", Workspace: "WS-1", Selected: true}},
		// A second workspace: the id must come from its own ancestor, and the
		// neighbour from its own pane.
		{"S-5", tabSlot{Surface: "S-5", Workspace: "WS-2", Before: "S-6"}},
	}
	for _, c := range cases {
		got, ok := findSlot([]byte(slotJSON), c.surface)
		if !ok {
			t.Errorf("findSlot(%s) not found", c.surface)
			continue
		}
		if got != c.want {
			t.Errorf("findSlot(%s) = %+v, want %+v", c.surface, got, c.want)
		}
	}
}

func TestFindSlotRefusesWhatItCannotPlace(t *testing.T) {
	// A surface board knows about but cmux does not: the caller must report that the
	// tab did not come forward, never guess at a placement.
	if s, ok := findSlot([]byte(slotJSON), "S-99"); ok {
		t.Errorf("unknown surface resolved to %+v", s)
	}
	if s, ok := findSlot([]byte("not json"), "S-1"); ok {
		t.Errorf("garbage resolved to %+v", s)
	}
	// No pane id means no tab strip to place within. Grouping paneless surfaces
	// together would reorder tabs across unrelated panes.
	const noPane = `{"workspaces": [{"kind": "workspace", "id": "WS-1", "panes": [
	  {"kind": "pane", "surfaces": [
	    {"kind": "surface", "id": "S-1", "index_in_pane": 0, "selected_in_pane": false},
	    {"kind": "surface", "id": "S-2", "index_in_pane": 1, "selected_in_pane": true}
	  ]}
	]}]}`
	if s, ok := findSlot([]byte(noPane), "S-1"); ok {
		t.Errorf("paneless surface resolved to %+v", s)
	}
}

func TestFocusRefusesEmptySurface(t *testing.T) {
	// Background agents have no surface. Focus must say so rather than shell out and
	// let cmux pick a target for us.
	if err := Focus(""); err == nil {
		t.Error("Focus(\"\") succeeded; it must refuse")
	}
}
