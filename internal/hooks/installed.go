package hooks

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/zalts1/dashy/internal/host"
)

// Installed reports which of board's hooks are wired up. Read-only, deliberately: the
// command that diagnoses this file must never be the one that changes it, and `doctor`
// runs on machines where `install-hooks` would refuse (§8).
//
// No settings file is not an error — it is the fresh-install answer, and it means the
// same thing as an empty list.
func Installed() ([]string, error) {
	m, err := marker()
	if err != nil {
		return nil, err
	}
	path := host.Home(".claude", "settings.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		// The same file Install refuses to rewrite, reported rather than repaired: this is
		// exactly the state where install-hooks keeps saying no, and the reader needs to
		// know that is why.
		return nil, fmt.Errorf("unparseable %s: %w", path, err)
	}
	return installedEvents(settings, m), nil
}

// installedEvents is the judgement, split out so it can be pinned by a fixture: which
// events carry our marker. It shares hookEvents and the marker with Install, because a
// checker keyed on anything else would report hooks as missing on a machine that has
// them.
func installedEvents(settings map[string]any, marker string) []string {
	hooks, _ := settings["hooks"].(map[string]any)
	var out []string
	for _, event := range hookEvents {
		list, _ := hooks[event].([]any)
		if hasCommand(list, marker) {
			out = append(out, event)
		}
	}
	return out
}
