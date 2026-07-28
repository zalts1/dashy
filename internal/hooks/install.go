package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"board/internal/config"
	"board/internal/host"
)

// Install merges the Stop and Notification hooks into the user's Claude Code
// settings. It is idempotent via the `<binary> notify` command marker, it refuses to
// touch a file it cannot parse, and it backs up before its first change — this is
// the one file board edits that it does not own.
func Install() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	path := host.Home(".claude", "settings.json")
	settings := map[string]any{}
	var original []byte
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("refusing to rewrite unparseable %s: %w", path, err)
		}
		original = b
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	marker := filepath.Base(self) + " notify"
	var added []string
	for _, event := range []string{"Stop", "Notification"} {
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
	if original != nil {
		bak := fmt.Sprintf("%s.board-bak-%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(bak, original, 0o600); err != nil {
			return err
		}
		fmt.Println("backed up:", bak)
	}
	settings["hooks"] = hooks
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
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
