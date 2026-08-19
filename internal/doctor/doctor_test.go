package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/version"
)

// Format is the whole reason this package is not a handful of Printf calls in
// cmd/board: every interesting case is a broken machine, and they are only reachable
// as fixtures (§11). doctor is the command someone runs *because* board is not working,
// so "it printed nothing" and "it returned an error" are both failures of the command.

func healthy() Report {
	return Report{
		Versions:     version.Info{Board: "v0.1.0", Claude: "2.1.220 (Claude Code)", Cmux: "0.64.16 (96)"},
		Sessions:     16,
		Tabs:         22,
		Workspaces:   7,
		Hooks:        []string{"Stop", "Notification"},
		ConfigPath:   "/Users/x/.board.json",
		ConfigOnDisk: true,
		NotifyOn:     true,
		MakiProcs:    2,
		MakiReports:  2,
		MakiSessions: 3,
		MakiHooked:   true,
		Previews:     1,
		PreviewsSeen: 1,
		Editor:       "cursor",
		EditorFound:  true,
	}
}

// The Storybook clause, and the one thing it must not do: appear on a machine where nobody
// runs Storybook. board scans the port range every tick regardless, so a permanent
// "0 storybooks" would be a clause every reader learns to skip (§9.13).
func TestFormatReportsStorybooks(t *testing.T) {
	cases := []struct {
		name, want string
		r          Report
	}{
		{"one placed", "1 storybook", func() Report { r := healthy(); r.Storybooks, r.StorybooksSeen = 1, 1; return r }()},
		{"several", "3 storybooks", func() Report { r := healthy(); r.Storybooks, r.StorybooksSeen = 3, 3; return r }()},
		// Listening and unplaceable — the process is not inside any session's worktree.
		{"unplaceable", "2 storybook ports outside every worktree",
			func() Report { r := healthy(); r.Storybooks, r.StorybooksSeen = 0, 2; return r }()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Format(c.r); !strings.Contains(got, c.want) {
				t.Errorf("links row does not say %q:\n%s", c.want, got)
			}
		})
	}
	// Nothing listening: silent.
	silent := healthy()
	silent.Storybooks, silent.StorybooksSeen = 0, 0
	if got := Format(silent); strings.Contains(got, "storybook") {
		t.Errorf("a machine with no storybook still gets a clause:\n%s", got)
	}
}

// The `links` row carries both halves of "can a row point anywhere": the preview read, and
// the editor the folder half opens. Two halves on one row for the same reason `roster` has
// two — one concern, two sources that fail independently (§14, §18).
func TestFormatReportsTheEditor(t *testing.T) {
	cases := []struct {
		name string
		r    Report
		want string
	}{
		{"found", healthy(), "cursor"},
		// The state that explains a missing glyph on every row at once.
		{"none", func() Report { r := healthy(); r.Editor, r.EditorFound = "", false; return r }(),
			"no editor"},
		// Configured, honoured, and board cannot find the bundle: the glyph is drawn and the
		// click may reach nothing, which is worth one clause.
		{"configured but absent", func() Report { r := healthy(); r.Editor, r.EditorFound = "zed", false; return r }(),
			"zed, not installed here"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Format(c.r); !strings.Contains(got, c.want) {
				t.Errorf("links row does not say %q:\n%s", c.want, got)
			}
		})
	}
}

// The preview row is the third read, and it is the one whose failure is invisible in the
// frame: a missing preview link costs a glyph, so nothing on screen says board looked and
// found nothing. This row is where it says so (§18).
func TestFormatReportsThePreviewRead(t *testing.T) {
	cases := []struct {
		name string
		r    Report
		want string
	}{
		{"one route", healthy(), "1 portless route"},
		{"several", func() Report { r := healthy(); r.Previews, r.PreviewsSeen = 3, 3; return r }(),
			"3 portless routes"},
		{"none up", func() Report { r := healthy(); r.Previews, r.PreviewsSeen = 0, 0; return r }(),
			"no routes up"},
		// The interesting failure, and the reason the read has two halves: routes.json
		// outlives the dev servers it names, so a file full of dead entries is a machine
		// where no row will ever carry a link and portless still says three things are up.
		{"listed but dead", func() Report { r := healthy(); r.Previews, r.PreviewsSeen = 0, 3; return r }(),
			"3 portless routes, none live"},
		{"no portless", func() Report { r := healthy(); r.NoPortless = true; return r }(),
			"no portless"},
		{"unreadable", func() Report {
			r := healthy()
			r.PreviewErr = errors.New("portless routes answered in an unknown shape")
			return r
		}(),
			"unknown shape"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Format(c.r)
			if !strings.Contains(got, c.want) {
				t.Errorf("preview row does not say %q:\n%s", c.want, got)
			}
		})
	}
}

// Every label is present in every state. A diagnostic that silently omits a row is one
// that answers a question the reader did not ask.
func TestFormatAlwaysReportsEveryLabel(t *testing.T) {
	labels := []string{"board", "claude", "cmux", "roster", "tabs", "links", "hooks", "config", "notify"}
	for _, c := range []struct {
		name string
		r    Report
	}{
		{"healthy", healthy()},
		{"nothing at all", Report{}},
		{"no upstreams", Report{RosterErr: claude.ErrNotInstalled, NoCmux: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Format(c.r)
			for _, l := range labels {
				if !strings.Contains(got, l) {
					t.Errorf("no %q row:\n%s", l, got)
				}
			}
			for i, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
				if len(strings.Fields(line)) < 2 {
					t.Errorf("line %d is a bare label with no answer: %q", i, line)
				}
			}
		})
	}
}

// The roster row carries the error's own words, not the frame's one-liner. The frame
// says "roster unreadable · board doctor"; inside doctor that would be circular, and
// the underlying error carries the detail a maintainer actually needs.
func TestFormatReportsTheRosterErrorVerbatim(t *testing.T) {
	got := Format(Report{RosterErr: errors.New("claude agents answered in an unknown shape: invalid character 'x'")})
	if !strings.Contains(got, "invalid character 'x'") {
		t.Errorf("the roster error lost its detail:\n%s", got)
	}
	if strings.Contains(got, "board doctor") {
		t.Errorf("doctor told the reader to run doctor:\n%s", got)
	}
}

// A working roster states its size, because "0 sessions" and "could not ask" are the
// two answers this row exists to separate.
func TestFormatCountsAReadableRoster(t *testing.T) {
	if got := Format(healthy()); !strings.Contains(got, "16 claude sessions") {
		t.Errorf("roster row does not state the count:\n%s", got)
	}
	got := Format(Report{Sessions: 0})
	if !strings.Contains(got, "0 claude sessions") {
		t.Errorf("an empty but readable roster does not say so:\n%s", got)
	}
}

// Hooks are the wiring `notify` depends on, and "not installed" has to name its repair:
// this is the one row with an action the reader can take immediately.
func TestFormatNamesTheHookRepair(t *testing.T) {
	got := Format(Report{})
	if !strings.Contains(got, "install-hooks") {
		t.Errorf("uninstalled hooks do not name the fix:\n%s", got)
	}
	if on := Format(healthy()); !strings.Contains(on, "Stop") || !strings.Contains(on, "Notification") {
		t.Errorf("installed hooks do not name the events:\n%s", on)
	}
}

// An unparseable settings.json is a different fact from "not installed", and it is the
// one install-hooks refuses on (§8) — so doctor has to distinguish them or the reader
// will keep running a command that keeps refusing.
func TestFormatDistinguishesUnparseableSettings(t *testing.T) {
	got := Format(Report{HooksErr: errors.New("unparseable /Users/x/.claude/settings.json: bad token")})
	if !strings.Contains(got, "unparseable") {
		t.Errorf("an unreadable settings file reads as merely uninstalled:\n%s", got)
	}
}

// doctor output is meant to be pasted into a bug report, and notify_cmd is a shell
// command that routinely carries a webhook URL or a token. Whether it is set is the
// diagnostic; the command itself is the user's secret.
func TestFormatNeverPrintsTheNotifyCommand(t *testing.T) {
	r := healthy()
	r.NotifyCmd = "curl -sS -d @- https://ntfy.sh/secret-topic-abc123"
	got := Format(r)
	if strings.Contains(got, "ntfy.sh") || strings.Contains(got, "secret-topic-abc123") {
		t.Errorf("doctor leaked the notify command into pasteable output:\n%s", got)
	}
	if !strings.Contains(got, "on") {
		t.Errorf("doctor does not say notifications are on:\n%s", got)
	}
	if off := Format(Report{}); !strings.Contains(off, "off") {
		t.Errorf("doctor does not say notifications are off:\n%s", off)
	}
}

// The same rule as the notify command, for the same reason: this output is meant to be
// pasted, and a preview hostname is derived from a branch name — which is somebody's work
// (§18). doctor counts routes and never names one.
func TestFormatNeverPrintsAPreviewURL(t *testing.T) {
	r := healthy()
	r.Previews, r.PreviewsSeen = 2, 2
	got := Format(r)
	for _, leak := range []string{"http", "://", ".localhost"} {
		if strings.Contains(got, leak) {
			t.Errorf("doctor's links row could carry a URL (%q):\n%s", leak, got)
		}
	}
	if !strings.Contains(got, "2 portless routes") {
		t.Errorf("doctor does not count the routes:\n%s", got)
	}
}

// A config file that has never been written is worth saying: it is the normal state on
// a fresh install and it explains an empty todo list and missing labels.
func TestFormatSaysWhenTheConfigDoesNotExistYet(t *testing.T) {
	got := Format(Report{ConfigPath: "/Users/x/.board.json"})
	if !strings.Contains(got, "/Users/x/.board.json") {
		t.Errorf("the config path is missing:\n%s", got)
	}
	if !strings.Contains(got, "not created") {
		t.Errorf("an absent config file is reported as if it existed:\n%s", got)
	}
}

// The rows line up under each other, including the three that come from version.Format:
// one block, one column, or the reader has to hunt for the answer on every line.
func TestFormatAlignsEveryAnswer(t *testing.T) {
	got := Format(healthy())
	var cols []int
	for _, l := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		cols = append(cols, strings.Index(l, strings.Fields(l)[1]))
	}
	for i, c := range cols {
		if c != cols[0] {
			t.Errorf("line %d starts its answer at column %d, want %d:\n%s", i, c, cols[0], got)
		}
	}
}

// The tabs row is the §9.3 failure made visible: a readable roster whose sessions have
// no tabs is the exact shape of the bug where board saw 21 of 31 sessions.
func TestFormatReportsTabsSeparatelyFromTheRoster(t *testing.T) {
	got := Format(Report{Sessions: 16, Tabs: 0, Workspaces: 0})
	if !strings.Contains(got, "16 claude sessions") {
		t.Errorf("roster row missing:\n%s", got)
	}
	if !strings.Contains(got, "0 tabs") {
		t.Errorf("a roster with no tabs does not say so:\n%s", got)
	}
	if missing := Format(Report{NoCmux: true}); !strings.Contains(missing, "not found") {
		t.Errorf("a missing cmux is not reported on the tabs row:\n%s", missing)
	}
}

// board reports on two agents, so every row that describes a read describes both. A
// diagnostic that covers one of them is the same silence §9.26 fixed, one agent up.

func TestFormatReportsTheMakiRosterBesideTheClaudeOne(t *testing.T) {
	got := Format(healthy())
	if !strings.Contains(got, "3 maki sessions in 2 tabs") {
		t.Errorf("the maki roster is missing from the roster row:\n%s", got)
	}
}

// Not installed is the ordinary case, not a fault: board reports on whichever agents are
// here. So the rows that diagnose maki say nothing at all rather than crying wolf — the
// version block above has already said it is absent.
func TestFormatKeepsQuietAboutMakiWhenItIsNotInstalled(t *testing.T) {
	r := healthy()
	r.NoMaki = true
	r.MakiProcs, r.MakiReports, r.MakiSessions, r.MakiHooked = 0, 0, 0, false
	got := Format(r)
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "roster") || strings.HasPrefix(line, "hooks") {
			if strings.Contains(line, "maki") {
				t.Errorf("a machine without maki is nagged about it: %q", line)
			}
		}
	}
}

// The interesting maki failure, and the one a reader can fix: maki is running and board
// is seeing nothing from it. On the board that is one line saying so; here it is the two
// reads side by side, which is what shows *which* half is missing.
func TestFormatShowsAMakiRunningWithNoReports(t *testing.T) {
	r := healthy()
	r.MakiReports, r.MakiSessions, r.MakiHooked = 0, 0, false
	got := Format(r)
	if !strings.Contains(got, "2 maki") || !strings.Contains(got, "no reports") {
		t.Errorf("a running maki with no reports does not say so:\n%s", got)
	}
	if !strings.Contains(got, "install-hooks") {
		t.Errorf("the repair is not named:\n%s", got)
	}
}

// The hooks row states both wirings, and a half install is its own state on this side
// too: board's Lua block missing from init.lua is a maki that never reports.
func TestFormatReportsBothHookInstalls(t *testing.T) {
	if got := Format(healthy()); !strings.Contains(got, "maki init.lua") {
		t.Errorf("the maki hook is not reported as installed:\n%s", got)
	}
	r := healthy()
	r.MakiHooked = false
	got := Format(r)
	if !strings.Contains(got, "not wired") {
		t.Errorf("a missing maki hook reads as installed:\n%s", got)
	}
	if !strings.Contains(got, "install-hooks") {
		t.Errorf("the repair is not named:\n%s", got)
	}
}

// An init.lua whose markers make no sense is the state install-hooks refuses on, exactly
// as an unparseable settings.json is — so doctor has to tell it from "not installed" or
// the reader keeps running a command that keeps refusing.
func TestFormatDistinguishesARefusedInitFile(t *testing.T) {
	r := healthy()
	r.MakiHooked = false
	r.MakiHooksErr = errors.New("refusing to rewrite /Users/x/.config/maki/init.lua: board's block is not closed")
	got := Format(r)
	if !strings.Contains(got, "refusing") {
		t.Errorf("a refused init.lua reads as merely unwired:\n%s", got)
	}
}

// The maki roster's own failure keeps its words, for the reason the claude roster's does:
// the frame says "maki roster unreadable · board doctor", and inside doctor that is
// circular.
func TestFormatReportsTheMakiRosterErrorVerbatim(t *testing.T) {
	r := healthy()
	r.MakiErr = errors.New("maki reports answered in an unknown shape: invalid character 'o'")
	got := Format(r)
	if !strings.Contains(got, "invalid character 'o'") {
		t.Errorf("the maki roster error lost its detail:\n%s", got)
	}
}

// The pull-request clause is silent unless the key is on, because off is the default and a
// permanent "pr off" is a clause every reader learns to skip (§9.13). When it is on, the one
// failure worth telling apart is the one with an obvious repair.
func TestFormatReportsPullRequests(t *testing.T) {
	off := healthy()
	if got := Format(off); strings.Contains(got, "pr") && strings.Contains(got, "github") {
		t.Errorf("the key is off and doctor still talks about pull requests:\n%s", got)
	}
	on := healthy()
	on.GitHubOn, on.PRs = true, 2
	if got := Format(on); !strings.Contains(got, "2 open prs") {
		t.Errorf("links row does not count open pull requests:\n%s", got)
	}
	none := healthy()
	none.GitHubOn, none.PRs = true, 0
	if got := Format(none); !strings.Contains(got, "0 open prs") {
		t.Errorf("links row is silent about a fleet with no open PR:\n%s", got)
	}
	missing := healthy()
	missing.GitHubOn, missing.NoGh = true, true
	if got := Format(missing); !strings.Contains(got, "no gh on PATH") {
		t.Errorf("links row does not name the one repairable PR failure:\n%s", got)
	}
}

// Where a ⌘-click lands is cmux's preference and board's only job is to say which. Reported
// always, because board's output is otherwise silent on it and "why did that open there" has no
// other answer (§9.42).
func TestFormatSaysWhereLinksOpen(t *testing.T) {
	inside := healthy()
	inside.LinksInCmux = true
	if got := Format(inside); !strings.Contains(got, "cmux browser") {
		t.Errorf("links row does not say links open inside cmux:\n%s", got)
	}
	outside := healthy()
	outside.LinksInCmux = false
	if got := Format(outside); !strings.Contains(got, "system browser") {
		t.Errorf("links row does not say links open in the system browser:\n%s", got)
	}
}
