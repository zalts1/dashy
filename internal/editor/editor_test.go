package editor

import (
	"strings"
	"testing"
)

// choose is the whole of the policy, and it is pure so the interesting cases are the ones
// nobody has on their machine (§11). Everything below is a fixture: which editors are
// installed, and what the config says.
func TestChoose(t *testing.T) {
	all := map[string]bool{"cursor": true, "vscode": true, "zed": true}
	cases := []struct {
		name       string
		configured string
		installed  map[string]bool
		want       string
	}{
		// One installed and nothing configured: no ambiguity, so no choice to make.
		{"only one", "", map[string]bool{"vscode": true}, "vscode"},
		{"only zed", "", map[string]bool{"zed": true}, "zed"},
		// Several installed and nothing configured: alphabetical, deliberately — board has
		// no opinion about which editor is better, and a stable answer beats a clever one.
		{"several", "", all, "cursor"},
		{"several without cursor", "", map[string]bool{"vscode": true, "zed": true}, "vscode"},
		// Configured wins over everything, including over being the only one installed.
		{"configured", "zed", all, "zed"},
		{"configured against the order", "vscode", all, "vscode"},
		// Configured but not installed here is still honoured: the user said so, and board
		// is a reporting surface — refusing to build a URL because it could not find an app
		// bundle would be board overruling an explicit instruction. doctor flags it.
		{"configured elsewhere", "zed", map[string]bool{"cursor": true}, "zed"},
		// A name board does not know cannot become a URL scheme.
		{"configured nonsense", "notepad", all, "cursor"},
		// Nothing installed and nothing configured: no editor, and so no folder link at all.
		// A link to nothing is worse than no link.
		{"nothing", "", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := choose(c.configured, c.installed).Name; got != c.want {
				t.Errorf("choose(%q, %v) = %q, want %q", c.configured, c.installed, got, c.want)
			}
		})
	}
}

// The scheme is what the link is built from, so a wrong one is a glyph that opens nothing.
func TestSchemes(t *testing.T) {
	want := map[string]string{"cursor": "cursor", "vscode": "vscode", "zed": "zed"}
	if len(Known) != len(want) {
		t.Fatalf("Known has %d editors, want %d", len(Known), len(want))
	}
	for _, e := range Known {
		if e.Scheme != want[e.Name] {
			t.Errorf("%s scheme = %q, want %q", e.Name, e.Scheme, want[e.Name])
		}
		if !strings.HasSuffix(e.App, ".app") {
			t.Errorf("%s app = %q, want a bundle name", e.Name, e.App)
		}
	}
	// Known is the documented order and the tie-break, so it may not drift out of
	// alphabetical: the order *is* the policy (§18).
	for i := 1; i < len(Known); i++ {
		if Known[i-1].Name >= Known[i].Name {
			t.Errorf("Known is not alphabetical at %d: %q then %q",
				i, Known[i-1].Name, Known[i].Name)
		}
	}
}

func TestLookup(t *testing.T) {
	if e, ok := Lookup("zed"); !ok || e.Scheme != "zed" {
		t.Errorf("Lookup(zed) = %+v, %v", e, ok)
	}
	if _, ok := Lookup("notepad"); ok {
		t.Error("Lookup accepted an editor board does not know")
	}
	if _, ok := Lookup(""); ok {
		t.Error("Lookup accepted an empty name")
	}
}

// Format is `board editor`, and it is the chooser: it has to say what folder links open
// now, what else is here to choose, and the one command that changes it. A listing that
// omits any of the three sends the reader to the README.
func TestFormat(t *testing.T) {
	r := Report{
		Chosen:     Editor{"cursor", "Cursor.app", "cursor"},
		Installed:  map[string]bool{"cursor": true, "vscode": true},
		ConfigPath: "/Users/x/.board.json",
	}
	got := Format(r)
	for _, want := range []string{"Cursor.app", "cursor://", "vscode", "not installed",
		"board editor vscode"} {
		if !strings.Contains(got, want) {
			t.Errorf("listing does not say %q:\n%s", want, got)
		}
	}
	// Automatic and pinned are different facts — one is a default that moves when you
	// install something, the other is a decision on disk — and only one of them has
	// anything to reset. Offering `auto` to somebody who is already on it is the kind of
	// line a reader learns to skip (§9.14 read forwards).
	if !strings.Contains(got, "automatically") {
		t.Errorf("an automatic choice is not reported as automatic:\n%s", got)
	}
	if strings.Contains(got, "board editor auto") {
		t.Errorf("an automatic choice offers a reset to automatic:\n%s", got)
	}
	r.Configured = "cursor"
	pinned := Format(r)
	if strings.Contains(pinned, "automatically") {
		t.Errorf("a configured editor is still reported as automatic:\n%s", pinned)
	}
	if !strings.Contains(pinned, "board editor auto") {
		t.Errorf("a pinned editor offers no way back to automatic:\n%s", pinned)
	}
	if !strings.Contains(pinned, r.ConfigPath) {
		t.Errorf("a pinned editor does not say where the decision lives:\n%s", pinned)
	}
}

// The state nobody wants and everybody hits once: no editor found. It must say so rather
// than print a listing with a blank at the top, because it is also the explanation for a
// missing glyph.
func TestFormatWithNoEditor(t *testing.T) {
	got := Format(Report{ConfigPath: "/Users/x/.board.json"})
	if !strings.Contains(got, "no editor") {
		t.Errorf("listing does not say no editor was found:\n%s", got)
	}
	for _, e := range Known {
		if !strings.Contains(got, e.Name) {
			t.Errorf("listing does not name %q as installable:\n%s", e.Name, got)
		}
	}
}

// A configured editor board cannot find is worth naming: it is the one shape where the
// glyph is there, the link is built, and clicking it may reach nothing.
func TestFormatFlagsAConfiguredEditorThatIsMissing(t *testing.T) {
	got := Format(Report{
		Chosen:     Editor{"zed", "Zed.app", "zed"},
		Configured: "zed",
		Installed:  map[string]bool{"cursor": true},
		ConfigPath: "/Users/x/.board.json",
	})
	if !strings.Contains(got, "not installed") {
		t.Errorf("a configured editor that is absent is not flagged:\n%s", got)
	}
}
