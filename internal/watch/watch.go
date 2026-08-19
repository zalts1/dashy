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

	"github.com/zalts1/dashy/internal/board"
	"github.com/zalts1/dashy/internal/cmux"
	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/editor"
	"github.com/zalts1/dashy/internal/view"
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
			Now:          time.Now(),
			Interval:     interval,
			Threshold:    st.Threshold(),
			Rows:         envInt("LINES", pipedRows),
			Cols:         envInt("COLUMNS", pipedCols),
			EditorScheme: editor.Scheme(st.Config.Editor),
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
	// The one mode this UI has, and the only state that holds unsaved user text.
	var typing bool
	var input string
	// The legend, which is a display mode like the fold: it belongs to the viewer, survives a
	// refresh, and is not persisted (§9.38).
	var help bool
	// The quiet fold, which belongs to the viewer and so survives a refresh. It is not
	// persisted: a fold is about this sitting at the tab, not a preference (§9.21).
	var folded bool
	var lastKey, lastFetch time.Time

	// One UI value for both the frame and the navigation order, because the order has to
	// follow the screen — building it twice is how the two would come to disagree about
	// which rows are on it.
	ui := func() view.UI {
		return view.UI{Sel: sel, Paused: sel != "" || typing, Notice: notice,
			Typing: typing, Input: input, QuietCollapsed: folded, Help: help}
	}

	// Resolved once, outside draw: it is a stat over three app bundles, and unlike the
	// threshold it cannot change while the tab is open in any way worth a redraw for —
	// `board editor` writes the config, and picking the change up on the next `board watch`
	// is the same deal `poll_seconds` gets.
	scheme := editor.Scheme(config.Load().Config.Editor)

	draw := func() {
		rows, cols := termSize()
		s := view.Frame(f, view.Screen{
			Now:          lastFetch,
			Interval:     interval,
			Threshold:    config.Load().Threshold(),
			Rows:         rows,
			Cols:         cols,
			EditorScheme: scheme,
		}, ui())
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
		case ev := <-keys:
			lastKey = time.Now()
			// Capture mode owns every key while it is on, which is why the decoder hands
			// over both readings: in here `q` is a letter, not the quit key. ctrl-c still
			// raises SIGINT (ISIG is left on), so there is always a way out.
			if typing {
				switch ev.k {
				case keyEnter:
					typing = false
					notice, input = commitTodo(input), ""
					refresh()
					continue
				case keyEscape:
					typing, input, notice = false, "", ""
				case keyBackspace:
					if r := []rune(input); len(r) > 0 {
						input = string(r[:len(r)-1])
					}
				default:
					input += ev.text
				}
				draw()
				continue
			}
			switch ev.k {
			case keyQuit:
				return
			case keyAdd:
				// Refused before the mode opens rather than after the text is typed: at the
				// cap the answer is the same either way, and this way it costs no keystrokes
				// (DESIGN.md §12).
				if _, _, todos, _ := f.Bands(); len(todos) >= config.MaxTodos {
					notice = fmt.Sprintf("at the cap of %d todos — finish one first", config.MaxTodos)
				} else {
					// The prompt takes the same line the legend was on, so the legend goes.
					typing, notice, help = true, "", false
				}
			case keyEscape:
				sel, help = "", false
			case keyHelp:
				// A display mode, so it does not pause: nothing on this line goes stale, and a
				// reader holding the legend open should still watch the fleet move (§9.38).
				help = !help
			case keyList:
				// Straight to the top of the list: stepping there means walking past every
				// quiet row (DESIGN.md §12).
				if k := view.FirstTodo(f); k != "" {
					sel, notice = k, ""
				} else {
					notice = "no todos · a to add one"
				}
			case keyFold:
				folded = !folded
				// A folded row is not a navigation stop, so a selection left inside the band
				// would sit on a row the frame is not drawing — with Enter still pointed at
				// it and the refresh still paused for it.
				if r, ok := f.ByKey(sel); folded && ok && r.Rank == board.RankQuiet {
					sel = ""
				}
			case keyUp:
				sel = view.Step(view.DisplayOrder(f, ui()), sel, -1)
			case keyDown:
				sel = view.Step(view.DisplayOrder(f, ui()), sel, +1)
			case keyDone:
				// The only write this loop makes, and the only destructive key in the
				// frame. It is gated on the row being a todo — nothing here can end a
				// session (DESIGN.md §8) — and it reports the text it removed, which is
				// the only record of it once it is gone (§12).
				if r, ok := f.ByKey(sel); ok {
					if id, isTodo := r.TodoID(); isTodo {
						sel, notice = "", finishTodo(id)
						refresh()
						continue
					}
					notice = "d finishes a todo; this row is a session"
				}
			case keyEnter:
				if r, ok := f.ByKey(sel); ok {
					// Focus the target and keep running. cmux switches the visible tab;
					// board stays alive in its own, so it is still here on return.
					sel, notice = "", ""
					if !r.Jumpable() {
						// Selectable but not jumpable: a background agent is a row so its
						// blocked state is visible, and it has no tab to bring forward.
						notice = "no tab to jump to"
					} else if err := cmux.Focus(r.Surface); err != nil {
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
			// Capture has no timeout, deliberately: §7's 10s return exists so a stray arrow
			// key cannot leave the tab paused forever, and typing is not stray. A timer that
			// discarded half-typed text would be the one destructive thing in this loop.
			if typing {
				continue
			}
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
