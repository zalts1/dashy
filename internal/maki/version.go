package maki

import "github.com/zalts1/dashy/internal/host"

// Version is what `maki --version` says, or "" if it cannot be asked. It lives here for
// the same reason claude's does: this package is the only one that talks to maki, and a
// version report is a different concern from the roster but not a different source.
func Version() string { return host.Probe("maki", "--version") }
