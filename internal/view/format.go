package view

import (
	"fmt"
	"strings"
	"time"
)

// pad truncates with an ellipsis rather than wrapping: a row must stay one line,
// or the bands stop being scannable. Width is counted in runes, not bytes.
func pad(s string, w int) string {
	r := []rune(s)
	if len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(r))
}

// cut shortens to fit without padding, unlike pad: the text it trims sits inline
// between other blocks, so trailing spaces would move whatever follows.
func cut(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w < 2 {
		return ""
	}
	return string(r[:w-1]) + "…"
}

// clampLine truncates to w printed columns, passing escape sequences through without
// counting them — an escape costs no columns, so counting bytes or even runes would
// cut the visible text a fifth of the way in.
//
// This is the backstop behind the layout arithmetic, not a substitute for it: a line
// wider than the terminal wraps, and a wrapped line makes the frame occupy more screen
// rows than height() counted, so the fit loop under-reports and the terminal scrolls
// the header away (EVIDENCE.md §9.10, §9.12). The arithmetic has been wrong before; this
// makes the consequence impossible rather than unlikely.
func clampLine(s string, w int) string {
	if w < 0 {
		w = 0
	}
	r := []rune(s)
	var b strings.Builder
	col := 0
	for i := 0; i < len(r); {
		// Frame lines carry SGR colour codes and nothing else; \033[K is added by the
		// writer, after this.
		if r[i] == '\033' {
			j := i
			for j < len(r) && r[j] != 'm' {
				j++
			}
			if j < len(r) {
				j++
			}
			b.WriteString(string(r[i:j]))
			i = j
			continue
		}
		if col == w {
			// Reset, or the colour of the run we cut into bleeds onto the next line.
			return b.String() + "\033[0m"
		}
		b.WriteRune(r[i])
		col++
		i++
	}
	return b.String()
}

// runes is the printed width of unpainted text.
func runes(s string) int { return len([]rune(s)) }

// humanize is the table's duration format: fixed width per magnitude so the IDLE
// column stays aligned.
func humanize(d time.Duration) string {
	m := int(d.Minutes())
	switch {
	case m < 60:
		return fmt.Sprintf("%dm", m)
	case m < 24*60:
		return fmt.Sprintf("%dh%02dm", m/60, m%60)
	default:
		return fmt.Sprintf("%dd%02dh", m/1440, (m/60)%24)
	}
}

// since is a todo's age, and it is deliberately not humanize: a session's idle time is
// a **gap** that resets the moment the session is touched, while a todo's age is a
// **lifetime** that only grows. Rendering both the same way made "0m" read as "active
// this minute" on a note written seconds ago (EVIDENCE.md §9.19). "ago" is what says
// which quantity this is, and the one-unit form is honest — minutes never matter on
// something you wrote yesterday.
func since(d time.Duration) string {
	if d < time.Minute {
		return "now"
	}
	return short(d) + " ago"
}

// short is the one-unit form, for chrome where precision does not matter.
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
