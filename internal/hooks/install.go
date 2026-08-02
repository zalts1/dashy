package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalts1/dashy/internal/config"
)

// hookEvents are the two lifecycle points notify answers to. Shared with Installed so
// the installer and the diagnosis cannot come to disagree about what "installed" means.
var hookEvents = []string{"Stop", "Notification"}

// marker identifies our own hook inside a settings file that may hold several tools'.
// It is the running binary's name, not a constant, so a renamed or copied binary
// installs and reports on itself rather than on whatever it used to be called.
func marker() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Base(self) + " notify", nil
}

// Install merges the Stop and Notification hooks into the user's Claude Code
// settings. It is idempotent via the `<binary> notify` command marker, it refuses to
// touch a file it cannot parse, and it backs up before its first change — this is
// the one file board edits that it does not own.
func Install() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	path := settingsPath()
	settings, original, mode, err := readSettings(path)
	if err != nil {
		return err
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	marker, err := marker()
	if err != nil {
		return err
	}
	var added []string
	for _, event := range hookEvents {
		list, _ := hooks[event].([]any)
		if hasCommand(list, marker) {
			continue
		}
		hooks[event] = append(list, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": self + " notify"}},
		})
		added = append(added, event)
	}
	if len(added) == 0 {
		fmt.Println("hooks already installed — nothing to do")
		return nil
	}
	if err := backup(path, original, mode); err != nil {
		return err
	}
	settings["hooks"] = hooks
	if err := writeSettings(path, settings, mode); err != nil {
		return err
	}
	fmt.Printf("installed hooks: %s\n", strings.Join(added, ", "))
	if config.Load().Config.NotifyCmd == "" {
		fmt.Printf("set notify_cmd in %s to start pushing (it receives JSON on stdin)\n", config.Path())
	}
	return nil
}

func hasCommand(list []any, marker string) bool {
	for _, group := range list {
		g, _ := group.(map[string]any)
		entries, _ := g["hooks"].([]any)
		for _, e := range entries {
			h, _ := e.(map[string]any)
			if c, _ := h["command"].(string); strings.Contains(c, marker) {
				return true
			}
		}
	}
	return false
}
