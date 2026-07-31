package hooks

import "testing"

// installedEvents is the read-only half of the installer, and it must agree with what
// Install writes — a diagnosis keyed on a different marker than the installer uses would
// report hooks as missing on a machine that has them.
func TestInstalledEventsFindsBothWiredEvents(t *testing.T) {
	settings := map[string]any{"hooks": map[string]any{
		"Stop": []any{map[string]any{"hooks": []any{
			map[string]any{"type": "command", "command": "/Users/x/.local/bin/board notify"}}}},
		"Notification": []any{map[string]any{"hooks": []any{
			map[string]any{"type": "command", "command": "/Users/x/.local/bin/board notify"}}}},
	}}
	got := installedEvents(settings, "board notify")
	if len(got) != 2 {
		t.Fatalf("found %v, want both Stop and Notification", got)
	}
}

// Half-installed is its own state: one event wired and one not is what a partial install
// or a hand-edited settings file leaves behind, and reporting it as "installed" would
// hide a hook that never fires.
func TestInstalledEventsReportsAPartialInstall(t *testing.T) {
	settings := map[string]any{"hooks": map[string]any{
		"Stop": []any{map[string]any{"hooks": []any{
			map[string]any{"type": "command", "command": "/Users/x/.local/bin/board notify"}}}},
	}}
	got := installedEvents(settings, "board notify")
	if len(got) != 1 || got[0] != "Stop" {
		t.Errorf("got %v, want only Stop", got)
	}
}

// Another tool's hooks are not ours. This is the same rule TestHasCommand pins one level
// down, checked here because the whole settings shape is what doctor reads.
func TestInstalledEventsIgnoresOtherToolsHooks(t *testing.T) {
	settings := map[string]any{"hooks": map[string]any{
		"Stop": []any{map[string]any{"hooks": []any{
			map[string]any{"type": "command", "command": "/usr/bin/some-other-tool notify"}}}},
	}}
	if got := installedEvents(settings, "board notify"); len(got) != 0 {
		t.Errorf("got %v, want nothing — those hooks belong to another tool", got)
	}
}

// A settings file with no hooks block at all is the fresh-install case and must not
// panic on the missing map.
func TestInstalledEventsHandlesSettingsWithNoHooks(t *testing.T) {
	if got := installedEvents(map[string]any{}, "board notify"); len(got) != 0 {
		t.Errorf("got %v, want nothing", got)
	}
}
