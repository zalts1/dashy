package watch

import (
	"os"
	"unicode"
)

type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyEnter
	keyEscape
	keyQuit
	keyDone
	keyAdd
	keyList
	keyFold
	keyBackspace
	keyClick
	// The wheel moves the caret exactly as keyUp and keyDown do, and is a separate key only
	// so the loop can ration it: one flick of a trackpad is many reports, and a keystroke
	// is one keystroke (§9.32).
	keyScrollUp
	keyScrollDown
)

// event carries both readings of one keystroke: the command it names, and the text it
// would type. The decoder stays mode-blind — it is the loop that knows whether `q` is
// "quit" or the letter q, and a decoder that had to be told would be a second place
// where the mode can be wrong (DESIGN.md §12).
type event struct {
	k    key
	text string // the printable rune, when there is one
	// row is the screen line a keyClick landed on, counted from zero. It is the one thing
	// no keystroke carries, and the only position anywhere in this UI — the loop turns it
	// into a session key against the frame on screen and never keeps it (§7).
	row int
}

// readKeys decodes the keys this UI uses, plus text for the one mode it has.
func readKeys(f *os.File) <-chan event {
	ch := make(chan event, 32)
	go func() {
		defer close(ch)
		buf := make([]byte, 256)
		for {
			n, err := f.Read(buf)
			if err != nil {
				return
			}
			for _, ev := range decode(string(buf[:n])) {
				ch <- ev
			}
		}
	}()
	return ch
}

// decode turns one read into the keystrokes it contains. **A read is not a keystroke:**
// typing quickly, holding a key or pasting coalesces bytes, so answering one event per
// chunk silently drops everything after the first character — which is how two
// backspaces became none (EVIDENCE.md §9.18).
func decode(s string) []event {
	var out []event
	r := []rune(s)
	for i := 0; i < len(r); {
		switch {
		case r[i] == '\x1b':
			// A CSI or SS3 sequence is consumed whole. Its tail is printable, so leaving
			// it to the text branch types "[A" on every arrow key.
			if i+1 < len(r) && (r[i+1] == '[' || r[i+1] == 'O') {
				j := i + 2
				for j < len(r) && !isFinal(r[j]) {
					j++
				}
				if j == len(r) {
					return out // truncated: never guess at a partial sequence
				}
				switch r[j] {
				case 'A':
					out = append(out, event{k: keyUp})
				case 'B':
					out = append(out, event{k: keyDown})
				case 'M', 'm':
					// The pre-SGR mouse encoding is `ESC [ M` with no parameters and three
					// coordinate bytes after the final one. board never asks for it, but a
					// terminal that declined ?1006 sends it anyway — and those three bytes are
					// printable, so leaving them types them, and one of them can be `q`.
					if j == i+2 {
						i = min(j+4, len(r))
						continue
					}
					if ev, ok := sgrMouse(string(r[i+2:j]), r[j]); ok {
						out = append(out, ev)
					}
				}
				i = j + 1
				continue
			}
			out = append(out, event{k: keyEscape})
			i++
		case r[i] == '\r' || r[i] == '\n':
			out = append(out, event{k: keyEnter})
			i++
		case r[i] == '\x7f' || r[i] == '\b':
			out = append(out, event{k: keyBackspace})
			i++
		case unicode.IsPrint(r[i]):
			out = append(out, event{k: command(r[i]), text: string(r[i])})
			i++
		default:
			// Control bytes are neither text nor a command: a pasted tab must not end up
			// inside a todo.
			i++
		}
	}
	return out
}

// command is the letter half of the keymap. Every one of these is also text, and which
// reading applies is the loop's decision, not this function's.
//
// The Hebrew runes are the *same physical keys* on the Israeli layout. A keymap of single
// letters is otherwise dead in any non-Latin input source, and telling someone to switch
// layouts to press one key is not a keymap. They are unambiguous because none of them can
// be produced by a Latin layout — unlike `q`, whose Hebrew position emits `/`, which a
// Latin user does have a key for, so quitting there stays ctrl-c.
func command(r rune) key {
	switch r {
	case 'k', 'ל':
		return keyUp
	case 'j', 'ח':
		return keyDown
	case 'q':
		return keyQuit
	case 'd', 'ג':
		return keyDone
	case 'a', 'ש':
		return keyAdd
	case 't', 'א':
		return keyList
	// `z` is vim's fold prefix, and deliberately not `c`: this tool has exactly one
	// dangerous documented action and it is called `close` (§10.6).
	case 'z', 'ז':
		return keyFold
	}
	return keyNone
}

// isFinal reports the last byte of a CSI sequence, which is the range 0x40–0x7E.
func isFinal(r rune) bool { return r >= '@' && r <= '~' }
