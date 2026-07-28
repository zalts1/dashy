package watch

import "os"

type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyEnter
	keyEscape
	keyQuit
)

// readKeys decodes just the keys this UI uses — there are no modes and nothing to
// learn. Escape sequences usually arrive as one read, so the whole chunk is parsed
// at once rather than byte by byte.
func readKeys(f *os.File) <-chan key {
	ch := make(chan key, 8)
	go func() {
		defer close(ch)
		buf := make([]byte, 8)
		for {
			n, err := f.Read(buf)
			if err != nil {
				return
			}
			switch s := string(buf[:n]); {
			case s == "\x1b[A", s == "k":
				ch <- keyUp
			case s == "\x1b[B", s == "j":
				ch <- keyDown
			case s == "\r", s == "\n":
				ch <- keyEnter
			case s == "\x1b":
				ch <- keyEscape
			case s == "q":
				ch <- keyQuit
			}
		}
	}()
	return ch
}
