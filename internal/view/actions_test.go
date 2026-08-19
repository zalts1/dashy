package view

import (
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// linkFleet is the awkward fleet with links on it: one row with both, one with only a
// folder — the ordinary case, a session in a worktree with no dev server up — and one with
// neither, which is what a fleet looked like before any of this existed.
func linkFleet() board.Fleet {
	return board.Fleet{
		Rows: []board.Row{
			{Key: "K-1", State: "running", Label: "build the export endpoint", Workspace: "API",
				Surface: "S-1", Rank: board.RankWorking,
				Folder: "/Users/you/work/repo", Preview: "https://api.localhost"},
			{Key: "K-2", State: "done", Label: "migrate auth handlers", Workspace: "AUTH",
				Surface: "S-2", Idle: 30 * time.Minute, Rank: board.RankQuiet,
				Folder: "/Users/you/work/repo/.claude/worktrees/auth-v2"},
			{Key: "K-3", State: "done", Label: "no directory at all", Workspace: "OPS",
				Surface: "S-3", Idle: 90 * time.Minute, Rank: board.RankQuiet, Stale: true},
			{Key: "K-4", State: "done", Label: "a preview and no folder", Workspace: "WEB",
				Surface: "S-4", Idle: 20 * time.Minute, Rank: board.RankQuiet,
				Preview: "https://web.localhost"},
			{Key: "todo:t1", State: "todo", Label: "book the quarterly review",
				Idle: 26 * time.Hour, Rank: board.RankTodo},
		},
		Stale: 1, Workspaces: 4, Oldest: 90 * time.Minute, TodoCap: 10,
	}
}

func frameOf(f board.Fleet, cols int) string { return frameWith(f, cols, "vscode") }

func frameWith(f board.Fleet, cols int, scheme string) string {
	return Frame(f, Screen{Now: time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC),
		Interval: 10 * time.Second, Threshold: 45 * time.Minute, Rows: 44, Cols: cols,
		EditorScheme: scheme}, UI{})
}

// Each glyph carries its own destination, and a row with only one of them leaves the other
// column empty rather than sliding into it: the two have to line up down the band, or the
// column stops being a column.
func TestActionCell(t *testing.T) {
	rows := linkFleet().Rows
	both := actionCell(rows[0], "cursor")
	if !strings.Contains(both, linkOpen+"https://api.localhost"+st) {
		t.Errorf("preview glyph is not a link to the preview: %q", both)
	}
	if !strings.Contains(both, linkOpen+"cursor://file/Users/you/work/repo"+st) {
		t.Errorf("folder glyph is not a link to the chosen editor: %q", both)
	}
	if printed(both) != actionsW {
		t.Errorf("cell with both links printed %d columns, want %d: %q", printed(both), actionsW, both)
	}

	folderOnly := actionCell(rows[1], "cursor")
	if strings.Contains(folderOnly, "https://") {
		t.Errorf("row with no preview got a preview link: %q", folderOnly)
	}
	// The folder sits in the second column, so it lines up with the row above it.
	if !strings.HasPrefix(folderOnly, "  ") {
		t.Errorf("folder glyph did not keep the preview's column: %q", folderOnly)
	}
	if printed(folderOnly) != actionsW {
		t.Errorf("folder-only cell printed %d columns, want %d: %q", printed(folderOnly), actionsW, folderOnly)
	}

	// Nothing to point at is no cell at all, not an empty one — that is what keeps a row
	// with no links byte-identical to the frame before links existed.
	if got := actionCell(rows[2], "cursor"); got != "" {
		t.Errorf("row with neither link got a cell: %q", got)
	}
	if got := actionCell(rows[4], "cursor"); got != "" {
		t.Errorf("todo row got a cell: %q", got)
	}

	// The cell is the last thing on the line, so the absent second glyph is trimmed rather
	// than spaced out: padding there would widen the frame's right edge on a row that has
	// nothing in the column it widened for (§9.29).
	previewOnly := actionCell(rows[3], "cursor")
	if printed(previewOnly) != 1 {
		t.Errorf("preview-only cell printed %d columns, want 1: %q", printed(previewOnly), previewOnly)
	}
	if strings.HasSuffix(previewOnly, " ") {
		t.Errorf("preview-only cell was padded rather than trimmed: %q", previewOnly)
	}
}

// The whole point of the derived fields living on Row: a fleet with nothing to point at
// renders the frame it always did, escape for escape.
func TestFleetWithNoLinksIsUnchanged(t *testing.T) {
	bare := linkFleet()
	for i := range bare.Rows {
		bare.Rows[i].Folder, bare.Rows[i].Preview = "", ""
	}
	if got := frameOf(bare, 118); strings.Contains(got, linkOpen) {
		t.Error("a fleet with no links rendered a hyperlink")
	}
	// And the cell is spent out of the surplus, never out of the label: the label is what
	// the reader came for, and a column that narrows to make room for a glyph is the wrong
	// trade (§9.29).
	linked, _, _ := columns(linkFleet(), 118, true)
	unlinked, _, _ := columns(bare, 118, true)
	if linked != unlinked {
		t.Errorf("links narrowed the label column: %d with, %d without", linked, unlinked)
	}
}

// The frame fits in both directions, links or no links. This is the hard rule, and the
// cell is a new thing on the right-hand end of every linked row (§6, §9.12).
func TestLinkedFrameNeverWraps(t *testing.T) {
	f := linkFleet()
	for cols := 40; cols <= 200; cols++ {
		for _, l := range strings.Split(frameOf(f, cols), "\n") {
			if w := printed(l); w > cols {
				t.Fatalf("at %d cols a line printed %d: %q", cols, w, l)
			}
		}
	}
}

// Shed whole when the row cannot hold it, like the KPI strip's cells: half a link cell is
// a glyph that no longer lines up with the one above it, which reads as a rendering fault
// rather than as an absent link.
func TestActionCellIsShedWhenNarrow(t *testing.T) {
	f := linkFleet()
	if got := actionCols(f.Rows, 118, true); got != actionsGap+actionsW {
		t.Errorf("at 118 cols the cell costs %d, want %d", got, actionsGap+actionsW)
	}
	if got := actionCols(f.Rows, 44, true); got != 0 {
		t.Errorf("at 44 cols the cell costs %d, want it shed", got)
	}
	// Shed means shed: the glyphs go with the column they lived in.
	if strings.Contains(frameOf(f, 44), linkOpen) {
		t.Error("a terminal too narrow for the cell still drew a link")
	}
	// And a fleet with nothing to point at never reserves it, at any width.
	bare := linkFleet()
	for i := range bare.Rows {
		bare.Rows[i].Folder, bare.Rows[i].Preview = "", ""
	}
	if got := actionCols(bare.Rows, 118, true); got != 0 {
		t.Errorf("a fleet with no links reserved %d columns", got)
	}
}

// The label is what the reader came for, so the cell is spent out of the surplus and never
// out of the label's floor.
func TestActionCellNeverEatsTheLabelFloor(t *testing.T) {
	f := linkFleet()
	for cols := 40; cols <= 200; cols++ {
		labelW, _, _ := columns(f, cols, true)
		if labelW < minLabelW {
			t.Fatalf("at %d cols the label fell to %d, below the floor %d", cols, labelW, minLabelW)
		}
	}
}

// The arithmetic and the renderer answer the same question through two different
// predicates — actionCols asks pointsSomewhere, the row asks actionCell — so they are
// pinned to the same answer here. Disagreeing either reserves a column nothing fills or
// draws a glyph nothing reserved room for, and the second one wraps the frame.
func TestReservationAgreesWithWhatIsDrawn(t *testing.T) {
	f := linkFleet()
	for _, scheme := range []string{"vscode", "cursor", ""} {
		folders := scheme != ""
		drawn := false
		for _, r := range f.Rows {
			if actionCell(r, scheme) != "" {
				drawn = true
			}
		}
		reserved := actionCols(f.Rows, 118, folders) > 0
		if reserved != drawn {
			t.Errorf("scheme %q: reserved=%v but drawn=%v", scheme, reserved, drawn)
		}
	}
}

// No editor found means no folder link and no column reserved for one: a glyph that opens
// nothing is worse than an absent glyph, and the preview half is unaffected (§18).
func TestNoEditorDropsTheFolderGlyph(t *testing.T) {
	f := linkFleet()
	got := frameWith(f, 118, "")
	if strings.Contains(got, folderGlyph) {
		t.Error("a machine with no editor still drew the folder glyph")
	}
	// The row that has a preview keeps it, and its column is still reserved.
	if !strings.Contains(got, "https://api.localhost") {
		t.Error("dropping the folder link took the preview with it")
	}
	// Rows whose only link was the folder now render exactly as an unlinked row.
	folderOnly := f.Rows[1]
	if cell := actionCell(folderOnly, ""); cell != "" {
		t.Errorf("folder-only row still rendered a cell with no editor: %q", cell)
	}
}

// The scheme is whatever `internal/editor` chose, and the URL shape is the same for all
// three: `<scheme>://file` then the absolute path. Zed's own open_listener strips exactly
// that prefix, VS Code documents it, and Cursor is a VS Code fork.
func TestEditorURLPerScheme(t *testing.T) {
	cases := map[string]string{
		"vscode": "vscode://file/Users/you/work/repo",
		"cursor": "cursor://file/Users/you/work/repo",
		"zed":    "zed://file/Users/you/work/repo",
		"":       "",
	}
	for scheme, want := range cases {
		if got := editorURL(scheme, "/Users/you/work/repo"); got != want {
			t.Errorf("editorURL(%q) = %q, want %q", scheme, got, want)
		}
	}
	// A space in a path is ordinary on macOS and an unescaped one truncates the URL.
	if got := editorURL("zed", "/Users/you/my repo"); got != "zed://file/Users/you/my%20repo" {
		t.Errorf("editorURL did not escape a space: %q", got)
	}
	if got := editorURL("vscode", ""); got != "" {
		t.Errorf("editorURL with no folder = %q, want empty", got)
	}
}
