package cmux

import (
	"encoding/json"
	"os"
	"time"

	"board/internal/host"
)

// hookSession is one record in cmux's Claude Code hook file. Only the clock is
// used; lifecycle fields are deliberately ignored — needsInput fires ~60s after
// any finished turn, so it means "sitting at the prompt", not "asked you something".
type hookSession struct {
	SessionID string  `json:"sessionId"`
	UpdatedAt float64 `json:"updatedAt"`
}

// HookClock maps session id -> last hook activity. This is the most accurate idle
// clock available, but the file misses sessions entirely, so callers must have a
// fallback.
func HookClock() map[string]time.Time {
	b, err := os.ReadFile(host.Home(".cmuxterm", "claude-hook-sessions.json"))
	if err != nil {
		return map[string]time.Time{}
	}
	return parseHookClock(b)
}

func parseHookClock(b []byte) map[string]time.Time {
	out := map[string]time.Time{}
	var file struct {
		Sessions map[string]hookSession `json:"sessions"`
	}
	if json.Unmarshal(b, &file) != nil {
		return out
	}
	// One session can appear under several surface keys; the newest wins.
	for _, s := range file.Sessions {
		if t := time.Unix(int64(s.UpdatedAt), 0); t.After(out[s.SessionID]) {
			out[s.SessionID] = t
		}
	}
	return out
}
