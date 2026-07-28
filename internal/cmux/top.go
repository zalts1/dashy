package cmux

import (
	"encoding/json"
	"strings"
)

// Titles is what cmux knows about one session's tab.
type Titles struct {
	ID        string // surface UUID, the handle Focus needs
	Surface   string // tab title
	Workspace string
}

// TitlesByPid maps agent pid -> its surface and workspace titles. cmux surface
// nodes carry no UUID in the tree, so pid is the join key; cmux_process_pids is 1:1.
func TitlesByPid() map[int]Titles {
	b, err := output("--id-format", "both", "top", "--all", "--json")
	if err != nil {
		return map[int]Titles{}
	}
	return parseTop(b)
}

func parseTop(b []byte) map[int]Titles {
	out := map[int]Titles{}
	var root any
	if json.Unmarshal(b, &root) != nil {
		return out
	}
	walk(root, "", out)
	return out
}

// walk descends the whole tree rather than a known path: cmux nests surfaces
// under panels under tabs under workspaces, and that shape has changed before.
func walk(n any, ws string, out map[int]Titles) {
	switch v := n.(type) {
	case map[string]any:
		kind, _ := v["kind"].(string)
		title, _ := v["title"].(string)
		if kind == "workspace" {
			ws = title
		}
		if kind == "surface" {
			id, _ := v["id"].(string)
			pids, _ := v["cmux_process_pids"].([]any)
			for _, p := range pids {
				if f, ok := p.(float64); ok {
					out[int(f)] = Titles{id, StripSpinner(title), ws}
				}
			}
		}
		for _, c := range v {
			walk(c, ws, out)
		}
	case []any:
		for _, c := range v {
			walk(c, ws, out)
		}
	}
}

// StripSpinner drops the leading activity glyph Claude Code puts in tab titles
// (✳ or a braille spinner frame) so labels stay stable between renders.
func StripSpinner(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > 0 && (r[0] == '✳' || (r[0] >= 0x2800 && r[0] <= 0x28FF)) {
		return strings.TrimSpace(string(r[1:]))
	}
	return string(r)
}
