// Package host isolates the two ways board touches the machine it runs on:
// where files live, and how child processes are run. Everything else in the tree
// goes through here, so there is one place to look when either changes.
package host

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Home joins parts onto the user's home directory.
func Home(parts ...string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(append([]string{h}, parts...)...)
}

// Output runs a read-only query and returns its stdout.
//
// cmux treats CMUX_SURFACE_ID/CMUX_WORKSPACE_ID as the implicit target of every
// command, so a stale inherited value makes even a global query fail. They are
// blanked for every child process, not just cmux's, because board is always
// launched from inside a session and must never ask about that session by accident.
func Output(bin string, args ...string) ([]byte, error) {
	c := exec.Command(bin, args...)
	c.Env = append(os.Environ(), "CMUX_QUIET=1",
		"CMUX_SURFACE_ID=", "CMUX_WORKSPACE_ID=", "CMUX_TAB_ID=", "CMUX_PANEL_ID=")
	return c.Output()
}

// Look reports whether a binary is on PATH, without running it. exec.LookPath by another name,
// wrapped here so callers do not import os/exec for one question and so there is still one place
// that knows how board touches the machine.
func Look(bin string) (string, error) { return exec.LookPath(bin) }
