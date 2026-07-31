// Package version answers the first question asked of any bug report: what is
// installed. board's own version is only a third of that — it reads a roster from
// `claude` and tabs from `cmux`, neither of which is a documented contract, so a
// report that names board alone cannot be acted on (§9.1, §9.3).
//
// The split is the usual one: Report reads the world, Format is pure, and the
// interesting cases are the missing tools (§11).
package version

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
)

// Info is the three answers, each empty when the tool could not be asked.
type Info struct {
	Board  string
	Claude string
	Cmux   string
}

// Report gathers the three. It is the impure half and holds no judgement.
func Report() Info {
	return Info{Board: buildVersion(), Claude: claude.Version(), Cmux: cmux.Version()}
}

// buildVersion reads the version the toolchain stamped in. No -ldflags -X: since Go
// 1.24 a plain `go build` inside the repo synthesises a pseudo-version from VCS, and
// `go install ...@v0.1.0` carries the tag, so the three ways board is actually
// installed all self-report. Only a build with no VCS directory — a tarball or a
// copied tree — falls back to "(devel)", which is itself the useful answer.
func buildVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return bi.Main.Version
}

// LabelWidth is the column the answers start in. Exported because `doctor` prints these
// three lines above four more of its own and they have to line up as one block; a second
// copy of the number is a second thing to change.
const LabelWidth = 6

// Format renders the report as three pasteable lines.
//
// The upstream strings are passed through verbatim rather than parsed down to a
// number: both are undocumented surfaces that have changed shape before, and a parser
// over them is a thing that breaks silently on someone else's machine.
func Format(in Info) string {
	var b strings.Builder
	row := func(tool, got, absent string) {
		if got = unstutter(tool, got); got == "" {
			got = absent
		}
		fmt.Fprintf(&b, "%-*s %s\n", LabelWidth, tool, got)
	}
	// "unknown" and "not found" are different facts: board is always installed, so its
	// version can only be unreadable, while claude and cmux can genuinely be absent.
	row("board", in.Board, "unknown")
	row("claude", in.Claude, "not found")
	row("cmux", in.Cmux, "not found")
	return b.String()
}

// unstutter drops a leading token that only repeats the label — `cmux --version`
// names itself, `claude --version` does not. This is the one edit made to an upstream
// string, and it is deliberately not a parse: the remainder is returned untouched, so
// a change in either tool's format costs at worst a duplicated word.
func unstutter(tool, got string) string {
	got = strings.TrimSpace(got)
	if rest, ok := cutPrefixWord(got, tool); ok {
		return rest
	}
	return got
}

func cutPrefixWord(s, word string) (string, bool) {
	if !strings.EqualFold(s, word) && !strings.HasPrefix(strings.ToLower(s), strings.ToLower(word)+" ") {
		return s, false
	}
	return strings.TrimSpace(s[len(word):]), true
}
