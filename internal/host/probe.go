package host

import (
	"bytes"
	"strings"
)

// Probe asks a binary what version it is and returns the first line of its answer,
// or "" if the binary is missing, silent, or unintelligible. It never returns an
// error: every caller reports absence rather than failing (§8), and this is the one
// query that runs precisely when the machine is broken.
//
// A non-zero exit is not treated as failure. Some tools print a version and exit 1;
// what matters is whether anything usable came back.
func Probe(bin string, args ...string) string {
	out, _ := Output(bin, args...)
	return firstLine(out)
}

// firstLine is Probe's only judgement, split out so it can be pinned by a test: the
// answer belongs on one line of a three-line report, so a tool that prints a banner
// must not widen it.
func firstLine(b []byte) string {
	line, _, _ := bytes.Cut(b, []byte("\n"))
	return strings.TrimSpace(string(line))
}
