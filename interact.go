package main

// Keyboard selection for `board watch`, plus the jump itself.
//
// Two rules taken from prior art rather than invented: the selection is keyed on
// the session's surface id, never a row index (htop's Follow key exists because
// index-based selection drifts when the list re-sorts under you), and the refresh
// pauses visibly while a selection is live (less's +F makes the streaming/
// interacting boundary explicit instead of clever).

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const selectTimeout = 10 * time.Second

// rawMode disables echo and line buffering. ISIG is deliberately left on so
// ctrl-c still raises SIGINT and the normal restore path runs.
func rawMode(f *os.File) (func(), error) {
	var old syscall.Termios
	if _, _, e := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGETA,
		uintptr(unsafe.Pointer(&old)), 0, 0, 0); e != 0 {
		return nil, e
	}
	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON
	set := func(t *syscall.Termios) {
		syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCSETA,
			uintptr(unsafe.Pointer(t)), 0, 0, 0)
	}
	set(&raw)
	return func() { set(&old) }, nil
}

type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyEnter
	keyEscape
	keyQuit
)

// readKeys decodes just the keys this UI uses. Escape sequences usually arrive as
// one read, so the whole chunk is parsed at once.
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
			s := string(buf[:n])
			switch {
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

// focusSurface brings a session's cmux tab to the front. The parameter really is
// surface_id; passing "surface" fails with invalid_params.
func focusSurface(id string) error {
	if id == "" {
		return fmt.Errorf("session has no surface id")
	}
	out, err := cmux("rpc", "surface.focus", fmt.Sprintf(`{"surface_id":%q}`, id))
	if err != nil {
		return fmt.Errorf("cmux focus failed: %w", err)
	}
	if strings.Contains(string(out), "error") {
		return fmt.Errorf("cmux focus refused: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// jump focuses the one session whose label or workspace matches q.
func jump(q string) error {
	if q == "" {
		return fmt.Errorf("usage: board jump <substring>")
	}
	var hits []row
	for _, r := range collect().rows {
		if strings.Contains(strings.ToLower(r.label), strings.ToLower(q)) ||
			strings.Contains(strings.ToLower(r.workspace), strings.ToLower(q)) {
			hits = append(hits, r)
		}
	}
	switch len(hits) {
	case 0:
		return fmt.Errorf("no session matching %q", q)
	case 1:
		if err := focusSurface(hits[0].surface); err != nil {
			return err
		}
		fmt.Printf("→ %s  [%s]\n", hits[0].label, hits[0].workspace)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "%d sessions match %q:\n", len(hits), q)
		for _, r := range hits {
			fmt.Fprintf(os.Stderr, "   %s  [%s]\n", r.label, r.workspace)
		}
		return fmt.Errorf("be more specific")
	}
}

// displayOrder lists surface ids in the order frame draws them, so ↑/↓ move the
// way the screen looks rather than the way the data is sorted.
func displayOrder(f fleet) []string {
	blocked, working, quiet := bands(f)
	var out []string
	for _, group := range [][]row{blocked, working, quiet} {
		for _, r := range group {
			if r.surface != "" {
				out = append(out, r.surface)
			}
		}
	}
	return out
}

func step(order []string, sel string, delta int) string {
	if len(order) == 0 {
		return ""
	}
	if sel == "" {
		if delta > 0 {
			return order[0]
		}
		return order[len(order)-1]
	}
	for i, id := range order {
		if id == sel {
			j := i + delta
			if j < 0 {
				j = 0
			}
			if j >= len(order) {
				j = len(order) - 1
			}
			return order[j]
		}
	}
	return order[0] // the selected session vanished between ticks
}
