package claude

import "github.com/zalts1/dashy/internal/host"

// Version is what `claude --version` says, or "" if it cannot be asked. It lives here
// because this package is the only one that talks to the claude binary — a version
// report is a different concern from the roster, but not a different source.
func Version() string { return host.Probe("claude", "--version") }
