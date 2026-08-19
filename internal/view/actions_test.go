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
			{Key: "K-1", State: "running", Label: "build the export endpoint", Repo: "API",
				Surface: "S-1", Rank: board.RankWorking,
				Folder: "/Users/you/work/repo", Preview: "https://api.localhost"},
			{Key: "K-2", State: "done", Label: "migrate auth handlers", Repo: "AUTH",
				Surface: "S-2", Idle: 30 * time.Minute, Rank: board.RankQuiet,
				Folder: "/Users/you/work/repo/.claude/worktrees/auth-v2"},
			{Key: "K-5", State: "done", Label: "all four links", Repo: "UI",
				Surface: "S-5", Idle: 12 * time.Minute, Rank: board.RankQuiet,
				Folder:    "/Users/you/work/repo",
				Preview:   "https://ui.localhost",
				Storybook: "http://localhost:6006",
				PR:        "https://github.com/you/repo/pull/7",
				PRState:   "open"},
			{Key: "K-6", State: "done", Label: "a pull request and nothing else", Repo: "OPS",
				Surface: "S-6", Idle: 8 * time.Minute, Rank: board.RankQuiet,
				PR: "https://github.com/you/repo/pull/9", PRState: "merged"},
			{Key: "K-3", State: "done", Label: "no directory at all", Repo: "OPS",
				Surface: "S-3", Idle: 90 * time.Minute, Rank: board.RankQuiet, Stale: true},
			{Key: "K-4", State: "done", Label: "a preview and no folder", Repo: "WEB",
				Surface: "S-4", Idle: 20 * time.Minute, Rank: board.RankQuiet,
				Preview: "https://web.localhost"},
			{Key: "todo:t1", State: "todo", Label: "book the quarterly review",
				Idle: 26 * time.Hour, Rank: board.RankTodo},
		},
		Stale: 1, Workspaces: 6, Oldest: 90 * time.Minute, TodoCap: 10,
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
// byKey keeps these assertions readable as the fixture grows: a row's index is not a fact
// worth encoding in six places.
func byKey(t *testing.T, key string) board.Row {
	t.Helper()
	for _, r := range linkFleet().Rows {
		if r.Key == key {
			return r
		}
	}
	t.Fatalf("no row %q in the fixture", key)
	return board.Row{}
}

// slotEnd is the printed width of a cell whose last occupied slot is the nth of four: the glyphs
// up to it, plus the gaps between them. Derived rather than written out, so these assertions follow
// actionsSpace instead of having to be found and edited when it changes (§9.44).
func slotEnd(n int) int { return n + (n-1)*actionsSpace }

func TestActionCell(t *testing.T) {
	both := actionCell(byKey(t, "K-1"), "cursor")
	if !strings.Contains(both, linkOpen+"https://api.localhost"+st) {
		t.Errorf("preview glyph is not a link to the preview: %q", both)
	}
	if !strings.Contains(both, linkOpen+"cursor://file/Users/you/work/repo"+st) {
		t.Errorf("folder glyph is not a link to the chosen editor: %q", both)
	}
	// Five, not actionsW: the slots are fixed, so an absent *leading* glyph holds its column and an
	// absent *trailing* one is trimmed. This row has the middle two, so it keeps the Storybook's
	// blank on the left and loses the PR's on the right.
	if printed(both) != slotEnd(3) {
		t.Errorf("cell with preview+folder printed %d columns, want %d: %q",
			printed(both), slotEnd(3), both)
	}

	folderOnly := actionCell(byKey(t, "K-2"), "cursor")
	if strings.Contains(folderOnly, "https://") {
		t.Errorf("row with no preview got a preview link: %q", folderOnly)
	}
	// The folder sits in the second column, so it lines up with the row above it.
	if !strings.HasPrefix(folderOnly, "  ") {
		t.Errorf("folder glyph did not keep the preview's column: %q", folderOnly)
	}
	if printed(folderOnly) != slotEnd(3) {
		t.Errorf("folder-only cell printed %d columns, want %d: %q",
			printed(folderOnly), slotEnd(3), folderOnly)
	}

	// Nothing to point at is no cell at all, not an empty one — that is what keeps a row
	// with no links byte-identical to the frame before links existed.
	if got := actionCell(byKey(t, "K-3"), "cursor"); got != "" {
		t.Errorf("row with neither link got a cell: %q", got)
	}
	if got := actionCell(byKey(t, "todo:t1"), "cursor"); got != "" {
		t.Errorf("todo row got a cell: %q", got)
	}

	// The cell is the last thing on the line, so the absent second glyph is trimmed rather
	// than spaced out: padding there would widen the frame's right edge on a row that has
	// nothing in the column it widened for (§9.29).
	previewOnly := actionCell(byKey(t, "K-4"), "cursor")
	// All four, for the row that has them: the cell is exactly its width, no more.
	if all := actionCell(byKey(t, "K-5"), "cursor"); printed(all) != actionsW {
		t.Errorf("cell with all four printed %d columns, want %d: %q", printed(all), actionsW, all)
	} else {
		for _, g := range []string{storybookGlyph, previewGlyph, folderGlyph, prGlyph} {
			if !strings.Contains(all, g) {
				t.Errorf("cell with all four is missing %q: %q", g, all)
			}
		}
		// The pull request is the rightmost thing on the row, because it is the only one that
		// does not point at this machine (§18).
		if strings.Index(all, prGlyph) < strings.Index(all, folderGlyph) {
			t.Errorf("the PR glyph is not right of the folder: %q", all)
		}
	}
	// A pull request alone still fills the whole cell, because its column is the last one: the
	// three blanks in front of it are what keep it under the same header as everybody else's.
	prOnly := actionCell(byKey(t, "K-6"), "cursor")
	if printed(prOnly) != actionsW {
		t.Errorf("PR-only cell printed %d columns, want %d: %q", printed(prOnly), actionsW, prOnly)
	}
	if !strings.HasPrefix(prOnly, strings.Repeat(" ", slotEnd(3)+actionsSpace)) {
		t.Errorf("PR-only cell did not keep the three columns before it: %q", prOnly)
	}
	// The cell is the last thing on the line, so absent *trailing* glyphs are trimmed rather
	// than spaced out: padding there would widen the frame's right edge on a row that has
	// nothing in the column it widened for (§9.29). Absent *leading* ones still hold their
	// column, which is what keeps the glyphs lined up down a band.
	//
	// The order is rarest first — Storybook, preview, folder — so a preview-only row keeps one
	// leading blank and trims the two trailing ones.
	if printed(previewOnly) != slotEnd(2) {
		t.Errorf("preview-only cell printed %d columns, want %d: %q",
			printed(previewOnly), slotEnd(2), previewOnly)
	}
	if !strings.HasPrefix(previewOnly, strings.Repeat(" ", 1+actionsSpace)) {
		t.Errorf("preview-only cell did not keep the Storybook's column: %q", previewOnly)
	}
	if strings.HasSuffix(previewOnly, " ") {
		t.Errorf("preview-only cell was padded rather than trimmed: %q", previewOnly)
	}
	// The leftmost slot alone is the smallest the cell gets: everything after it trims away.
	storybookOnly := actionCell(board.Row{Storybook: "http://localhost:6006"}, "cursor")
	if printed(storybookOnly) != slotEnd(1) {
		t.Errorf("storybook-only cell printed %d columns, want 1: %q",
			printed(storybookOnly), storybookOnly)
	}
	if !strings.Contains(storybookOnly, storybookGlyph) {
		t.Errorf("storybook-only cell is not the Storybook glyph: %q", storybookOnly)
	}
}

// The whole point of the derived fields living on Row: a fleet with nothing to point at
// renders the frame it always did, escape for escape.
func TestFleetWithNoLinksIsUnchanged(t *testing.T) {
	bare := linkFleet()
	for i := range bare.Rows {
		bare.Rows[i].Folder, bare.Rows[i].Preview = "", ""
		bare.Rows[i].Storybook, bare.Rows[i].PR = "", ""
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
		bare.Rows[i].Storybook, bare.Rows[i].PR = "", ""
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

// cmux tells board which state the pull request is in, and the three want different things from
// the reader: an open one is something to go and look at, a merged one is context, a closed one is
// a branch somebody abandoned. Shape carries the first distinction and colour the second, so
// neither has to be read alone (§18).
func TestPRMarkByState(t *testing.T) {
	cases := map[string]struct{ glyph, colour string }{
		"open":   {prGlyph, linkPR},
		"merged": {prMergedGlyph, linkPR},
		"closed": {prGlyph, linkPRClosed},
		// A state cmux has not had yet draws as open, because a glyph board cannot classify is
		// still a pull request worth reaching.
		"":         {prGlyph, linkPR},
		"reopened": {prGlyph, linkPR},
	}
	for state, want := range cases {
		got := prMark(state)
		if got != fg(want.colour, want.glyph) {
			t.Errorf("prMark(%q) = %q, want %q in %s", state, got, want.glyph, want.colour)
		}
		// Whatever the state, it is one column: the slot's width does not depend on it.
		if printed(got) != 1 {
			t.Errorf("prMark(%q) printed %d columns, want 1", state, printed(got))
		}
	}
	// Merged and closed must not be told apart by colour alone or by shape alone — each pair
	// differs in at least one, so a reader who misses one cue still has the other.
	if prMark("merged") == prMark("closed") {
		t.Error("merged and closed render identically")
	}
	if prMark("open") == prMark("merged") || prMark("open") == prMark("closed") {
		t.Error("open is indistinguishable from a landed pull request")
	}
}

// The spacing is a fit decision, not a style: the frame takes it when everything fits and drops it
// when it does not, because the rows are the information and the gaps are not (§9.44).
func TestAiryOnlyWhenItFits(t *testing.T) {
	f := linkFleet()
	frame := func(lines int) string {
		return Frame(f, Screen{Now: time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC),
			Interval: 10 * time.Second, Threshold: 45 * time.Minute, Rows: lines, Cols: 118,
			EditorScheme: "cursor"}, UI{})
	}
	// A tall tab: a blank line between rows, so two row lines never sit adjacent.
	tall := frame(120)
	if !strings.Contains(tall, "\n\n   "+dim(quietGlyph)) {
		t.Errorf("a tall tab did not space the quiet rows out:\n%s", tall)
	}
	// A tab exactly one line too short for the airy form falls back to compact rather than
	// shedding a row to stay airy.
	airyHeight := strings.Count(tall, "\n") + 2
	tight := frame(airyHeight - 1)
	if strings.Contains(tight, "\n\n   "+dim(quietGlyph)) {
		t.Errorf("a tab too short for the airy form still spaced rows out:\n%s", tight)
	}
	if got, want := strings.Count(tight, quietGlyph), strings.Count(tall, quietGlyph); got != want {
		t.Errorf("falling back to compact lost rows: %d vs %d", got, want)
	}
}

// Spacing may never be the reason a row is missing. At every height where the compact frame would
// show the whole fleet, the frame board actually renders shows the whole fleet too — whichever form
// it chose.
func TestAiryNeverCostsARow(t *testing.T) {
	f := linkFleet()
	_, _, todo, quiet := f.Bands()
	for lines := 12; lines <= 60; lines++ {
		s := Screen{Now: time.Date(2026, 8, 19, 14, 30, 0, 0, time.UTC),
			Interval: 10 * time.Second, Threshold: 45 * time.Minute, Rows: lines, Cols: 118,
			EditorScheme: "cursor"}
		got := Frame(f, s, UI{})
		if h := strings.Count(got, "\n") + 2; h > lines {
			t.Fatalf("at %d lines the frame is %d", lines, h)
		}
		// Would a compact frame with every row have fitted? Then nothing may be missing.
		compactAll := compose(f, s, UI{}, pick(quiet, len(quiet), ""), pick(todo, len(todo), ""), false)
		if height(compactAll) > lines {
			continue
		}
		if got, want := strings.Count(got, quietGlyph), len(quiet); got != want {
			t.Errorf("at %d lines the frame shows %d quiet rows, want all %d", lines, got, want)
		}
	}
}
