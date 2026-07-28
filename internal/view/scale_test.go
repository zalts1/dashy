package view

import (
	"strings"
	"testing"
	"time"
)

func TestIdleScale(t *testing.T) {
	t.Run("monotonic and bounded", func(t *testing.T) {
		var lastFrac float64
		var lastStep int
		for _, d := range []time.Duration{
			-time.Hour, 0, time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour,
			72 * time.Hour, 168 * time.Hour, 52 * 24 * time.Hour,
		} {
			frac, step := idleScale(d)
			if frac < 0 || frac > 1 {
				t.Fatalf("%v: frac = %v, out of range", d, frac)
			}
			if step < 0 || step >= len(idleRamp) {
				t.Fatalf("%v: step = %d, out of range", d, step)
			}
			if frac < lastFrac || step < lastStep {
				t.Fatalf("%v went backwards: frac %v→%v step %d→%d", d, lastFrac, frac, lastStep, step)
			}
			lastFrac, lastStep = frac, step
		}
	})

	t.Run("negative reads as fresh", func(t *testing.T) {
		// A clock in the future must not wrap round to the top of the ramp.
		if frac, step := idleScale(-time.Hour); frac != 0 || step != 0 {
			t.Errorf("negative idle = %v/%d, want 0/0", frac, step)
		}
	})

	t.Run("the scale is absolute, and saturates at its top", func(t *testing.T) {
		// Absolute so bars stay comparable between refreshes; anything past a week
		// pins to the end rather than rescaling the whole board.
		week, wStep := idleScale(168 * time.Hour)
		year, yStep := idleScale(365 * 24 * time.Hour)
		if week != 1 || year != 1 || wStep != yStep {
			t.Errorf("saturation: 7d = %v/%d, 1y = %v/%d", week, wStep, year, yStep)
		}
	})

	t.Run("log, not linear", func(t *testing.T) {
		// Linear would flatten everything under a day into the first cell. An hour
		// must already be visibly off the floor.
		if frac, _ := idleScale(time.Hour); frac < 0.1 {
			t.Errorf("1h maps to %v of the scale; too compressed to read", frac)
		}
	})
}

func TestBar(t *testing.T) {
	for _, d := range []time.Duration{0, time.Minute, 5 * time.Hour, 200 * 24 * time.Hour} {
		got := bar(d)
		// Every bar occupies the same cells, or the IDLE column stops lining up.
		if n := visibleWidth(got); n != barCells {
			t.Errorf("bar(%v) occupies %d cells, want %d", d, n, barCells)
		}
		if !strings.Contains(got, "▇") {
			t.Errorf("bar(%v) has no filled cell; a zero-length bar reads as missing data", d)
		}
	}
	if filled(bar(0)) >= filled(bar(50*time.Hour)) {
		t.Error("bar length does not grow with idle time")
	}
	if filled(bar(168*time.Hour)) != barCells {
		t.Errorf("a week fills %d of %d cells, want full", filled(bar(168*time.Hour)), barCells)
	}
}

// The ramp must brighten with age: on a dark surface the sequential anchor flips, so
// the documented dark floor fails the ordinal contrast gate. See DESIGN.md §6 Rendering.
func TestIdleRampBrightensWithAge(t *testing.T) {
	last := -1.0
	for i, hex := range idleRamp {
		l := luminance(hex)
		if l <= last {
			t.Fatalf("ramp step %d (%s) is not brighter than the one before it (%.3f vs %.3f)",
				i, hex, l, last)
		}
		last = l
	}
}

func TestScaleLegendCoversTheRamp(t *testing.T) {
	legend := scaleLegend()
	for _, mark := range []string{"1h", "6h", "1d", "3d", "7d"} {
		if !strings.Contains(legend, mark) {
			t.Errorf("legend is missing the %s mark", mark)
		}
	}
}

// visibleWidth counts printable runes, ignoring the SGR escape sequences.
func visibleWidth(s string) int {
	n, esc := 0, false
	for _, r := range s {
		switch {
		case r == '\033':
			esc = true
		case esc:
			if r == 'm' {
				esc = false
			}
		default:
			n++
		}
	}
	return n
}

func filled(s string) int { return strings.Count(s, "▇") }

// luminance is the relative luminance used by the contrast checks.
func luminance(hex string) float64 {
	r, g, b := rgb(hex)
	lin := func(c int) float64 {
		v := float64(c) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		// Approximation is fine: this test asserts ordering, not exact ratios.
		return ((v + 0.055) / 1.055) * ((v + 0.055) / 1.055)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
