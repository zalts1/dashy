package view

import (
	"fmt"
	"strings"
	"time"
)

// pad truncates with an ellipsis rather than wrapping: a row must stay one line,
// or the bands stop being scannable. Width is counted in runes, not bytes.
func pad(s string, w int) string {
	// A column can be squeezed to nothing — the tail gives way last and reaches zero on a
	// narrow tab (§9.12) — and a column of no width holds no text. Without this, r[:w-1]
	// indexed backwards and the frame panicked instead of rendering (§9.34).
	if w <= 0 {
		return ""
	}
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
//
// A truncated line closes what it cut into. A colour left open bleeds onto the next line;
// a hyperlink left open is worse, because it is not a colour — every cell written after it
// joins the link, so one clipped row would make the rest of the screen click through to
// somebody else's dev server (§9.34).
func clampLine(s string, w int) string {
	if w < 0 {
		w = 0
	}
	r := []rune(s)
	var b strings.Builder
	col, linked := 0, false
	for i := 0; i < len(r); {
		if r[i] == '\033' {
			j := escape(r, i)
			seq := string(r[i:j])
			// The only escape here with state to unwind. Its close is the same sequence with
			// an empty URI, so "did this open one" is "was there a URI".
			if strings.HasPrefix(seq, linkOpen) {
				linked = seq != linkClose
			}
			b.WriteString(seq)
			i = j
			continue
		}
		if col == w {
			out := b.String() + "\033[0m"
			if linked {
				out += linkClose
			}
			return out
		}
		b.WriteRune(r[i])
		col++
		i++
	}
	return b.String()
}

// escape returns the index just past the escape sequence beginning at r[i]. Two families
// reach the frame and they end differently: SGR colour is a CSI, terminated by one byte in
// 0x40–0x7E, and a hyperlink is an OSC, terminated by ST or BEL. Scanning to the first `m`
// covered the first and mangled the second — an `m` inside a URL ended the sequence early
// and the rest of the URL was then counted as visible text (§9.34).
//
// A sequence that runs off the end of the line consumes the remainder rather than
// resuming as text: half an escape is not something to guess at.
func escape(r []rune, i int) int {
	j := i + 1
	if j >= len(r) {
		return j
	}
	switch r[j] {
	case '[':
		for j++; j < len(r) && (r[j] < '@' || r[j] > '~'); j++ {
		}
		if j < len(r) {
			j++
		}
	case ']':
		for j++; j < len(r); j++ {
			if r[j] == '\a' {
				j++
				break
			}
			if r[j] == '\033' && j+1 < len(r) && r[j+1] == '\\' {
				j += 2
				break
			}
		}
	default:
		// A two-byte sequence — ST itself, when an OSC was terminated by something this
		// scanner already consumed.
		j++
	}
	return j
}

// runes is the printed width of unpainted text.
func runes(s string) int { return len([]rune(s)) }

// printed is the same measurement for a line that has already been painted: the width
// clampLine would count. Measuring a rendered line beats re-deriving its width from the
// arithmetic that produced it — that is two places to get one number wrong. It shares
// escape() with clampLine for exactly that reason: two scanners disagreeing about where a
// sequence ends is two answers to one question (§9.34).
func printed(s string) int {
	r := []rune(s)
	n := 0
	for i := 0; i < len(r); {
		if r[i] == '\033' {
			i = escape(r, i)
			continue
		}
		n++
		i++
	}
	return n
}

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
