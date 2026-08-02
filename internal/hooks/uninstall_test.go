package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// withoutCommand is hasCommand's counterpart, and the one that has to be exhaustive: a
// settings file can carry several board entries — an old ~/.local/bin path and a newer
// one — and an uninstall that removes the first and stops leaves a hook pointing at a
// binary that may no longer exist.
func TestWithoutCommandDropsEveryBoardEntry(t *testing.T) {
	list := listOf(t, `[
	  {"hooks": [{"type": "command", "command": "/usr/bin/other-tool notify"}]},
	  {"hooks": [{"type": "command", "command": "/Users/x/.local/bin/board notify"}]},
	  {"hooks": [{"type": "command", "command": "/opt/homebrew/bin/board notify"}]}
	]`)
	got, n := withoutCommand(list, "board notify")
	if n != 2 {
		t.Errorf("removed %d, want 2 — a stale path is still a board hook", n)
	}
	if hasCommand(got, "board notify") {
		t.Error("a board hook survived")
	}
	if !hasCommand(got, "other-tool notify") {
		t.Error("removed a hook belonging to a different tool")
	}
}

// The group is not the unit of removal. board writes one command per group, but this
// file belongs to the user and nothing stops a group holding board's command alongside
// somebody else's — dropping the whole group would uninstall a tool board does not own.
func TestWithoutCommandKeepsForeignEntriesSharingAGroup(t *testing.T) {
	list := listOf(t, `[
	  {"matcher": "*", "hooks": [
	    {"type": "command", "command": "/Users/x/.local/bin/board notify"},
	    {"type": "command", "command": "/usr/bin/other-tool notify"}
	  ]}
	]`)
	got, n := withoutCommand(list, "board notify")
	if n != 1 {
		t.Fatalf("removed %d, want 1", n)
	}
	if len(got) != 1 {
		t.Fatalf("kept %d groups, want 1 — the group still has a live entry", len(got))
	}
	if hasCommand(got, "board notify") {
		t.Error("board's entry survived inside a shared group")
	}
	if !hasCommand(got, "other-tool notify") {
		t.Error("took another tool's hook down with board's")
	}
	// The rest of the group is the user's: a matcher board never wrote must come back.
	g, _ := got[0].(map[string]any)
	if g["matcher"] != "*" {
		t.Errorf("matcher = %v, want \"*\" — rewrote a key board does not own", g["matcher"])
	}
}

// A group board empties is a group board created. Leaving it behind means an
// install/uninstall cycle silently accretes scaffolding.
func TestWithoutCommandDropsAGroupItEmpties(t *testing.T) {
	list := listOf(t, `[{"hooks": [{"type": "command", "command": "/x/board notify"}]}]`)
	got, n := withoutCommand(list, "board notify")
	if n != 1 {
		t.Fatalf("removed %d, want 1", n)
	}
	if len(got) != 0 {
		t.Errorf("kept %d empty groups, want 0", len(got))
	}
}

func TestWithoutCommandLeavesAFileWithNoBoardHooksAlone(t *testing.T) {
	const src = `[{"hooks": [{"type": "command", "command": "/usr/bin/other-tool notify"}]}]`
	list := listOf(t, src)
	got, n := withoutCommand(list, "board notify")
	if n != 0 {
		t.Errorf("removed %d from a file with no board hooks, want 0", n)
	}
	if !reflect.DeepEqual(got, listOf(t, src)) {
		t.Error("rewrote a list it had nothing to remove from")
	}
}

// This file belongs to the user and anything can be in it.
func TestWithoutCommandSurvivesMalformedShapes(t *testing.T) {
	list := listOf(t, `[{"hooks": "not-a-list"}, {}, null, 3, {"hooks": [null, 7]}]`)
	got, n := withoutCommand(list, "board notify")
	if n != 0 {
		t.Errorf("removed %d from malformed settings, want 0", n)
	}
	if len(got) != len(list) {
		t.Errorf("dropped %d malformed groups; shapes board did not write are not board's to delete",
			len(list)-len(got))
	}
}

// The property that matters most: uninstall is install's inverse. Anything else means
// the file a user gets back is not the file they had.
func TestUninstallIsTheInverseOfInstall(t *testing.T) {
	path := settingsAt(t, `{
	  "model": "opus",
	  "hooks": {"PreToolUse": [{"hooks": [{"type": "command", "command": "/usr/bin/audit"}]}]}
	}`)
	before := parsed(t, path)

	if err := Install(); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(parsed(t, path), before) {
		t.Fatal("install changed nothing; the inverse test proves nothing")
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	if got := parsed(t, path); !reflect.DeepEqual(got, before) {
		t.Errorf("uninstall is not install's inverse:\n got %v\nwant %v", got, before)
	}
}

// An empty hook list and an absent key are the same instruction to Claude Code, so
// removing the container cannot change behaviour — and it is the only choice that leaves
// no trace of a tool that has been uninstalled.
func TestUninstallLeavesNoEmptyContainers(t *testing.T) {
	path := settingsAt(t, `{"model": "opus"}`)
	if err := Install(); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	s, _ := parsed(t, path).(map[string]any)
	if _, ok := s["hooks"]; ok {
		t.Errorf("left an empty hooks container behind: %v", s["hooks"])
	}
	if s["model"] != "opus" {
		t.Errorf("model = %v, want opus — dropped a key board does not own", s["model"])
	}
}

func TestUninstallIsIdempotent(t *testing.T) {
	path := settingsAt(t, `{"model": "opus"}`)
	if err := Install(); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	after := readFile(t, path)
	if err := Uninstall(); err != nil {
		t.Errorf("second uninstall returned %v; nothing to do is not a failure", err)
	}
	if got := readFile(t, path); got != after {
		t.Error("a second uninstall rewrote the file it had nothing to change")
	}
}

func TestUninstallWithNoSettingsFileIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Uninstall(); err != nil {
		t.Errorf("Uninstall with no settings file = %v, want nil", err)
	}
}

// §8: the same refusal Install makes. This is a file board does not own, and a parse
// failure means board cannot know what it would be destroying.
func TestUninstallRefusesUnparseableSettings(t *testing.T) {
	path := settingsAt(t, `{"model": "opus", oops`)
	before := readFile(t, path)
	err := Uninstall()
	if err == nil {
		t.Fatal("Uninstall rewrote an unparseable settings file")
	}
	if !strings.Contains(err.Error(), "refusing") {
		t.Errorf("error = %q, want it to say it is refusing", err)
	}
	if readFile(t, path) != before {
		t.Error("touched a file it refused to parse")
	}
}

// ~/.claude/settings.json is not board's file. CreateTemp opens at 0600 and rename
// carries that mode, so an atomic write silently tightens a file board only borrows.
func TestTheSettingsFileKeepsItsMode(t *testing.T) {
	for _, name := range []string{"install", "uninstall"} {
		t.Run(name, func(t *testing.T) {
			path := settingsAt(t, `{"model": "opus"}`)
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := Install(); err != nil {
				t.Fatal(err)
			}
			if name == "uninstall" {
				if err := Uninstall(); err != nil {
					t.Fatal(err)
				}
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := fi.Mode().Perm(); got != 0o644 {
				t.Errorf("mode = %o, want 644 — board changed the permissions of a file it does not own", got)
			}
		})
	}
}

// The same convention Install follows: a copy before the first change, so a mistake is
// recoverable without board having to be right.
func TestUninstallBacksUpBeforeChanging(t *testing.T) {
	path := settingsAt(t, `{"model": "opus"}`)
	if err := Install(); err != nil {
		t.Fatal(err)
	}
	installed := readFile(t, path)
	rmBackups(t, path)
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	baks := backups(t, path)
	if len(baks) != 1 {
		t.Fatalf("wrote %d backups, want 1", len(baks))
	}
	if got := readFile(t, baks[0]); got != installed {
		t.Error("the backup is not the file as it stood before the change")
	}
}

// The backup name is second-resolution, so an install and an uninstall inside the same
// second land on one path. The older copy is the pristine one — the file as it was before
// board ever touched it — and that is precisely the one worth recovering, so a second
// change must not overwrite it.
func TestABackupNeverOverwritesAnEarlierOne(t *testing.T) {
	path := settingsAt(t, `{"model": "opus"}`)
	pristine := readFile(t, path)
	if err := Install(); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	baks := backups(t, path)
	if len(baks) == 0 {
		t.Fatal("no backup at all")
	}
	for _, b := range baks {
		if readFile(t, b) == pristine {
			return
		}
	}
	t.Errorf("no backup holds the file as it was before board touched it; %d backup(s) and the "+
		"pristine copy was overwritten by a later one", len(baks))
}

// No temp file may outlive either call: this directory is the user's, and board leaving
// litter in it is the same overreach as changing its mode.
func TestNeitherPathLeavesATempFile(t *testing.T) {
	path := settingsAt(t, `{"model": "opus"}`)
	if err := Install(); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if n == "settings.json" || strings.Contains(n, ".board-bak-") {
			continue
		}
		t.Errorf("left %q behind in the user's .claude directory", n)
	}
}

// --- helpers ---

func listOf(t *testing.T, src string) []any {
	t.Helper()
	var list []any
	if err := json.Unmarshal([]byte(src), &list); err != nil {
		t.Fatal(err)
	}
	return list
}

// settingsAt isolates $HOME and seeds a settings file in it. Every test here must go
// through this: a test that reached the real ~/.claude/settings.json would break the
// user's Claude Code, not just fail.
func settingsAt(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func parsed(t *testing.T, path string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(readFile(t, path)), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func backups(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".board-bak-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func rmBackups(t *testing.T, path string) {
	t.Helper()
	for _, b := range backups(t, path) {
		if err := os.Remove(b); err != nil {
			t.Fatal(err)
		}
	}
}
