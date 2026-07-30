package version

import (
	"strings"
	"testing"
)

// Format is the whole reason this package is not three Printf calls in cmd/board:
// the interesting cases are the broken machines, and they are only testable if the
// three strings arrive as data (§11).

func TestFormatReportsAllThree(t *testing.T) {
	got := Format(Info{
		Board:  "v0.1.0",
		Claude: "2.1.220 (Claude Code)",
		Cmux:   "0.64.16 (96) [5321becb6]",
	})
	for _, want := range []string{"v0.1.0", "2.1.220 (Claude Code)", "0.64.16 (96) [5321becb6]"} {
		if !strings.Contains(got, want) {
			t.Errorf("version output is missing %q — a bug report needs all three:\n%s", want, got)
		}
	}
	if n := len(strings.Split(strings.TrimRight(got, "\n"), "\n")); n != 3 {
		t.Errorf("got %d lines, want one per tool:\n%s", n, got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("output does not end in a newline; it is meant to be pasted")
	}
}

// The upstream strings are reported verbatim rather than parsed down to a number.
// Both are undocumented surfaces that have changed shape before (§9.1, §9.3), and
// cmux's build number and commit are exactly what a maintainer would ask for next.
func TestFormatDoesNotParseUpstreamStrings(t *testing.T) {
	got := Format(Info{Board: "v0.1.0", Claude: "2.1.220 (Claude Code)", Cmux: "0.64.16 (96) [5321becb6]"})
	if strings.Contains(got, "2.1.220\n") {
		t.Error("claude's version was trimmed to a bare number; report it verbatim")
	}
}

// A missing tool is the normal case for this command: it is what someone runs when
// board is not working. Every line must still name its tool and say something.
func TestFormatMissingToolsStillReportThreeLines(t *testing.T) {
	cases := []struct {
		name string
		in   Info
	}{
		{"no claude", Info{Board: "v0.1.0", Cmux: "0.64.16"}},
		{"no cmux", Info{Board: "v0.1.0", Claude: "2.1.220"}},
		{"nothing at all", Info{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Format(c.in)
			lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
			if len(lines) != 3 {
				t.Fatalf("got %d lines, want 3 even when tools are missing:\n%s", len(lines), got)
			}
			for _, tool := range []string{"board", "claude", "cmux"} {
				if !strings.Contains(got, tool) {
					t.Errorf("no line for %q:\n%s", tool, got)
				}
			}
			for i, l := range lines {
				if fields := strings.Fields(l); len(fields) < 2 {
					t.Errorf("line %d is a bare label with no answer: %q", i, l)
				}
			}
		})
	}
}

// An absent tool and an unreadable one are different facts, and the difference is the
// first thing to check: "not found" sends you to the install, "unknown" does not.
func TestFormatNamesTheKindOfAbsence(t *testing.T) {
	got := Format(Info{})
	if !strings.Contains(got, "not found") {
		t.Errorf("a missing claude/cmux does not read as not found:\n%s", got)
	}
	if !strings.Contains(got, "unknown") {
		t.Errorf("board with no build info does not read as unknown:\n%s", got)
	}
}

// "(devel)" is what ReadBuildInfo returns for a build with no VCS directory — a
// tarball, or a copied tree. It is an honest answer and must survive verbatim rather
// than being laundered into "unknown", which would hide how the binary was made.
func TestFormatKeepsDevelVerbatim(t *testing.T) {
	got := Format(Info{Board: "(devel)"})
	if !strings.Contains(got, "(devel)") {
		t.Errorf("(devel) was rewritten; it is the answer, not a failure:\n%s", got)
	}
}

// `cmux --version` answers "cmux 0.64.16 (96) [5321becb6]" — it names itself, and
// `claude --version` does not. Printed under a label, the first stutters. Dropping a
// leading token that just repeats the label is the one edit made to an upstream
// string, and it is not a parse: everything after it survives untouched.
func TestFormatDropsAStutteringToolName(t *testing.T) {
	got := Format(Info{Cmux: "cmux 0.64.16 (96) [5321becb6]"})
	if strings.Contains(got, "cmux   cmux") {
		t.Errorf("the tool name is printed twice:\n%s", got)
	}
	if !strings.Contains(got, "0.64.16 (96) [5321becb6]") {
		t.Errorf("dropping the label took the version with it:\n%s", got)
	}
}

// The stutter rule is anchored to the label and must not eat a real first token.
func TestFormatKeepsANonStutteringFirstToken(t *testing.T) {
	got := Format(Info{Claude: "2.1.220 (Claude Code)"})
	if !strings.Contains(got, "2.1.220 (Claude Code)") {
		t.Errorf("claude's version was altered:\n%s", got)
	}
	// A version that is only the tool's name is nothing to report, not an empty line.
	if bare := Format(Info{Cmux: "cmux"}); !strings.Contains(bare, "cmux   not found") {
		t.Errorf("a bare tool name with no version should read as absent:\n%s", bare)
	}
}

// The labels line up so three pasted lines read as a block.
func TestFormatAlignsVersions(t *testing.T) {
	got := Format(Info{Board: "v0.1.0", Claude: "2.1.220", Cmux: "0.64.16"})
	var cols []int
	for _, l := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		cols = append(cols, strings.Index(l, strings.Fields(l)[1]))
	}
	for i, c := range cols {
		if c != cols[0] {
			t.Errorf("line %d starts its version at column %d, want %d:\n%s", i, c, cols[0], got)
		}
	}
}
