// Package doctor answers "why is board not working" on a machine that is not the
// author's. It reports the wiring: the two upstream versions, whether each of board's
// two reads succeeded, whether the hooks are installed, and where its own state lives.
//
// It is the escalation from `version` (§13), not a replacement for it: version is three
// lines to paste into a bug report, doctor is the diagnosis. The overlap is real and
// deliberate — a version mismatch is the likeliest cause of everything below it, so
// doctor embeds version's Info and calls its Format rather than rendering its own.
//
// Two rules follow from what this command is for. It must never fail and never print an
// empty row, because it runs precisely when everything else is broken (§8). And it must
// never print a secret: its output is meant to be pasted, and notify_cmd routinely
// carries a webhook URL.
package doctor

import (
	"fmt"
	"os"
	"strings"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/hooks"
	"github.com/zalts1/dashy/internal/version"
)

// Report is one machine's wiring. Every field has a meaningful zero: an empty Report
// formats as a machine with nothing installed, which is a real state and the one a
// stranger hits first.
type Report struct {
	Versions version.Info

	// Sessions and RosterErr are the two answers to board's first read. A zero count
	// with no error is a genuinely quiet fleet; an error is a fleet board cannot see.
	Sessions  int
	RosterErr error

	// Tabs, Workspaces and NoCmux are the second read. A readable roster whose sessions
	// have no tabs is the exact shape of §9.3, where board saw 21 of 31 sessions.
	Tabs       int
	Workspaces int
	NoCmux     bool

	Hooks    []string // events with board's notify hook wired up
	HooksErr error    // settings.json unreadable — the state install-hooks refuses on

	ConfigPath   string
	ConfigOnDisk bool
	NotifyOn     bool
	// NotifyCmd is carried so Gather stays a plain description of the machine, and is
	// deliberately never rendered. See TestFormatNeverPrintsTheNotifyCommand.
	NotifyCmd string
}

// Gather reads the world. Impure and judgement-free, mirroring version.Report and
// board.Collect: everything interesting happens in Format, over a fixture (§11).
func Gather() Report {
	st := config.Load()
	agents, rosterErr := claude.Agents()
	titles := cmux.TitlesByPid()

	spans := map[string]bool{}
	for _, t := range titles {
		if t.Workspace != "" {
			spans[t.Workspace] = true
		}
	}
	installed, hooksErr := hooks.Installed()
	_, statErr := os.Stat(config.Path())

	return Report{
		Versions:     version.Report(),
		Sessions:     len(agents),
		RosterErr:    rosterErr,
		Tabs:         len(titles),
		Workspaces:   len(spans),
		NoCmux:       !cmux.Available(),
		Hooks:        installed,
		HooksErr:     hooksErr,
		ConfigPath:   config.Path(),
		ConfigOnDisk: statErr == nil,
		NotifyOn:     st.Config.NotifyCmd != "",
		NotifyCmd:    st.Config.NotifyCmd,
	}
}

// Format renders the report. The three version rows come from version.Format so there
// is one place that knows how to print a version string, including the stutter rule
// (§9.23); the rest align to the same label width.
func Format(r Report) string {
	var b strings.Builder
	b.WriteString(version.Format(r.Versions))
	row := func(label, answer string) {
		fmt.Fprintf(&b, "%-*s %s\n", version.LabelWidth, label, answer)
	}

	// The error's own words, not the frame's. The frame says "roster unreadable · board
	// doctor", which inside doctor would be circular, and the wrapped error carries the
	// detail a maintainer asks for next.
	if r.RosterErr != nil {
		row("roster", r.RosterErr.Error())
	} else {
		row("roster", fmt.Sprintf("%d %s", r.Sessions, plural(r.Sessions, "session")))
	}

	switch {
	case r.NoCmux:
		row("tabs", "cmux not found")
	default:
		row("tabs", fmt.Sprintf("%d %s in %d %s",
			r.Tabs, plural(r.Tabs, "tab"), r.Workspaces, plural(r.Workspaces, "workspace")))
	}

	switch {
	case r.HooksErr != nil:
		row("hooks", r.HooksErr.Error())
	case len(r.Hooks) == 0:
		row("hooks", "not installed — run board install-hooks")
	default:
		// Named individually because a half-install is its own state: one event wired and
		// one not means a hook that never fires, and "installed" would hide it.
		row("hooks", strings.Join(r.Hooks, ", "))
	}

	cfg := r.ConfigPath
	if cfg == "" {
		cfg = config.Path()
	}
	if !r.ConfigOnDisk {
		// Normal on a fresh install, and it explains an empty todo list and missing labels
		// without the reader having to go looking for the file.
		cfg += "  (not created yet)"
	}
	row("config", cfg)

	if r.NotifyOn {
		row("notify", "on  (command not shown)")
	} else {
		row("notify", "off — set notify_cmd to push")
	}
	return b.String()
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
