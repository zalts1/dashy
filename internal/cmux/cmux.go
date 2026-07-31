// Package cmux reads the terminal multiplexer board reports on. It supplies tab
// titles, workspace names, the idle clock and the one write action board has
// (focus a tab) — but never the session roster: cmux's hook file misses live
// sessions, so it is enrichment only. See EVIDENCE.md §9.3.
package cmux

import (
	"os"
	"os/exec"

	"github.com/zalts1/dashy/internal/host"
)

func output(args ...string) ([]byte, error) { return host.Output("cmux", args...) }

// Available reports whether cmux can be asked anything — which is a question about the
// binary, not about the environment. It used to return true for an inherited
// CMUX_WORKSPACE_ID as well, but board is always launched from inside a session, so that
// branch answered "am I under cmux" and hid the case it was meant to catch: env var
// present, binary gone, every tab read empty and nothing said (EVIDENCE.md §9.26).
//
// Being inside a session is a separate fact with its own accessors below.
func Available() bool {
	_, err := exec.LookPath("cmux")
	return err == nil
}

// CurrentSurface and CurrentWorkspace identify the session board itself was
// invoked from. Only the label command and the notify hook care; every other
// caller wants the whole fleet.
func CurrentSurface() string   { return os.Getenv("CMUX_SURFACE_ID") }
func CurrentWorkspace() string { return os.Getenv("CMUX_WORKSPACE_ID") }
