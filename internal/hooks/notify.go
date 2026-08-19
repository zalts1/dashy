// Package hooks is board's agent integration: the notify entrypoint, and the installers
// that wire board into both agents. Two mechanisms, because the agents have two — hooks in
// a JSON settings file for Claude Code, a Lua block and its permission manifest for maki
// (maki.go, DESIGN.md §17).
//
// Invariant: notify must never fail the agent. It runs inside the agent's own hook
// chain, so every error path here is a silent success — including a broken
// notify_cmd — and a sink that does not answer is abandoned on a deadline rather
// than waited for (§9.30).
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
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
	// The sink gets a budget, not a promise. Discarding the error honours "never fail the
	// agent" for failures; it did nothing for latency, and a hook that blocks has failed
	// slowly — a notify_cmd pointing at a dead host held every Stop hook until Claude
	// Code's own hook timeout fired (§9.30).
	//
	// The deadline bounds what board waits for, not what the sink does: killing `sh`
	// leaves a `curl` it spawned to finish on its own, which is harmless because nothing
	// here reads the result.
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "sh", "-c", st.Config.NotifyCmd)
	c.Stdin = bytes.NewReader(payload)
	c.Run()
}

// A push that arrives after you have already looked is not a push, so the budget is
// small on purpose: it is the ceiling on what the agent pays for a sink that is not
// answering, on every single hook.
const defaultNotifyTimeout = 3 * time.Second

// A var only so a test can shorten it — the value that ships is the constant above.
var notifyTimeout = defaultNotifyTimeout
