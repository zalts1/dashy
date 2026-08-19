package view

import (
	"regexp"
	"strings"
	"testing"
)

// The column header names all four areas of a row. `LABEL` was implementation vocabulary — the
// column is the session — and `REPO` sat over a cell that routinely shows `repo -> worktree`,
// which is a location. The link cell had no name at all (§19).
func TestColumnHeaderNamesEveryColumn(t *testing.T) {
	out := bare(Frame(goldenLinkFleet(), linkScreen(), UI{}))
	for _, want := range []string{"SESSION", "IDLE", "WHERE", "RELATED"} {
		if !strings.Contains(out, want) {
			t.Errorf("column header does not name %q", want)
		}
	}
	for _, gone := range []string{"LABEL", "REPO"} {
		if strings.Contains(out, gone) {
			t.Errorf("column header still says %q", gone)
		}
	}
}

// RELATED sits over the link cell it names, not merely somewhere on the line. A header naming a
// column it is not above is worse than no header.
func TestRelatedSitsOverTheLinkCell(t *testing.T) {
	out := bare(Frame(goldenLinkFleet(), linkScreen(), UI{}))
	var header, row string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "RELATED") {
			header = line
		}
		if strings.Contains(line, "merge app#1497") {
			row = line
		}
	}
	if header == "" || row == "" {
		t.Fatalf("missing header (%q) or row (%q)", header, row)
	}
	at := func(s, sub string) int { return len([]rune(s[:strings.Index(s, sub)])) }
	if h, g := at(header, "RELATED"), at(row, "⧆"); h != g {
		t.Errorf("RELATED starts at column %d, the link cell at %d", h, g)
	}
}

// A rule under the column header anchors it to the table below (variant C).
func TestColumnHeaderHasARuleUnderIt(t *testing.T) {
	lines := strings.Split(plain(Frame(grouped(), screen(44, 130), UI{})), "\n")
	for i, line := range lines {
		if strings.Contains(line, "SESSION") {
			if i+1 >= len(lines) || !strings.Contains(lines[i+1], ruleGlyph) {
				t.Fatalf("no rule under the column header; next line was %q", lines[i+1])
			}
			return
		}
	}
	t.Fatal("no column header found")
}

// A group's name is closed off to the right, so the block it opens is bounded.
func TestGroupHeaderCarriesARule(t *testing.T) {
	for _, line := range strings.Split(plain(Frame(grouped(), screen(44, 130), UI{})), "\n") {
		if strings.Contains(line, "Payments rework") {
			if !strings.Contains(line, ruleGlyph) {
				t.Errorf("group header has no rule: %q", line)
			}
			return
		}
	}
	t.Fatal("no group header found")
}

// The nesting complaint this fixes: a grouped row's state mark sat at the same column as an
// ungrouped one's, so the two read as the same level. Grouped rows indent under their header.
func TestGroupedRowsIndentUnderTheirHeader(t *testing.T) {
	out := plain(Frame(grouped(), screen(44, 130), UI{}))
	grpRow := lineWith(t, out, "backfill refunds") // in a group of two
	soloRow := lineWith(t, out, "wizard copy")     // alone in its workspace
	g, s := runeIndex(grpRow, "○"), runeIndex(soloRow, "○")
	if g != s+groupIndent {
		t.Errorf("grouped mark at column %d, solo at %d: want a %d-column nest", g, s, groupIndent)
	}
}

// The indent must come out of the label's own width, so every column to the right of it stays
// where it was. A nest that shifted the whole row would undo §9.12's width invariant.
func TestTheIndentDoesNotMoveAnyOtherColumn(t *testing.T) {
	out := plain(Frame(grouped(), screen(44, 130), UI{}))
	where := runeIndex(lineWith(t, out, "SESSION"), "WHERE")
	if where < 0 {
		t.Fatal("no WHERE in the column header")
	}
	// Both rows must start their location cell in the column the header names, whatever the
	// indent did to the label to the left of it.
	for _, c := range []struct{ row, loc string }{
		{"backfill refunds", "app"}, {"wizard copy", "date-invite"},
	} {
		line := []rune(lineWith(t, out, c.row))
		if where >= len(line) || !strings.HasPrefix(string(line[where:]), c.loc) {
			t.Errorf("%s: WHERE column at %d does not start %q", c.row, where, c.loc)
		}
	}
}

// The rail runs down every row of a group, including one the user never coloured — otherwise an
// uncoloured group has a header and then nothing holding its rows together.
func TestAnUncolouredGroupStillRailsItsRows(t *testing.T) {
	f := grouped()
	for i := range f.Rows {
		f.Rows[i].GroupColour = ""
	}
	out := plain(Frame(f, screen(44, 130), UI{}))
	if !strings.Contains(lineWith(t, out, "backfill refunds"), railGlyph) {
		t.Error("a row in an uncoloured group has no rail")
	}
	// A row that belongs to no group must not gain one.
	if strings.Contains(lineWith(t, out, "wizard copy"), railGlyph) {
		t.Error("a solo uncoloured row drew a rail; it belongs to no group")
	}
}

// Spacing carries the grouping: rows inside a group are adjacent, groups are separated. Equal
// gaps everywhere is what made the group fail to read as one block.
func TestAGroupIsTightAndGroupsAreSeparated(t *testing.T) {
	lines := strings.Split(plain(Frame(grouped(), screen(120, 130), UI{})), "\n")
	idx := func(sub string) int {
		for i, l := range lines {
			if strings.Contains(l, sub) {
				return i
			}
		}
		t.Fatalf("no line containing %q", sub)
		return -1
	}
	// Two rows of one group: adjacent.
	if a, b := idx("backfill refunds"), idx("webhook retries"); b != a+1 {
		t.Errorf("rows of one group are %d lines apart, want adjacent", b-a)
	}
	// The next group: separated by a blank.
	if a, b := idx("webhook retries"), idx("wizard copy"); b != a+2 {
		t.Errorf("groups are %d lines apart, want one blank between", b-a)
	}
}

// bare strips hyperlinks as well as colour. plain() only removes SGR, and a row with links has
// OSC 8 sequences in it — measuring a column through those measures the escape, not the glyph.
var osc8 = regexp.MustCompile("\x1b\\]8;;[^\x1b\x07]*(\x1b\\\\|\x07)")

func bare(s string) string { return osc8.ReplaceAllString(plain(s), "") }

// linkScreen is a screen wide enough for the link cell to be reserved, with an editor scheme so
// the folder glyph is drawn (§18).
func linkScreen() Screen {
	s := screen(44, 118)
	s.EditorScheme = "vscode"
	return s
}

func lineWith(t *testing.T, out, sub string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	t.Fatalf("no line containing %q", sub)
	return ""
}

func runeIndex(line, sub string) int {
	i := strings.Index(line, sub)
	if i < 0 {
		return -1
	}
	return len([]rune(line[:i]))
}
