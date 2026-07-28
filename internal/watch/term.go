package watch

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

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

// isTTY asks the kernel for a window size. A ModeCharDevice check is not enough:
// /dev/null is a character device, so redirecting to it would look like a terminal
// and spin the redraw loop forever.
func isTTY(f *os.File) bool {
	ws := &winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	return err == 0
}

// rawMode disables echo and line buffering. ISIG is deliberately left on so ctrl-c
// still raises SIGINT and the normal restore path runs.
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

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}
