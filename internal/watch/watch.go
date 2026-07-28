// Package watch is the ambient dashboard's loop: `board watch`. It owns everything
// impure — the alternate screen, termios, signals, the ticker — and nothing about
// layout, which lives in the pure view package. Keep that split: it is the only
// reason render bugs are testable here.
package watch

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"board/internal/board"
	"board/internal/cmux"
	"board/internal/config"
	"board/internal/view"
)

// selectTimeout returns the tab to ambient on its own, so a stray arrow key can
// never leave the dashboard paused forever.
const selectTimeout = 10 * time.Second

// Fallback size for the non-TTY single frame, overridable with LINES/COLUMNS.
const (
	pipedRows = 44
	pipedCols = 118
)

func Run(interval time.Duration) {
	out := os.Stdout
	// Piped or redirected: emit one frame and exit, so the dashboard can be captured
	// or diffed without a terminal to drive. This is the main manual verification path.
	if !isTTY(out) {
		st := config.Load()
		fmt.Fprint(out, view.Frame(board.Collect(), view.Screen{
			Now:       time.Now(),
			Interval:  interval,
			Threshold: st.Threshold(),
			Rows:      envInt("LINES", pipedRows),
			Cols:      envInt("COLUMNS", pipedCols),
		}, view.UI{}))
		return
	}

	// Alternate screen so exiting restores the shell untouched.
	fmt.Fprint(out, "\033[?1049h\033[?25l")
	restoreTerm, _ := rawMode(os.Stdin)
	var once sync.Once
	restore := func() {
		once.Do(func() {
			if restoreTerm != nil {
				restoreTerm()
			}
			fmt.Fprint(out, "\033[?25h\033[?1049l")
		})
	}
	// Deferred so it runs on panic as well as on the normal paths — a process that
	// dies without restoring termios leaves the user's shell with no echo.
	defer restore()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGWINCH)
	keys := readKeys(os.Stdin)

	var f board.Fleet
	var sel, notice string
	var lastKey, lastFetch time.Time

	draw := func() {
		rows, cols := termSize()
		s := view.Frame(f, view.Screen{
			Now:       lastFetch,
			Interval:  interval,
			Threshold: config.Load().Threshold(),
			Rows:      rows,
			Cols:      cols,
		}, view.UI{Sel: sel, Paused: sel != "", Notice: notice})
		render(out, s)
	}
	refresh := func() { f, lastFetch = board.Collect(), time.Now(); draw() }

	refresh()
	// One-second cadence drives both the data interval and the selection timeout; no
	// work happens unless something is actually due.
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case s := <-sig:
			if s == syscall.SIGWINCH {
				draw()
				continue
			}
			return
		case k := <-keys:
			lastKey = time.Now()
			switch k {
			case keyQuit:
				return
			case keyEscape:
				sel = ""
			case keyUp:
				sel = view.Step(view.DisplayOrder(f), sel, -1)
			case keyDown:
				sel = view.Step(view.DisplayOrder(f), sel, +1)
			case keyEnter:
				if sel != "" {
					// Focus the target and keep running. cmux switches the visible tab;
					// board stays alive in its own, so it is still here on return.
					target := sel
					sel, notice = "", ""
					if err := cmux.Focus(target); err != nil {
						// Reported in the frame, not on stderr: once the jump lands, this
						// tab is not the visible one.
						notice = err.Error()
					}
					refresh()
					continue
				}
			}
			draw()
		case <-tick.C:
			// Refresh pauses while a selection is live so rows cannot re-sort under the
			// cursor; the timeout guarantees it always returns to ambient.
			if sel != "" {
				if time.Since(lastKey) > selectTimeout {
					sel = ""
					refresh()
				}
				continue
			}
			if time.Since(lastFetch) >= interval {
				refresh()
			}
		}
	}
}

// render writes one frame as a single buffer: cursor home, overwrite line by line,
// then clear below. A full-screen clear each tick would flash — the previous frame
// must stay until its pixels are replaced.
func render(out *os.File, s string) {
	var b bytes.Buffer
	b.WriteString("\033[H")
	for _, line := range strings.Split(s, "\n") {
		b.WriteString(line + "\033[K\n")
	}
	b.WriteString("\033[J")
	out.Write(b.Bytes())
}
