package main

// The ambient dashboard: `board watch`. Redraws in place on an interval so it can
// live in a dedicated cmux tab.
//
// Colour comes from the validated data-viz palette. Every value below cleared the
// checks against both the lightest and darkest plausible terminal backgrounds
// (#282c34 and #040404) — see DESIGN.md §11. Do not substitute by eye.

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	inkPrimary   = "#ffffff"
	inkSecondary = "#c3c2b7"
	inkMuted     = "#898781"

	statusCritical = "#d03b3b" // blocked — only ever as a filled badge, never bare text
	statusGood     = "#0ca30c" // running
	statusWarning  = "#fab219" // past the idle threshold
)

// Ordinal ramp for idle magnitude: dim = fresh, bright = rotting. Brightness
// increases with age because on a dark surface the sequential anchor flips.
var idleRamp = []string{"#256abf", "#3987e5", "#6da7ec", "#9ec5f4", "#cde2fb"}

const sevenDays = 168.0 // hours, the top of the idle scale

func rgb(hex string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

func fg(hex, s string) string {
	r, g, b := rgb(hex)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[39m", r, g, b, s)
}

func badge(fgHex, bgHex, s string) string {
	fr, fgc, fb := rgb(fgHex)
	br, bgc, bb := rgb(bgHex)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm%s\033[0m", fr, fgc, fb, br, bgc, bb, s)
}

func dim(s string) string  { return fg(inkMuted, s) }
func body(s string) string { return fg(inkSecondary, s) }

// idleScale maps a duration onto the shared log scale: 0..1, plus a ramp index.
// Log because the range spans 0m to a week; linear would flatten everything under
// a day into the first cell.
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

const barCells = 12

func bar(d time.Duration) string {
	frac, step := idleScale(d)
	n := 1 + int(math.Round(frac*float64(barCells-1)))
	return fg(idleRamp[step], strings.Repeat("▇", n)) + strings.Repeat(" ", barCells-n)
}

type winsize struct{ row, col, x, y uint16 }

func termSize() (rows, cols int) {
	ws := &winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if err != 0 || ws.row == 0 {
		return 40, 100
	}
	return int(ws.row), int(ws.col)
}

// frame renders one complete screen. Pure function of the fleet so it can be
// tested and diffed without a terminal.
func frame(f fleet, now time.Time, interval time.Duration, thresh time.Duration, rows, cols int) string {
	var b bytes.Buffer
	// Size the label column to the longest label actually present, so the bars sit
	// next to the text instead of across a gap of padding.
	labelW := 0
	for _, r := range f.rows {
		if n := len([]rune(r.label)); n > labelW {
			labelW = n
		}
	}
	if max := cols - 46; labelW > max {
		labelW = max
	}
	if labelW > 80 {
		labelW = 80
	}
	if labelW < 18 {
		labelW = 18
	}

	var blocked, working, quiet []row
	for _, r := range f.rows {
		switch r.rank {
		case 0:
			blocked = append(blocked, r)
		case 2:
			working = append(working, r)
		default:
			quiet = append(quiet, r)
		}
	}

	b.WriteString("\n  " + fg(inkPrimary, "BOARD") + "   " +
		dim(fmt.Sprintf("%d sessions · %s · every %s",
			len(f.rows), now.Format("15:04:05"), short(interval))) + "\n\n")

	// KPI strip — the sub-second read. Blocked is the only thing allowed to shout.
	blockedCell := dim(fmt.Sprintf("%d blocked", len(blocked)))
	if len(blocked) > 0 {
		blockedCell = badge(inkPrimary, statusCritical, fmt.Sprintf(" %d BLOCKED ", len(blocked)))
	}
	b.WriteString("  " + blockedCell +
		dim("     ") + body(fmt.Sprintf("%d working", len(working))) +
		dim("     ") + body(fmt.Sprintf("%d quiet >%s", f.stale, short(thresh))) +
		dim("     ") + dim("oldest ") + body(humanize(f.oldest)) + "\n")

	// Height budget: chrome is fixed, the quiet tail absorbs whatever is left.
	spent := 6 + 2 + len(working) + 2 + 3
	if len(f.asks) > 0 {
		spent += 8 // ASKED band: header, up to six rows, spacer
	}
	if len(blocked) > 0 {
		spent += len(blocked)
	}
	room := rows - spent
	if room < 3 {
		room = 3
	}

	// One gutter width for every band so labels share a single left edge. The state
	// mark is right-aligned inside it so it hugs the label instead of leaving a gap.
	const gutter = 10
	mark := func(s string, width int) string {
		return strings.Repeat(" ", gutter-width-1) + s + " "
	}
	// Spacer must match the row layout exactly: gap + bar + gap + warn + gap.
	b.WriteString("\n   " + dim(strings.Repeat(" ", gutter)+pad("LABEL", labelW)+
		strings.Repeat(" ", barCells+4)+fmt.Sprintf("%7s", "IDLE")+"  WORKSPACE") + "\n")

	line := func(state, label string, showBar bool, r row) string {
		warn := " "
		if r.stale {
			warn = fg(statusWarning, "⚠")
		}
		barCell := strings.Repeat(" ", barCells)
		if showBar {
			barCell = bar(r.idle)
		}
		return "   " + state + body(pad(label, labelW)) + " " + barCell + " " + warn + " " +
			body(fmt.Sprintf("%7s", humanize(r.idle))) + "  " + dim(r.workspace) + "\n"
	}

	if len(blocked) > 0 {
		b.WriteString("\n  " + fg(statusCritical, "NEEDS YOU") + "\n")
		for _, r := range blocked {
			// Blocked rows carry the bar too: the same quantity on the same absolute
			// scale, so "waiting 3h" is comparable to anything in QUIET.
			b.WriteString(line(mark(badge(inkPrimary, statusCritical, " BLOCKED "), 9), r.label, true, r))
		}
	} else {
		b.WriteString("\n  " + dim("NEEDS YOU") + "   " + dim("nothing blocked") + "\n")
	}

	if len(working) > 0 {
		b.WriteString("\n  " + fg(statusGood, "WORKING") + " " + dim(fmt.Sprintf("· %d", len(working))) + "\n")
		for _, r := range working {
			// No bar: for a working agent elapsed time is progress, not rot.
			b.WriteString(line(mark(fg(statusGood, "◐"), 1), r.label, false, r))
		}
	}

	if len(quiet) > 0 {
		b.WriteString("\n  " + fg(inkSecondary, "QUIET") + " " + dim(fmt.Sprintf("· %d", len(quiet))) + "\n")
		shown := quiet
		if len(shown) > room {
			shown = shown[:room]
		}
		for _, r := range shown {
			b.WriteString(line(mark(dim("○"), 1), r.label, true, r))
		}
		if n := len(quiet) - len(shown); n > 0 {
			b.WriteString("   " + strings.Repeat(" ", gutter) + dim(fmt.Sprintf("⌄  %d more", n)) + "\n")
		}
	}

	// ASKED — the ledger. Rows are the last 24h to keep the band short; the header
	// carries the full window's aggregates so the tail is never silently dropped.
	if len(f.asks) > 0 {
		var day []ask
		var never int
		var longest time.Duration
		for _, a := range f.asks {
			if a.open {
				never++
			}
			if a.waited > longest {
				longest = a.waited
			}
			if now.Sub(a.at) < 24*time.Hour {
				day = append(day, a)
			}
		}
		summary := fmt.Sprintf("%s: %d asks · %d never answered · longest %s",
			short(now.Sub(f.asksSince)), len(f.asks), never, short(longest))
		b.WriteString("\n  " + fg(inkSecondary, "ASKED") + " " +
			dim(fmt.Sprintf("· 24h · %d", len(day))) + "   " + dim(summary) + "\n")
		if len(day) > 6 {
			day = day[:6]
		}
		for _, a := range day {
			// No bar here: waits are minutes against a scale that tops out at a week,
			// so every one would render a single cell and mean nothing. "never" is the
			// only thing in this band worth colour.
			waited := body(fmt.Sprintf("%7s", humanize(a.waited)))
			if a.open {
				waited = fg(statusWarning, fmt.Sprintf("%7s", "never"))
			}
			b.WriteString("   " + mark(dim("·"), 1) + body(pad(a.what, labelW)) +
				strings.Repeat(" ", barCells+3) + waited + "  " +
				dim(a.at.Format("15:04")+" "+a.where) + "\n")
		}
	}

	b.WriteString("\n  " + dim("elapsed ") + scaleLegend() + dim("   ctrl-c to exit") + "\n")
	return b.String()
}

// scaleLegend is the key for the bar ramp. A value scale without a legend is
// decoration; this makes bar length and colour readable as a quantity.
func scaleLegend() string {
	out := ""
	for i, d := range []time.Duration{time.Hour, 6 * time.Hour, 24 * time.Hour, 72 * time.Hour, 168 * time.Hour} {
		_, step := idleScale(d)
		out += fg(idleRamp[step], "▇") + dim(" "+short(d))
		if i < 4 {
			out += dim("  ")
		}
	}
	return out
}

func short(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// isTTY asks the kernel for a window size. A ModeCharDevice check is not enough:
// /dev/null is a character device, so redirecting to it would look like a terminal
// and spin the redraw loop forever.
func isTTY(f *os.File) bool {
	ws := &winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	return err == 0
}

func watch(interval time.Duration) {
	out := os.Stdout
	// Piped or redirected: emit one frame and exit, so the dashboard can be
	// captured or diffed without a terminal to drive.
	if !isTTY(out) {
		st := loadState()
		rows, cols := envInt("LINES", 44), envInt("COLUMNS", 118)
		fmt.Fprint(out, frame(collect(), time.Now(), interval, st.threshold(), rows, cols))
		return
	}
	// Alternate screen so exiting restores the shell untouched.
	fmt.Fprint(out, "\033[?1049h\033[?25l")
	restore := func() { fmt.Fprint(out, "\033[?25h\033[?1049l") }

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGWINCH)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	draw := func() {
		st := loadState()
		f := collect()
		rows, cols := termSize()
		s := frame(f, time.Now(), interval, st.threshold(), rows, cols)
		// Home, overwrite line by line, then clear below. A full-screen clear each
		// tick would flash — the previous frame stays until its pixels are replaced.
		var b bytes.Buffer
		b.WriteString("\033[H")
		for _, line := range strings.Split(s, "\n") {
			b.WriteString(line + "\033[K\n")
		}
		b.WriteString("\033[J")
		out.Write(b.Bytes())
	}

	draw()
	for {
		select {
		case s := <-sig:
			if s == syscall.SIGWINCH {
				draw()
				continue
			}
			restore()
			return
		case <-tick.C:
			draw()
		}
	}
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}
