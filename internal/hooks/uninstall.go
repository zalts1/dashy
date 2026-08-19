package hooks

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalts1/dashy/internal/config"
)

// Uninstall is Install's inverse, on both agents, and for the same reason it attempts
// both halves whatever the other one did.
func Uninstall() error {
	return errors.Join(uninstallClaude(), uninstallMaki())
}

// uninstallClaude removes board's hooks from the user's Claude Code settings. It holds
// the same line installClaude does (§8): it refuses a file it cannot parse, it backs up
// before its first change, and having nothing to remove is a report rather than an error.
//
// It exists because the alternative was a README paragraph asking the reader to delete
// two entries out of somebody else's JSON by eye. Install being safe while uninstall is
// manual is not a tool you can ask a colleague to try.
func uninstallClaude() error {
	path := settingsPath()
	settings, original, mode, err := readSettings(path)
	if err != nil {
		return err
	}
	m, err := marker()
	if err != nil {
		return err
	}
	hooks, _ := settings["hooks"].(map[string]any)
	var removed []string
	for _, event := range hookEvents {
		list, ok := hooks[event].([]any)
		if !ok {
			continue
		}
		kept, n := withoutCommand(list, m)
		if n == 0 {
			continue
		}
		// An empty hook list and an absent key are the same instruction to Claude Code, so
		// dropping the container cannot change what runs — and it is the only choice that
		// leaves no trace of a tool that has been removed. Leaving `"Stop": []` behind
		// would mean each install/uninstall cycle accretes scaffolding nobody owns.
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
		removed = append(removed, event)
	}
	if len(removed) == 0 {
		fmt.Printf("no board hooks in %s — nothing to do\n", path)
		return nil
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	if err := backup(path, original, mode); err != nil {
		return err
	}
	if err := writeSettings(path, settings, mode); err != nil {
		return err
	}
	fmt.Printf("removed hooks: %s\n", strings.Join(removed, ", "))
	// Named because uninstall is when someone is leaving, and this is the only other
	// thing board wrote. It holds the labels and the todos, which exist nowhere else.
	fmt.Printf("board's own state is still at %s\n", config.Path())
	return nil
}

// withoutCommand is hasCommand's counterpart: every entry naming board, out of every
// group, with everything it did not write left as it was found.
//
// The entry is the unit, not the group. board writes one command per group, but this
// file is the user's and nothing stops a group holding board's command beside another
// tool's — so a group is dropped only once board's own removal has emptied it, and a
// group that arrived empty is left alone because board did not empty it.
func withoutCommand(list []any, marker string) ([]any, int) {
	kept := make([]any, 0, len(list))
	removed := 0
	for _, group := range list {
		g, ok := group.(map[string]any)
		if !ok {
			kept = append(kept, group)
			continue
		}
		entries, ok := g["hooks"].([]any)
		if !ok {
			kept = append(kept, group)
			continue
		}
		before := removed
		live := make([]any, 0, len(entries))
		for _, e := range entries {
			h, _ := e.(map[string]any)
			if c, _ := h["command"].(string); strings.Contains(c, marker) {
				removed++
				continue
			}
			live = append(live, e)
		}
		if removed == before {
			kept = append(kept, group)
			continue
		}
		if len(live) == 0 {
			continue
		}
		g["hooks"] = live
		kept = append(kept, g)
	}
	return kept, removed
}
