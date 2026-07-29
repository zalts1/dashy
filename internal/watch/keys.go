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
	keyBackspace
)

// event carries both readings of one keystroke: the command it names, and the text it
// would type. The decoder stays mode-blind — it is the loop that knows whether `q` is
// "quit" or the letter q, and a decoder that had to be told would be a second place
// where the mode can be wrong (DESIGN.md §12).
type event struct {
	k    key
	text string // the printable rune, when there is one
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
func command(r rune) key {
	switch r {
	case 'k':
		return keyUp
	case 'j':
		return keyDown
	case 'q':
		return keyQuit
	case 'd':
		return keyDone
	case 'a':
		return keyAdd
	case 't':
		return keyList
	}
	return keyNone
}

// isFinal reports the last byte of a CSI sequence, which is the range 0x40–0x7E.
func isFinal(r rune) bool { return r >= '@' && r <= '~' }
