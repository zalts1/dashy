package watch

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The decoder is mode-blind: every keystroke carries both the command it names and the
// text it would type, and the loop picks. These are the pairings that matter — a letter
// that is also a command, and an escape sequence that must never become text.
func TestDecodeCarriesCommandAndText(t *testing.T) {
	cases := []struct {
		in   string
		k    key
		text string
	}{
		{"j", keyDown, "j"},
		{"k", keyUp, "k"},
		{"q", keyQuit, "q"},
		{"d", keyDone, "d"},
		{"a", keyAdd, "a"},
		{"t", keyList, "t"},
		{"z", keyFold, "z"},
		// The same physical keys on the Hebrew layout: a keymap of single letters is dead
		// in any non-Latin input source, and switching layouts to press one key is not a
		// keymap. These runes cannot be typed on a Latin layout, so nothing is ambiguous.
		{"א", keyList, "א"},
		{"ש", keyAdd, "ש"},
		{"ג", keyDone, "ג"},
		{"ח", keyDown, "ח"},
		{"ל", keyUp, "ל"},
		{"ז", keyFold, "ז"},
		{"\r", keyEnter, ""},
		{"\x1b", keyEscape, ""},
		{"\x7f", keyBackspace, ""},
		// The trap: the tail of an arrow key is printable, so a rune-wise decode types
		// "[A" into whatever is being captured.
		{"\x1b[A", keyUp, ""},
		{"\x1b[B", keyDown, ""},
		{"\x1bOA", keyUp, ""}, // the same key, application mode
		// Ordinary text, including a letter that names no command.
		{"x", keyNone, "x"},
		{" ", keyNone, " "},
		{"é", keyNone, "é"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := decode(c.in)
			if len(got) != 1 {
				t.Fatalf("%q decoded to %d events, want 1: %+v", c.in, len(got), got)
			}
			if got[0].k != c.k {
				t.Errorf("%q decoded as key %d, want %d", c.in, got[0].k, c.k)
			}
			if got[0].text != c.text {
				t.Errorf("%q typed %q, want %q", c.in, got[0].text, c.text)
			}
		})
	}
}

// A read is not a keystroke. Typing quickly, holding a key, or pasting coalesces bytes
// into one read, so a decoder that answers one event per chunk drops everything after
// the first character — which is how two backspaces became none.
func TestDecodeSplitsACoalescedRead(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []event
	}{
		{"typed fast", "hel", []event{{keyNone, "h"}, {keyNone, "e"}, {keyNone, "l"}}},
		{"two backspaces", "\x7f\x7f", []event{{keyBackspace, ""}, {keyBackspace, ""}}},
		{"a key then its text", "ax", []event{{keyAdd, "a"}, {keyNone, "x"}}},
		{"text then enter", "hi\r", []event{{keyNone, "h"}, {keyNone, "i"}, {keyEnter, ""}}},
		{"typed over an arrow key", "\x1b[Ax", []event{{keyUp, ""}, {keyNone, "x"}}},
		{"a tab is neither text nor a command", "a\tb", []event{{keyAdd, "a"}, {keyNone, "b"}}},
		{"a truncated sequence types nothing", "\x1b[", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decode(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("%q decoded to %+v, want %+v", c.in, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("%q event %d = %+v, want %+v", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}

// readKeys reads a file, so a pipe is the whole fixture — no terminal involved.
func TestReadKeysStreamsEveryKeystrokeInAChunk(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ch := readKeys(r)
	w.Write([]byte("hi\x7f\r"))

	var typed strings.Builder
	var got []key
	for range 4 {
		select {
		case ev := <-ch:
			typed.WriteString(ev.text)
			got = append(got, ev.k)
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d events arrived: %v", len(got), got)
		}
	}
	w.Close()
	if typed.String() != "hi" {
		t.Errorf("typed %q, want hi", typed.String())
	}
	if got[2] != keyBackspace || got[3] != keyEnter {
		t.Errorf("keys = %v, want the backspace and the enter last", got)
	}
}

// `?` is help, and it is the one key in the map that is not a letter — so the one with no
// Hebrew twin, since the glyph is the same wherever it can be typed at all (§10.8, §9.38).
func TestHelpKey(t *testing.T) {
	if got := command('?'); got != keyHelp {
		t.Errorf("command('?') = %v, want keyHelp", got)
	}
	// It still carries its text, because inside the capture mode it is a character: a todo may
	// legitimately contain a question mark, and which reading applies is the loop's decision
	// rather than the decoder's (§12).
	evs := decode("?")
	if len(evs) != 1 || evs[0].text != "?" || evs[0].k != keyHelp {
		t.Errorf(`decode("?") = %+v, want one event carrying both readings`, evs)
	}
}
