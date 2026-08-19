package view

import (
	"math"
	"strings"
	"testing"
)

// §6 says colour is validated and never eyeballed, and until now that was a rule enforced
// by whoever remembered it. This file enforces it: every value in the palette is measured
// against both documented backgrounds, so an eyeballed substitution fails the suite rather
// than shipping and looking fine to the person who chose it (§9.4).
// The backgrounds and the floor live in palette.go: they are facts about the design, and
// groupColour has to lift against them at runtime, not only in a test.
const (
	epsilon = 0.01
	// rampStep is what §6 actually claims for the idle ramp, and it is a different claim:
	// the ramp is **ordinal**, so what matters is that adjacent rungs are distinguishable
	// from each other, not that each clears a text-contrast bar against the background.
	// The source ramp measured ΔL 0.049 between adjacent rungs, under this minimum, which
	// is why palette.go skips every other one.
	rampStep = 0.06
)

func TestPaletteContrast(t *testing.T) {
	// linkAbsent is absent deliberately too, and asserted the other way round below: it is a
	// placeholder whose whole job is to be sub-legible, so holding it to the floor would break the
	// thing it is for (§9.45).
	//
	// statusCritical is absent deliberately: bare #d03b3b measures 2.91 and is the finding
	// §9.4 records, which is why it may only ever appear as a filled badge with white text.
	// A test that let it through here would be asserting the opposite of the finding.
	values := map[string]string{
		"inkPrimary":    inkPrimary,
		"inkSecondary":  inkSecondary,
		"inkMuted":      inkMuted,
		"statusGood":    statusGood,
		"statusWarning": statusWarning,
		"linkPreview":   linkPreview,
		"linkFolder":    linkFolder,
		"linkStorybook": linkStorybook,
		"linkPR":        linkPR,
		"linkPRClosed":  linkPRClosed,
	}
	for name, hex := range values {
		for _, bg := range []string{bgLightest, bgDarkest} {
			if r := contrast(hex, bg); r < inkFloor-epsilon {
				t.Errorf("%s (%s) measures %.2f against %s, below the ink floor %.2f",
					name, hex, r, bg, inkFloor)
			}
		}
	}
}

// The idle ramp is held to the claim §6 makes about it, which is **ordinality** and not
// absolute contrast: dim means fresh and bright means rotting, so what has to hold is that
// adjacent rungs are told apart from each other.
//
// Its low rungs are deliberately below the ink floor and are not a defect to fix — measured,
// #256abf is 2.59 against #282c34 and 3.80 against #040404. That is the encoding working: a
// bar you can barely see is a session nobody needs to look at. Written down here because the
// next person to run a contrast check over this file will find those numbers and,
// reasonably, think something is broken (§9.36).
func TestIdleRampIsOrdinal(t *testing.T) {
	for i := 1; i < len(idleRamp); i++ {
		step := luminance(idleRamp[i]) - luminance(idleRamp[i-1])
		if step < rampStep {
			t.Errorf("ramp rungs %d→%d differ by ΔL %.3f, under the %.2f minimum",
				i-1, i, step, rampStep)
		}
	}
	// The placeholder, asserted as staying *under* the floor. Written as a test rather than a
	// comment because the obvious "fix" on reading the contrast table is to raise it, and raising it
	// is what would make it compete with the marks that mean something (§9.45).
	for _, bg := range []string{bgLightest, bgDarkest} {
		if r := contrast(linkAbsent, bg); r >= inkFloor {
			t.Errorf("linkAbsent measures %.2f against %s — at or above the floor %.2f, so it is "+
				"legible enough to compete with the real glyphs", r, bg, inkFloor)
		}
	}

	// And the badge's own guarantee, which is what makes it theme-independent: white on its
	// fill, measured against the fill rather than against the terminal.
	if r := contrast(inkPrimary, statusCritical); r < 4.5 {
		t.Errorf("the blocked badge measures %.2f on its own fill, want ≥4.5", r)
	}
}

// The link glyphs are a set and have to read as one: matched in measured contrast so none
// dominates the others in one cell. Chosen that way rather than by eye (DESIGN.md §18), so
// the match is the thing worth pinning — and the pink was the value that made this hard, so
// it is the one this test is really guarding (§9.36).
func TestLinkGlyphColoursAreMatched(t *testing.T) {
	// The four *slots*, which is the rule: these are the colours the eye compares side by side in
	// one cell, and a mark heavier than its neighbours reads as the only one that matters.
	//
	// linkPRClosed is deliberately not here. Open, merged and closed are alternatives — exactly
	// one is ever on a row — so nothing compares them to each other, and holding a red to this
	// band would make it a pale salmon nobody reads as red (§9.43). It is held to the floor by
	// TestPaletteContrast instead.
	set := map[string]string{
		"linkPreview": linkPreview, "linkFolder": linkFolder,
		"linkStorybook": linkStorybook, "linkPR": linkPR,
	}
	for _, bg := range []string{bgLightest, bgDarkest} {
		for an, a := range set {
			for bn, b := range set {
				if d := math.Abs(contrast(a, bg) - contrast(b, bg)); d > 0.25 {
					t.Errorf("against %s, %s and %s are %.2f apart, want within 0.25", bg, an, bn, d)
				}
			}
		}
	}
	// Distinct from statusGood, or "running" and "a preview is up" become one colour.
	if linkPreview == statusGood {
		t.Error("the preview glyph reuses the running colour; green would mean two things")
	}
}

// A swapped pair is the mistake a golden file catches only if somebody reads the escape
// codes, so it is asserted directly: cyan is the folder, green is the preview.
func TestEachGlyphGetsItsOwnColour(t *testing.T) {
	r := byKey(t, "K-5") // has all three links
	cell := actionCell(r, "cursor")
	for name, want := range map[string]string{
		"preview":   fg(linkPreview, previewGlyph),
		"storybook": fg(linkStorybook, storybookGlyph),
		"folder":    fg(linkFolder, folderGlyph),
		"pr":        fg(linkPR, prGlyph), // K-5's is open, so the hollow mark
	} {
		if !strings.Contains(cell, want) {
			t.Errorf("the %s glyph is not painted its own colour:\n%q", name, cell)
		}
	}
}

// contrast is the WCAG relative-luminance ratio, which is the measurement §6's table was
// built from. It lives here rather than in scale_test.go, which used a squared approximation
// because its own checks only need ordering — the floor asserted above needs the real
// exponent, and one exact formula serves both (§9.36).
func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	hi, lo := math.Max(la, lb), math.Min(la, lb)
	return (hi + 0.05) / (lo + 0.05)
}

func luminance(hex string) float64 {
	r, g, b := rgb(hex)
	lin := func(c int) float64 {
		v := float64(c) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
