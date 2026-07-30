package view

import (
	"math"
	"strings"
	"time"
)

// Bar length and ramp step both encode idle on one shared absolute log scale
// (0→7d). Absolute so bars stay comparable between refreshes; log because the
// range spans minutes to weeks and linear would flatten everything under a day
// into the first cell.
const (
	sevenDays = 168.0 // hours, the top of the scale
	barCells  = 12
)

// idleScale maps a duration onto the scale: a 0..1 fraction plus a ramp index.
func idleScale(d time.Duration) (float64, int) {
	h := d.Hours()
	if h < 0 {
		h = 0
	}
	frac := math.Log1p(h) / math.Log1p(sevenDays)
	if frac > 1 {
		frac = 1
	}
	step := int(frac * float64(len(idleRamp)))
	if step >= len(idleRamp) {
		step = len(idleRamp) - 1
	}
	return frac, step
}

func bar(d time.Duration) string {
	frac, step := idleScale(d)
	n := 1 + int(math.Round(frac*float64(barCells-1)))
	return fg(idleRamp[step], strings.Repeat("▇", n)) + strings.Repeat(" ", barCells-n)
}

// scaleLegend is the key for the ramp. A value scale without a legend is
// decoration; this is what makes bar length and colour readable as a quantity.
//
// The marks are round durations because a key is read, not measured — but which rung
// each one lands on is the log scale's business, and the boundaries are not round:
// they fall at ~1h48m, 6h47m, 20h42m and 2d12h. So the marks are chosen to sit one
// inside each rung, and TestScaleLegendCoversTheRamp holds them there. The obvious
// set (1h/6h/1d/3d/7d) reads better and is wrong: 1d is already past the third
// boundary, so it duplicated 3d's rung and dropped the one rows use for half a day
// (EVIDENCE.md §9.20).
func scaleLegend() string {
	out := ""
	marks := []time.Duration{time.Hour, 3 * time.Hour, 12 * time.Hour, 48 * time.Hour, 168 * time.Hour}
	for i, d := range marks {
		_, step := idleScale(d)
		out += fg(idleRamp[step], "▇") + dim(" "+short(d))
		if i < len(marks)-1 {
			out += dim("  ")
		}
	}
	return out
}
