// Package view is board's renderer, and it is pure: Frame and Table are functions
// of a fleet snapshot plus a terminal size and nothing else. That is what lets a
// golden file pin the entire screen (DESIGN.md §11). It never reads the world — no
// clock, no $HOME, no subprocess — and watch never formats. Keep both halves.
//
// Rendering and arithmetic live in separate files, because the seam between them is
// where every fit bug has been (EVIDENCE.md §9.10, §9.12):
//
//   - frame.go, header.go, table.go — rendering, the two output surfaces
//   - layout.go — the fit: how many lines the frame may occupy, how wide each
//     elastic column is, and what gives way first. Asserted directly rather than
//     inferred from rendered output.
//   - order.go — navigation, which follows the screen rather than the data
//   - palette.go, scale.go, format.go — colour, the idle scale, and cell padding
//
// The frame fits the terminal in both directions. A line wider than the screen
// wraps, a wrapped line makes the frame taller than height() counted, and the
// header is then the first thing to scroll away. clampLine is the backstop, not
// the layout: the arithmetic is what makes lines fit.
//
// Colour is validated, never eyeballed — do not add or substitute a value without
// re-validating it against both terminal backgrounds (DESIGN.md §6).
package view
