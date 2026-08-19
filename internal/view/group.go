package view

import (
	"fmt"
	"math"

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
func rail(colour string) string {
	c := groupColour(colour)
	if c == "" {
		return " "
	}
	return fg(c, railGlyph)
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
	name := cut(g.Name, max(0, width-headMargin-groupLead))
	c := groupColour(g.Colour)
	mark, painted := " ", underline(fg(inkPrimary, name))
	if c != "" {
		mark, painted = fg(c, railGlyph), fg(c, name)
	}
	return mark + "  " + painted + " " + dim(fmt.Sprintf("· %d", len(g.Rows))) + "\n"
}
