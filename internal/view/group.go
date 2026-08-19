package view

import (
	"fmt"
	"math"
	"strings"

	"github.com/zalts1/dashy/internal/board"
)

// railGlyph marks which workspace a row belongs to. It sits in the frame's leftmost column —
// the one the lead was already leaving blank — so grouping costs no width at all, which is what
// makes it affordable on a frame that has to fit in both directions (§6).
//
// A left half block rather than a line-drawing character: it is a colour field and not a border,
// the same thing cmux draws down the side of its own sidebar, so the two surfaces read alike.
const railGlyph = "▌"

// groupLead is the rail plus the two columns after it: where a group's name starts.
const groupLead = 3

// groupIndent is how far a grouped row sits inside its header. Without it a grouped row's state
// mark sat in the same column as an ungrouped one's, so the two read as the same level and the
// header looked like it applied to the whole band rather than to the two rows under it.
//
// **The indent comes out of the label's own width, not off the front of the row**, so every
// column to the right of the label — bar, IDLE, WHERE, RELATED — stays exactly where it was. A
// nest that shifted the whole row would break the width invariant §9.12 is about.
const groupIndent = 4

// ruleGlyph draws the two rules: one under the column header, anchoring it to the table, and one
// after a group's name, closing the block off to the right. Both are inkAbsent's weight, because
// a rule is structure and must not compete with anything that is information (§9.45).
const ruleGlyph = "─"

// groupColour is the user's workspace colour, lifted until it is legible.
//
// **This is the answer to a real tension.** §6 says colour is validated and never eyeballed, and
// every other value in palette.go was measured by hand before it shipped. A group's colour cannot
// be: it is whatever the user picked in cmux, and board sees it for the first time at runtime.
// The way to keep the promise anyway is to validate the *function* — every colour board draws is
// this one's output, so a test that holds this to the floor holds every group to it (§9.48).
//
// Lifting is necessary and not cosmetic. cmux's palette is built for a filled sidebar rail on the
// app's own background, so its values are dark: the three on the fleet this was built against
// measure 2.52, 2.21 and 1.48 against #282c34, and the darkest is half the bare red §9.4 already
// rejected as unreadable. Drawn as given, a group name would have been a smudge.
//
// Lightness moves and hue does not, because hue is the whole signal: the rail exists so two
// workspaces can be told apart at a glance, and a lift that slid magenta towards pink would put
// it on top of the storybook glyph's colour instead.
func groupColour(hex string) string {
	if hex == "" {
		return "" // a workspace with no colour is not a colour, and must not become one
	}
	h, s, l := toHSL(rgb(hex))
	for range 101 {
		cand := fromHSL(h, s, l)
		if ratio(cand, bgLightest) >= inkFloor && ratio(cand, bgDarkest) >= inkFloor {
			return cand
		}
		if l >= 1 {
			break
		}
		l = math.Min(1, l+0.01)
	}
	// White clears both backgrounds by a wide margin, so this is unreachable in practice. It is
	// here so the function has no path that returns something it has not checked.
	return inkPrimary
}

// ratio is the WCAG contrast between two colours. palette_test.go computes the same thing
// independently and deliberately: the test is the check on this, so sharing an implementation
// would let one mistake satisfy both.
func ratio(a, b string) float64 {
	la, lb := relLuminance(a), relLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relLuminance(hex string) float64 {
	r, g, b := rgb(hex)
	f := func(v int) float64 {
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

func toHSL(r, g, b int) (h, s, l float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	mx := math.Max(rf, math.Max(gf, bf))
	mn := math.Min(rf, math.Min(gf, bf))
	l = (mx + mn) / 2
	d := mx - mn
	if d == 0 {
		return 0, 0, l
	}
	s = d / (1 - math.Abs(2*l-1))
	switch mx {
	case rf:
		h = math.Mod((gf-bf)/d, 6)
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

func fromHSL(h, s, l float64) string {
	f := func(n float64) int {
		k := math.Mod(n+h/30, 12)
		a := s * math.Min(l, 1-l)
		return int(math.Round(255 * (l - a*math.Max(-1, math.Min(math.Min(k-3, 9-k), 1)))))
	}
	return fmt.Sprintf("#%02X%02X%02X", f(0), f(8), f(4))
}

// rail is a row's group mark: one column at the frame's edge, coloured for the workspace this
// session belongs to, and blank when the workspace has no colour.
//
// Blank and not white. A rail on every row would be ink spent on no information — most
// workspaces have no accent, and a mark that is always there marks nothing (§9.13).
func rail(colour string, inGroup bool) string {
	if c := groupColour(colour); c != "" {
		return fg(c, railGlyph)
	}
	// A group the user never coloured still has to hold its rows together, so it rails in the
	// faint weight. A row belonging to no group draws nothing: a mark on every row marks nothing.
	if inGroup {
		return fg(linkAbsent, railGlyph)
	}
	return " "
}

// groupHead is the line above a workspace holding more than one session.
//
// It sits one column inside the band header — band at 2, group at 3, label at 7 — so the three
// read as the nesting they are. **Not** at the label column, which was tried first: the gutter is
// elastic (§9.41), so a blocked row anywhere on the fleet widens it to twelve and drags every
// group name out to meet the labels, leaving the name stranded nine columns from the rail that
// owns it. Aligning to a fixed lead keeps the name against its rail whatever the gutter is doing.
//
// The name is cut for the reason every cell in this file is: a line wider than the terminal wraps,
// and a wrapped line makes the frame occupy a screen row the height measurement did not budget
// for (§6, EVIDENCE.md §9.10).
func groupHead(g board.Group, width int) string {
	count := fmt.Sprintf("· %d", len(g.Rows))
	name := cut(g.Name, max(0, width-headMargin-groupLead-runes(count)-2))
	c := groupColour(g.Colour)
	mark, painted := fg(linkAbsent, railGlyph), underline(fg(inkPrimary, name))
	if c != "" {
		mark, painted = fg(c, railGlyph), fg(c, name)
	}
	head := mark + "  " + painted + " " + dim(count)
	// The rule runs from the count to the right margin, so the group reads as a bounded block
	// rather than a name floating above some rows.
	if fill := width - headMargin - groupLead - runes(name) - runes(count) - 2; fill > 0 {
		head += " " + fg(linkAbsent, strings.Repeat(ruleGlyph, fill))
	}
	return head + "\n"
}

// columnRule underlines the column header. One line, and it is what turns four words floating
// above a table into a header the table hangs from.
func columnRule(width int) string {
	return strings.Repeat(" ", headMargin) +
		fg(linkAbsent, strings.Repeat(ruleGlyph, max(0, width-2*headMargin))) + "\n"
}
