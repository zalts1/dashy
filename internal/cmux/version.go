package cmux

import "github.com/zalts1/dashy/internal/host"

// Version is what `cmux --version` says, or "" if it cannot be asked. It carries a
// build number and a commit as well as the version — reported verbatim, because that
// is the part a maintainer asks for after the version itself.
func Version() string { return host.Probe("cmux", "--version") }
