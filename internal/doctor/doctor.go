// Package doctor answers "why is board not working" on a machine that is not the
// author's. It reports the wiring: every upstream version, whether each of board's reads
// succeeded, whether the hooks are installed on both agents, and where its own state
// lives.
//
// It is the escalation from `version` (§13), not a replacement for it: version is one line
// per tool to paste into a bug report, doctor is the diagnosis. The overlap is real and
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
	"github.com/zalts1/dashy/internal/editor"
	"github.com/zalts1/dashy/internal/hooks"
	"github.com/zalts1/dashy/internal/maki"
	"github.com/zalts1/dashy/internal/preview"
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

	// The maki roster is two reads that fail together, so one error covers both: the
	// processes running now, and the reports they wrote. MakiProcs above MakiReports of
	// zero is the shape of a missing hook, which is the one maki failure a reader can fix.
	MakiProcs    int
	MakiReports  int
	MakiSessions int
	MakiErr      error
	// NoMaki is not a fault. board reports on whichever agents are installed, so the rows
	// below say nothing about maki when it is absent — the version block has already.
	NoMaki bool

	// The preview read, in the two halves preview.Roster carries: PreviewsSeen is every
	// route portless names, Previews are the ones with a live process behind them. Seen
	// above Previews of zero is the shape of a stale routes.json — no row will carry a link
	// and portless still lists three things as up.
	//
	// This row exists because a failed preview read is invisible in the frame. A missing
	// roster costs every row and takes the header's own slot (§14); a missing preview costs
	// one glyph, so nothing on screen would say board looked (§18).
	Previews     int
	PreviewsSeen int
	PreviewErr   error
	// The Storybook half, in the same two pieces: Storybooks are the ones a row can carry,
	// StorybooksSeen every socket found in the range. Reported only when something is
	// listening — on a machine where nobody runs Storybook there is nothing to diagnose, and
	// a permanent "0 storybooks" is a clause readers learn to skip (§9.13).
	Storybooks     int
	StorybooksSeen int
	// NoPortless is not a fault, for the reason NoMaki is not: previews are optional and a
	// machine without portless still gets a folder link on every row.
	NoPortless bool

	// Editor is the name of the editor folder links open, empty when board found none —
	// which is the one answer that explains a missing glyph on every row at once.
	// EditorFound is whether its bundle is actually here: a configured editor is honoured
	// even when it is not, and then the link is drawn and may reach nothing (§18).
	Editor      string
	EditorFound bool

	// The pull requests cmux has correlated: PRs is how many of the fleet's tabs have one, Open
	// how many of those still want something done about them. Reported only when there is one,
	// the same exception rule the Storybook clause follows (§9.13).
	PRs     int
	OpenPRs int
	// LinksInCmux is where a ⌘-click lands: a cmux browser tab, or the system browser. cmux's
	// preference, never board's — reported because "why did that open there" has no other answer
	// in board's output (§9.42).
	LinksInCmux bool

	Hooks    []string // events with board's notify hook wired up
	HooksErr error    // settings.json unreadable — the state install-hooks refuses on

	// The maki hook is two files, and a block without a manifest is installed and inert:
	// maki denies every permission to an init.lua with no plugin.toml beside it (§9.32).
	MakiHooked   bool
	MakiManifest bool
	MakiHooksErr error // init.lua markers unbalanced — the state install-hooks refuses on

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

	noMaki := !maki.Available()
	var roster maki.Roster
	var makiErr error
	if !noMaki {
		roster, makiErr = maki.Read()
	}
	makiSessions := 0
	for _, rep := range roster.Reports {
		makiSessions += len(rep.Sessions)
	}
	noPortless := !preview.Available()
	var previews preview.Roster
	var previewErr error
	if !noPortless {
		previews, previewErr = preview.Read()
	}

	ed := editor.Gather(st.Config.Editor, config.Path())

	// The same read board's frame makes, over the workspaces cmux knows about.
	wsIDs := map[string]bool{}
	for _, t := range titles {
		if t.WorkspaceID != "" {
			wsIDs[t.WorkspaceID] = true
		}
	}
	ids := make([]string, 0, len(wsIDs))
	for id := range wsIDs {
		ids = append(ids, id)
	}
	spaces := cmux.WorkspaceStates(ids)
	pulls, openPRs := 0, 0
	for _, st := range spaces {
		if st.PR.URL == "" {
			continue
		}
		pulls++
		if st.PR.Open() {
			openPRs++
		}
	}

	makiHooked, makiManifest, makiHooksErr := hooks.MakiInstalled()

	spans := map[string]bool{}
	for _, t := range titles {
		if t.Workspace != "" {
			spans[t.Workspace] = true
		}
	}
	installed, hooksErr := hooks.Installed()
	_, statErr := os.Stat(config.Path())

	return Report{
		Versions:       version.Report(),
		Sessions:       len(agents),
		RosterErr:      rosterErr,
		Tabs:           len(titles),
		Workspaces:     len(spans),
		NoCmux:         !cmux.Available(),
		MakiProcs:      len(roster.Pids),
		MakiReports:    len(roster.Reports),
		MakiSessions:   makiSessions,
		MakiErr:        makiErr,
		NoMaki:         noMaki,
		Previews:       len(previews.Routes),
		PreviewsSeen:   previews.Listed,
		Storybooks:     len(previews.Storybooks),
		StorybooksSeen: previews.Listeners,
		PreviewErr:     previewErr,
		NoPortless:     noPortless,
		Editor:         ed.Chosen.Name,
		EditorFound:    ed.Installed[ed.Chosen.Name],
		PRs:            pulls,
		OpenPRs:        openPRs,
		LinksInCmux:    cmux.OpensLinksInternally(),
		Hooks:          installed,
		HooksErr:       hooksErr,
		MakiHooked:     makiHooked,
		MakiManifest:   makiManifest,
		MakiHooksErr:   makiHooksErr,
		ConfigPath:     config.Path(),
		ConfigOnDisk:   statErr == nil,
		NotifyOn:       st.Config.NotifyCmd != "",
		NotifyCmd:      st.Config.NotifyCmd,
	}
}

// Format renders the report. The version rows come from version.Format so there is one
// place that knows how to print a version string, including the stutter rule (§9.23);
// the rest align to the same label width.
//
// Two rows carry both agents rather than each agent getting a row of its own: `roster`
// is what board read, `hooks` is how it was wired, and those are the concerns — splitting
// them by agent would put the label `maki` on two different lines of a nine-line report.
func Format(r Report) string {
	var b strings.Builder
	b.WriteString(version.Format(r.Versions))
	row := func(label, answer string) {
		fmt.Fprintf(&b, "%-*s %s\n", version.LabelWidth, label, answer)
	}

	// The error's own words, not the frame's. The frame says "roster unreadable · board
	// doctor", which inside doctor would be circular, and the wrapped error carries the
	// detail a maintainer asks for next.
	claudeRoster := fmt.Sprintf("%d claude %s", r.Sessions, plural(r.Sessions, "session"))
	if r.RosterErr != nil {
		claudeRoster = r.RosterErr.Error()
	}
	row("roster", claudeRoster+makiRoster(r))

	switch {
	case r.NoCmux:
		row("tabs", "cmux not found")
	default:
		row("tabs", fmt.Sprintf("%d %s in %d %s",
			r.Tabs, plural(r.Tabs, "tab"), r.Workspaces, plural(r.Workspaces, "workspace")))
	}

	// `links`, not `preview`: the row answers "can a row point anywhere", and both of its
	// halves are stated in those terms. It is also six characters, which is what keeps the
	// answer column where version.LabelWidth puts it — widening a documented constant to
	// fit a label is the tail wagging the dog (§13).
	row("links", previewRow(r)+storybookRow(r)+editorRow(r)+prRow(r)+browserRow(r))

	claudeHooks, claudeOK := "not installed", false
	switch {
	case r.HooksErr != nil:
		claudeHooks = r.HooksErr.Error()
	case len(r.Hooks) > 0:
		// Named individually because a half-install is its own state: one event wired and
		// one not means a hook that never fires, and "installed" would hide it.
		claudeHooks, claudeOK = strings.Join(r.Hooks, ", "), true
	}
	makiHooks, makiOK := makiHooks(r)
	// The repair is named once, on the row that needs it, however many halves are missing.
	repair := ""
	if !claudeOK || !makiOK {
		repair = " — run board install-hooks"
	}
	row("hooks", claudeHooks+makiHooks+repair)

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

// previewRow says what the preview read found. It gets a row of its own rather than a half
// of `roster`, because a preview is not a session: the roster row answers "how much work is
// there", and this one answers "can a row point at it" (§18).
func previewRow(r Report) string {
	switch {
	case r.NoPortless:
		// Optional, like maki, and stated as what the reader still has rather than as an
		// absence: without portless every row still carries its folder.
		return "no portless — rows link to folders only"
	case r.PreviewErr != nil:
		// Verbatim, for the reason the roster errors are: the wrapped error carries the
		// detail a maintainer asks for next.
		return r.PreviewErr.Error()
	case r.Previews == 0 && r.PreviewsSeen > 0:
		// Both halves rather than a conclusion. routes.json outlives the dev servers it
		// names, so this is a file full of processes that have exited — and it is also what
		// a machine with nothing but `portless alias` entries looks like.
		return fmt.Sprintf("%d portless %s, none live",
			r.PreviewsSeen, plural(r.PreviewsSeen, "route"))
	case r.Previews == 0:
		return "portless installed, no routes up"
	default:
		return fmt.Sprintf("%d portless %s", r.Previews, plural(r.Previews, "route"))
	}
}

// editorRow is the folder half of the `links` row: which editor opens one, and whether
// board can see it. Named rather than counted, because there is exactly one and its name is
// the argument to the command that changes it.
func editorRow(r Report) string {
	switch {
	case r.Editor == "":
		// No editor board can build a URL for. The one answer that explains a folder glyph
		// missing from every row at once — and the repair is a command, not a config edit.
		return " · no editor — board editor"
	case !r.EditorFound:
		// Configured and honoured, and the bundle is not where board looks. Stated as both
		// halves rather than as a conclusion: an app installed elsewhere still works, and a
		// name for an editor that is genuinely gone does not.
		return " · " + r.Editor + ", not installed here"
	default:
		return " · " + r.Editor
	}
}

// storybookRow is the Storybook clause of the `links` row, and it is silent when nothing is
// listening in the range: board scans every tick whether or not anybody uses Storybook, and a
// permanent "0 storybooks" would be noise on most machines (§9.13).
func storybookRow(r Report) string {
	switch {
	case r.StorybooksSeen == 0:
		return ""
	case r.Storybooks == 0:
		// Listening and unplaceable: the process is not inside any session's worktree, which
		// is the whole diagnosis. Stated as both halves rather than as a conclusion.
		return fmt.Sprintf(" · %d storybook %s outside every worktree",
			r.StorybooksSeen, plural(r.StorybooksSeen, "port"))
	default:
		return fmt.Sprintf(" · %d %s", r.Storybooks, plural(r.Storybooks, "storybook"))
	}
}

// prRow is the pull-request clause, silent when the fleet's tabs have none — the same exception
// rule the Storybook clause follows, since board asks on every tick whether or not anybody is
// using pull requests (§9.13).
//
// Both numbers, because they are different facts: how many tabs cmux found a pull request for, and
// how many of those are still open. A fleet of merged PRs is a fleet with nothing to do.
func prRow(r Report) string {
	if r.PRs == 0 {
		return ""
	}
	return fmt.Sprintf(" · %d %s, %d open", r.PRs, plural(r.PRs, "pr"), r.OpenPRs)
}

// browserRow says where a ⌘-click lands. It is cmux's preference and board only reports it —
// but it is reported always, because it is the answer to a question every reader of the links
// eventually asks and board's output is otherwise silent on it (§9.42).
func browserRow(r Report) string {
	if r.LinksInCmux {
		return " · cmux browser"
	}
	return " · system browser"
}

// makiRoster is the maki half of the roster row: what the two reads found, or nothing at
// all on a machine without maki.
func makiRoster(r Report) string {
	switch {
	case r.NoMaki:
		return ""
	case r.MakiErr != nil:
		// Verbatim, for the reason the claude roster error is: the frame's one-liner says
		// "maki roster unreadable · board doctor", which inside doctor would be circular.
		return " · " + r.MakiErr.Error()
	case r.MakiProcs > 0 && r.MakiReports == 0:
		// The two reads disagreeing is the diagnosis. Stated as both halves rather than as
		// a conclusion, because "no reports" is also what a maki started this second looks
		// like for the moment before its first turn.
		return fmt.Sprintf(" · %d maki running, no reports", r.MakiProcs)
	default:
		return fmt.Sprintf(" · %d maki %s in %d %s",
			r.MakiSessions, plural(r.MakiSessions, "session"),
			r.MakiReports, plural(r.MakiReports, "tab"))
	}
}

// makiHooks is the maki half of the hooks row, and whether it counts as wired. Naming
// init.lua rather than an event list because that is the whole mechanism on this side:
// one Lua block, present or not.
func makiHooks(r Report) (string, bool) {
	switch {
	case r.NoMaki:
		// Nothing to wire up is not an unfinished install, so it must not summon a repair.
		return "", true
	case r.MakiHooksErr != nil:
		return " · " + r.MakiHooksErr.Error(), false
	case r.MakiHooked && !r.MakiManifest:
		// Installed and inert, which reads as installed everywhere else: the block is in
		// the file, maki grants it nothing, and no report ever arrives (§9.32).
		return " · maki init.lua without plugin.toml", false
	case r.MakiHooked:
		return " · maki init.lua", true
	default:
		return " · maki not wired", false
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
