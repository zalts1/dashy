package cmux

import (
	"testing"
	"time"
)

// The shape cmux top emits: workspaces containing tabs containing panels containing
// surfaces, with the pids on the surface and no session id anywhere. Nesting depth
// has changed before, which is why the parser walks rather than indexes.
const topJSON = `{
  "workspaces": [
    {"kind": "workspace", "title": "APP", "id": "WS-1", "tabs": [
      {"kind": "tab", "title": "tab", "panels": [
        {"kind": "panel", "surfaces": [
          {"kind": "surface", "id": "S-1", "title": "✳ merge app#1497", "cmux_process_pids": [101]}
        ]}
      ]}
    ]},
    {"kind": "workspace", "title": "REVIEWS", "id": "WS-2", "tabs": [
      {"kind": "tab", "panels": [
        {"kind": "surface", "id": "S-2", "title": "review PR", "cmux_process_pids": [202, 203]},
        {"kind": "surface", "id": "S-3", "title": "no pids", "cmux_process_pids": []}
      ]}
    ]}
  ]
}`

func TestParseTop(t *testing.T) {
	got := parseTop([]byte(topJSON))
	if len(got) != 3 {
		t.Fatalf("parsed %d pids, want 3: %+v", len(got), got)
	}
	if s := got[101]; s.ID != "S-1" || s.Workspace != "APP" {
		t.Errorf("pid 101 = %+v, want S-1 in APP", s)
	}
	// The workspace UUID is threaded down from the workspace node, because it is what
	// `sidebar-state --workspace` is addressed by (§18).
	if got[101].WorkspaceID != "WS-1" {
		t.Errorf("pid 101 workspace id = %q, want WS-1", got[101].WorkspaceID)
	}
	// The spinner glyph must be gone, or the label changes on every redraw.
	if got[101].Surface != "merge app#1497" {
		t.Errorf("title = %q, want the spinner stripped", got[101].Surface)
	}
	// A surface can host several processes; each pid must resolve to the same tab.
	if got[202].ID != "S-2" || got[203].ID != "S-2" {
		t.Errorf("multi-pid surface: %+v / %+v", got[202], got[203])
	}
	if s, ok := got[0]; ok {
		t.Errorf("a surface with no pids produced an entry: %+v", s)
	}
	// Malformed input is silently empty: board reports, it does not crash.
	if n := len(parseTop([]byte("not json"))); n != 0 {
		t.Errorf("garbage parsed into %d entries", n)
	}
}

func TestStripSpinner(t *testing.T) {
	cases := map[string]string{
		"✳ merge app#1497": "merge app#1497",
		"⠋ thinking":       "thinking", // braille spinner frame
		// The quarter-circle rotation, which is what cmux actually uses for a busy agent. It
		// survived into the label for as long as this function knew only ✳ and braille, so a
		// row's label changed on every redraw — the one thing this function exists to stop
		// (EVIDENCE.md §9.40).
		"◐ Dashy local preview": "Dashy local preview",
		"◑ Dashy local preview": "Dashy local preview",
		"◒ half way":            "half way",
		"◓ last frame":          "last frame",
		"⣿ late frame":          "late frame",
		"plain title":           "plain title",
		"  padded  ":            "padded",
		"":                      "",
		"✳":                     "",
		"✳✳ two glyphs":         "✳ two glyphs", // only the leading one is activity
		"done ✳ mid-string":     "done ✳ mid-string",
		// A quarter circle is a legitimate character mid-label, and only the leading one is
		// activity — the same rule ✳ already followed.
		"phase ◐ of the moon": "phase ◐ of the moon",
		"◐◑ two frames":       "◑ two frames",
	}
	for in, want := range cases {
		if got := StripSpinner(in); got != want {
			t.Errorf("StripSpinner(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseHookClock(t *testing.T) {
	// The same session appears under two surface keys; the newest wins. Sessions
	// missing from this file entirely are the reason callers need a fallback.
	const file = `{"sessions": {
	  "surface-a": {"sessionId": "sess-1", "updatedAt": 1000},
	  "surface-b": {"sessionId": "sess-1", "updatedAt": 2000},
	  "surface-c": {"sessionId": "sess-2", "updatedAt": 1500}
	}}`
	got := parseHookClock([]byte(file))
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2: %v", len(got), got)
	}
	if want := time.Unix(2000, 0); !got["sess-1"].Equal(want) {
		t.Errorf("sess-1 = %v, want the newer %v", got["sess-1"], want)
	}
	if n := len(parseHookClock([]byte("{"))); n != 0 {
		t.Errorf("garbage parsed into %d entries", n)
	}
}
