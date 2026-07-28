package hooks

import (
	"encoding/json"
	"testing"
)

// install-hooks is idempotent via this marker check. If it ever returns false for a
// hook it already wrote, a second run appends a duplicate and the agent fires the
// same hook twice.
func TestHasCommand(t *testing.T) {
	const settings = `[
	  {"hooks": [{"type": "command", "command": "/usr/bin/other-tool"}]},
	  {"hooks": [{"type": "command", "command": "/Users/x/.local/bin/board notify"}]}
	]`
	var list []any
	if err := json.Unmarshal([]byte(settings), &list); err != nil {
		t.Fatal(err)
	}
	if !hasCommand(list, "board notify") {
		t.Error("existing board hook not detected; a second install would duplicate it")
	}
	if hasCommand(list, "other notify") {
		t.Error("matched a hook belonging to a different tool")
	}
	if hasCommand(nil, "board notify") {
		t.Error("matched against no hooks at all")
	}
	// Shapes board did not write must not panic or match: this file belongs to the
	// user, and anything can be in it.
	var junk []any
	json.Unmarshal([]byte(`[{"hooks": "not-a-list"}, {}, null, 3]`), &junk)
	if hasCommand(junk, "board notify") {
		t.Error("matched malformed settings")
	}
}
