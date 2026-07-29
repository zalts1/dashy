// Package cmux reads the terminal multiplexer board reports on. It supplies tab
// titles, workspace names, the idle clock and the one write action board has
// (focus a tab) — but never the session roster: cmux's hook file misses live
// sessions, so it is enrichment only. See EVIDENCE.md §9.3.
package cmux

import (
	"os"
	"os/exec"

	"board/internal/host"
)

func output(args ...string) ([]byte, error) { return host.Output("cmux", args...) }

// Available reports whether we are running under cmux at all.
func Available() bool {
	if os.Getenv("CMUX_WORKSPACE_ID") != "" {
		return true
	}
	_, err := exec.LookPath("cmux")
	return err == nil
}

// CurrentSurface and CurrentWorkspace identify the session board itself was
// invoked from. Only the label command and the notify hook care; every other
// caller wants the whole fleet.
func CurrentSurface() string   { return os.Getenv("CMUX_SURFACE_ID") }
func CurrentWorkspace() string { return os.Getenv("CMUX_WORKSPACE_ID") }
