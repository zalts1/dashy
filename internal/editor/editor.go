// Package editor answers one question: which editor a row's folder link opens.
//
// The link is a URL and board runs nothing (§8), so an editor is supportable exactly when
// it registers a URL scheme that takes a path. Three do, in the same shape: VS Code
// documents `vscode://file/<path>`, Cursor is a VS Code fork and registers `cursor`, and
// Zed's own open_listener strips the prefix `zed://file` and parses the remainder as a
// path. One template covers all three.
//
// board picks one and says which. There is no way for it to ask at the moment of the
// click — the terminal opens the link and tells board nothing — so the chooser is a
// command instead: `board editor`. DESIGN.md §18 is the whole argument.
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalts1/dashy/internal/host"
)

// Editor is one supported editor: the name the config and `board editor` use, the bundle
// that is also how board detects it, and the URL scheme the link is built from.
type Editor struct {
	Name   string
	App    string
	Scheme string
}

// Known is every editor board can build a link for, and its order is the tie-break when
// several are installed and none is configured.
//
// **Alphabetical, deliberately.** board has no opinion about which editor is better, and
// any other order would be one — encoded in a table nobody reads, deciding for everybody
// who never runs `board editor`. Alphabetical is arbitrary in a way that is honest about
// being arbitrary, and it is stable: installing a second editor cannot silently move the
// link somewhere else unless the new one sorts first, and `doctor` names the choice either
// way.
//
// Adding an editor is this list plus a line in the README, and the bar is the one above:
// a registered URL scheme that takes a path. JetBrains IDEs are the near miss — `idea://`
// wants the path in a query parameter, which is a different template, not a fourth row.
var Known = []Editor{
	{Name: "cursor", App: "Cursor.app", Scheme: "cursor"},
	{Name: "vscode", App: "Visual Studio Code.app", Scheme: "vscode"},
	{Name: "zed", App: "Zed.app", Scheme: "zed"},
}

// Lookup resolves a name from the config or the command line.
func Lookup(name string) (Editor, bool) {
	for _, e := range Known {
		if e.Name == name {
			return e, true
		}
	}
	return Editor{}, false
}

// Names is the list a refusal prints, so the error names the whole vocabulary rather than
// making the reader guess the spelling of the one they wanted.
func Names() string {
	var n []string
	for _, e := range Known {
		n = append(n, e.Name)
	}
	return strings.Join(n, ", ")
}

// Report is one machine's answer, in the shape version.Info and doctor.Report have:
// impure to gather, pure to render (§11).
type Report struct {
	// Chosen is what folder links open. The zero Editor means board found none, which is a
	// real state and the reason a row can carry no folder glyph at all.
	Chosen Editor
	// Configured is the name in ~/.board.json, empty when the choice is automatic. Kept
	// apart from Chosen because they are different facts: one is a decision on disk, the
	// other a default that can move when something is installed.
	Configured string
	Installed  map[string]bool
	ConfigPath string
}

// Gather reads the machine. Judgement-free: choose and Format hold all of it.
func Gather(configured, configPath string) Report {
	installed := map[string]bool{}
	for _, e := range Known {
		if installedAt(e) != "" {
			installed[e.Name] = true
		}
	}
	return Report{
		Chosen:     choose(configured, installed),
		Configured: configured,
		Installed:  installed,
		ConfigPath: configPath,
	}
}

// Scheme is the one thing the renderer needs: the scheme to build folder links from, or ""
// when there is no editor to open them in.
func Scheme(configured string) string {
	installed := map[string]bool{}
	for _, e := range Known {
		if installedAt(e) != "" {
			installed[e.Name] = true
		}
	}
	return choose(configured, installed).Scheme
}

// choose is the policy, and it is three lines because that is the whole of it: what you
// said, then what is here, in Known's order.
//
// A configured editor wins even when board cannot find its bundle. The user said so, and
// an app installed somewhere board does not look is likelier than a typo — `Lookup` has
// already rejected the typos. `doctor` and `board editor` both flag the mismatch rather
// than second-guessing it here.
func choose(configured string, installed map[string]bool) Editor {
	if e, ok := Lookup(configured); ok {
		return e
	}
	for _, e := range Known {
		if installed[e.Name] {
			return e
		}
	}
	return Editor{}
}

// installedAt finds the bundle, or "" — /Applications first, then the per-user one, which
// is where a copy for one account lands.
func installedAt(e Editor) string {
	for _, dir := range []string{"/Applications", host.Home("Applications")} {
		p := filepath.Join(dir, e.App)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Format is `board editor`: the chooser, and the closest board can get to an "open with"
// panel. It has to answer three things at once — what folder links open now, what else is
// here to choose, and the one command that changes it — because a listing missing any of
// them sends the reader to the README (§9.14: name the route that exists).
func Format(r Report) string {
	var b strings.Builder
	if r.Chosen.Name == "" {
		fmt.Fprintf(&b, "  no editor found — rows carry no folder link\n\n")
		fmt.Fprintf(&b, "  board can open a folder in any of these:\n")
		for _, e := range Known {
			fmt.Fprintf(&b, "    %-8s %s\n", e.Name, e.App)
		}
		fmt.Fprintf(&b, "\n  install one, or name it anyway:  board editor %s\n", Known[0].Name)
		return b.String()
	}

	how := "automatically"
	if r.Configured != "" {
		how = "set in " + r.ConfigPath
	}
	fmt.Fprintf(&b, "  folder links open  %s   %s://   (%s)\n\n", r.Chosen.App, r.Chosen.Scheme, how)

	for _, e := range Known {
		lead, where := "  ", e.App
		if e.Name == r.Chosen.Name {
			// The one mark in the listing, for the row that answers the question asked.
			lead = "→ "
		}
		if !r.Installed[e.Name] {
			where = "not installed"
		}
		fmt.Fprintf(&b, "  %s%-8s %s\n", lead, e.Name, where)
	}

	// Named only when there is one to name, and never the one already chosen: a listing
	// that offers you what you have is a listing you stop reading (§9.14 read forwards).
	if other := suggest(r); other != "" {
		fmt.Fprintf(&b, "\n  change it:  board editor %s\n", other)
	}
	if r.Configured != "" {
		fmt.Fprintf(&b, "  reset:      board editor auto\n")
	}
	return b.String()
}

// suggest names one editor other than the chosen one, preferring one that is actually
// here: the point of the line is a command the reader can paste and have work.
func suggest(r Report) string {
	for _, e := range Known {
		if e.Name != r.Chosen.Name && r.Installed[e.Name] {
			return e.Name
		}
	}
	for _, e := range Known {
		if e.Name != r.Chosen.Name {
			return e.Name
		}
	}
	return ""
}
