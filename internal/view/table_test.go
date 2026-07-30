package view

import (
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// The one-shot table is often piped or pasted, so it carries no colour and no
// escape sequences at all.
func TestTableIsPlainText(t *testing.T) {
	out := Table(fixture(), 45*time.Minute)
	if strings.Contains(out, "\033") {
		t.Error("table emitted an escape sequence")
	}
}

func TestTableRowsAndSummary(t *testing.T) {
	f := fixture()
	f.Blocked, f.Stale = 2, 1
	out := Table(f, 45*time.Minute)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	if !strings.HasPrefix(lines[0], "STATE") || !strings.Contains(lines[0], "IDLE") {
		t.Errorf("header = %q", lines[0])
	}
	// Header, five rows, a blank line, the summary.
	if len(lines) != 1+len(f.Rows)+2 {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), 1+len(f.Rows)+2, out)
	}

	if !strings.Contains(lines[3], "⚠") {
		t.Errorf("stale row lost its warning mark: %q", lines[3])
	}
	if strings.Contains(lines[1], "⚠") {
		t.Errorf("fresh row marked stale: %q", lines[1])
	}

	summary := lines[len(lines)-1]
	// The running count is derived here rather than carried on the fleet; if the
	// bands and this line ever disagree, one of them is lying.
	for _, want := range []string{"5 sessions", "2 blocked", "1 running", "1 quiet >45m"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q is missing %q", summary, want)
		}
	}
}

func TestTableTruncatesRatherThanWrapping(t *testing.T) {
	f := board.Fleet{Rows: []board.Row{
		{State: "done", Label: strings.Repeat("x", 100), Workspace: strings.Repeat("w", 40)},
	}}
	out := strings.TrimRight(Table(f, time.Hour), "\n")
	for _, line := range strings.Split(out, "\n") {
		if strings.Count(line, "\n") > 0 {
			t.Fatal("a row wrapped")
		}
	}
	if !strings.Contains(out, "…") {
		t.Error("an over-long label was not truncated")
	}
}
