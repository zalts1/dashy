package view

import (
	"fmt"
	"strings"
)

// Colour comes from the validated data-viz palette. Every value below cleared the
// contrast checks against both the lightest and darkest plausible terminal
// backgrounds (#282c34 and #040404) — see DESIGN.md §6 Rendering.
//
// Do not substitute by eye and do not add a colour without re-validating. Two
// results are load-bearing: bare #d03b3b text fails at 2.91, which is why blocked
// is a filled badge with white text; and the idle ramp brightens with age because
// the sequential anchor flips on a dark surface.
const (
	inkPrimary   = "#ffffff"
	inkSecondary = "#c3c2b7"
	inkMuted     = "#898781"

	statusCritical = "#d03b3b" // blocked — only ever as a filled badge, never bare text
	statusGood     = "#0ca30c" // running
	statusWarning  = "#fab219" // past the idle threshold
)

// idleRamp is the ordinal ramp for idle magnitude: dim = fresh, bright = rotting.
// Steps skip every other rung of the source ramp; adjacent rungs measured ΔL 0.049,
// under the 0.06 minimum.
var idleRamp = []string{"#256abf", "#3987e5", "#6da7ec", "#9ec5f4", "#cde2fb"}

func rgb(hex string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

func fg(hex, s string) string {
	r, g, b := rgb(hex)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[39m", r, g, b, s)
}

// badge paints text on our own fill, which makes its contrast independent of the
// user's theme. The only element that needs that guarantee is BLOCKED.
func badge(fgHex, bgHex, s string) string {
	fr, fgc, fb := rgb(fgHex)
	br, bgc, bb := rgb(bgHex)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm%s\033[0m", fr, fgc, fb, br, bgc, bb, s)
}

func dim(s string) string  { return fg(inkMuted, s) }
func body(s string) string { return fg(inkSecondary, s) }
