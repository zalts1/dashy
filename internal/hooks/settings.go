package hooks

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/zalts1/dashy/internal/host"
)

// This file is the whole of board's access to ~/.claude/settings.json — the one file it
// edits that it does not own. Install and Uninstall share it so that the care taken on
// one path cannot quietly go missing from the other.

func settingsPath() string { return host.Home(".claude", "settings.json") }

// defaultSettingsMode applies only when board creates the file. 0600 is what Install has
// always written and what Claude Code's own file carries: a settings file that may come
// to hold credentials is not one to widen on somebody else's behalf.
const defaultSettingsMode fs.FileMode = 0o600

// readSettings parses the file and reports the bytes and mode alongside it, so a writer
// can put both back. A missing file is the fresh-install answer, not a failure.
//
// A parse failure is a refusal (§8): board cannot know what it would be destroying, and
// the honest move on somebody else's config is to stop.
func readSettings(path string) (map[string]any, []byte, fs.FileMode, error) {
	settings := map[string]any{}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil, defaultSettingsMode, nil
		}
		return nil, nil, 0, err
	}
	if err := json.Unmarshal(b, &settings); err != nil {
		return nil, nil, 0, fmt.Errorf("refusing to rewrite unparseable %s: %w", path, err)
	}
	mode := defaultSettingsMode
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return settings, b, mode, nil
}

// backup copies the file as it stood before the change, named for the moment it was
// taken so several are orderable.
//
// The name is second-resolution, so an install and an uninstall inside the same second
// collide — and the copy already there is the older one, taken before board had touched
// the file at all. That is the copy somebody restoring wants, so an existing backup is
// never overwritten: the point of this file is to survive board being wrong, which a
// backup that can be silently replaced by a later state does not do.
func backup(path string, original []byte, mode fs.FileMode) error {
	if original == nil {
		return nil
	}
	bak := fmt.Sprintf("%s.board-bak-%s", path, time.Now().Format("20060102-150405"))
	if _, err := os.Stat(bak); err == nil {
		fmt.Println("kept earlier backup:", bak)
		return nil
	}
	if err := os.WriteFile(bak, original, mode); err != nil {
		return err
	}
	fmt.Println("backed up:", bak)
	return nil
}

// writeSettings renames a finished file over the target, because the rename is the only
// step a reader can see and it is atomic. os.WriteFile truncated first and wrote second;
// on ~/.board.json that window cost a todo list (§9.27), and here it would cost the
// user's Claude Code configuration.
//
// The mode is carried over explicitly rather than inherited. CreateTemp opens at 0600 and
// rename keeps the temp file's mode, so without this line an atomic write would silently
// tighten a file board only borrows — the same overreach as leaving litter in the
// directory, just harder to notice.
func writeSettings(path string, settings map[string]any, mode fs.FileMode) error {
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Same directory: a rename across filesystems is a copy, and a copy is not atomic.
	f, err := os.CreateTemp(filepath.Dir(path), ".settings.json-*")
	if err != nil {
		return err
	}
	// Removed on every path. After a successful rename nothing is there and this is a
	// harmless miss; on every failure it is what keeps a temp file out of the user's
	// .claude directory.
	defer os.Remove(f.Name())
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
