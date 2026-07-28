// Package hooks is board's Claude Code integration: the notify entrypoint and the
// installer that wires it up.
//
// Invariant: notify must never fail the agent. It runs inside the agent's own hook
// chain, so every error path here is a silent success — including a broken
// notify_cmd.
package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"board/internal/cmux"
	"board/internal/config"
)

func Notify() {
	st := config.Load()
	// Push is parked: with no sink configured this returns before reading stdin, so
	// there is no network, no credential read and no sink process. See DESIGN.md §10.1 Deferred.
	if st.Config.NotifyCmd == "" {
		return
	}
	var hook struct {
		Event   string `json:"hook_event_name"`
		Message string `json:"message"`
		Cwd     string `json:"cwd"`
	}
	if in, err := io.ReadAll(os.Stdin); err == nil {
		json.Unmarshal(in, &hook)
	}
	surface := cmux.CurrentSurface()
	label := st.Labels[surface]
	if label == "" {
		label = filepath.Base(hook.Cwd)
	}
	ws := cmux.WorkspaceTitle(cmux.CurrentWorkspace())
	if ws == "" {
		ws = "?"
	}
	verb := "finished"
	if hook.Event == "Notification" {
		verb = "needs input"
	}
	text := fmt.Sprintf("%s — %s [%s]", verb, label, ws)
	if hook.Message != "" {
		text += ": " + hook.Message
	}
	payload, _ := json.Marshal(map[string]string{
		"event": hook.Event, "state": verb, "label": label, "workspace": ws,
		"surface_id": surface, "cwd": hook.Cwd, "message": hook.Message, "text": text,
	})
	c := exec.Command("sh", "-c", st.Config.NotifyCmd)
	c.Stdin = bytes.NewReader(payload)
	c.Run()
}
