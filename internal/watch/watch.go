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
	// Read once at startup, not per draw: a tab that started reporting mouse events would
	// have to stop reporting them too, and a mode that turns itself on mid-session is a
	// mode nobody asked for.
	mouse := config.Load().Config.Mouse
	if mouse {
		fmt.Fprint(out, mouseOn)
	}
	var once sync.Once
	restore := func() {
		once.Do(func() {
			if restoreTerm != nil {
				restoreTerm()
			}
			// Before the alternate screen goes: a terminal left reporting presses swallows
			// drag-to-select in the shell board hands back, with nothing on screen to explain it.
			if mouse {
				fmt.Fprint(out, mouseOff)
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
	// The quiet fold, which belongs to the viewer and so survives a refresh. It is not
	// persisted: a fold is about this sitting at the tab, not a preference (§9.21).
	var folded bool
	var lastKey, lastFetch time.Time
	// The caret's own clock. It is not the selection timeout's: that one measures the
	// reader going quiet, this one measures one gesture ending.
	var step stepClock

	// One UI value for both the frame and the navigation order, because the order has to
	// follow the screen — building it twice is how the two would come to disagree about
	// which rows are on it.
	ui := func() view.UI {
		return view.UI{Sel: sel, Paused: sel != "" || typing, Notice: notice,
			Typing: typing, Input: input, QuietCollapsed: folded}
	}

	// hits is the frame currently on the screen, line by line. A click is the only input
	// here that is a position rather than an identity, so it is resolved against the frame
	// the reader actually clicked on and never against a freshly computed one (§7).
	var hits []string
	draw := func() {
		rows, cols := termSize()
		s, h := view.FrameHits(f, view.Screen{
			Now:       lastFetch,
			Interval:  interval,
			Threshold: config.Load().Threshold(),
			Rows:      rows,
			Cols:      cols,
		}, ui())
		hits = h
		render(out, s)
	}
	refresh := func() { f, lastFetch = board.Collect(), time.Now(); draw() }

	// jump focuses a row's tab and keeps looping. cmux switches the visible tab; board
	// stays alive in its own, so it is still here on return (§9.7).
	jump := func(r board.Row) {
		sel, notice = "", ""
		if !r.Jumpable() {
			// Selectable but not jumpable: a background agent is a row so its blocked state
			// is visible, and it has no tab to bring forward.
			notice = "no tab to jump to"
		} else if err := cmux.Focus(r.Surface); err != nil {
			// Reported in the frame, not on stderr: once the jump lands, this tab is not
			// the visible one.
			notice = err.Error()
		}
		refresh()
	}

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
				case keyClick, keyScrollUp, keyScrollDown:
					// Capture owns every input while it is on, and the mouse carries no text, so
					// the default branch would type nothing while looking like it did something.
					// There is no row to point at: the mode is the frame's bottom line.
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
					typing, notice = true, ""
				}
			case keyEscape:
				sel = ""
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
			case keyUp, keyDown, keyScrollUp, keyScrollDown:
				// One rule for all four, because they are the same events: with mouse reporting
				// off the terminal turns wheel notches into arrow keys, so a flick has always
				// arrived here as nine presses inside one millisecond and moved the caret nine
				// rows. Collapsing the burst is what makes a gesture one step; a finger is never
				// that fast, so nothing a reader types is affected (§9.32).
				if !step.allow(lastKey) {
					continue
				}
				delta := +1
				if ev.k == keyUp || ev.k == keyScrollUp {
					delta = -1
				}
				sel = view.Step(view.DisplayOrder(f, ui()), sel, delta)
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
					jump(r)
					continue
				}
			case keyClick:
				// Two clicks, not one: the first selects, which pauses the refresh, and only
				// then is the frame still enough for the second to mean what it looks like.
				// One-click-to-jump would be the §7 cursor problem with no defence — selection
				// survives a re-sort because it is keyed on the session, and a position cannot
				// be. The second click is checked against the key, so it jumps to the row the
				// caret is on or it does nothing.
				k := hitAt(hits, ev.row)
				if k == "" {
					// A miss — the header, a band heading, the legend, the space below the
					// frame. It must not move the caret to whatever row was nearest.
					continue
				}
				if k == sel {
					if r, ok := f.ByKey(k); ok {
						jump(r)
						continue
					}
				}
				sel, notice = k, ""
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
//
// Cursor home is also what makes a click resolvable: the frame starts at the top row, so
// its line index and the terminal's row are the same number, and the hit map can be
// indexed by a mouse coordinate directly.
func render(out *os.File, s string) {
	var b bytes.Buffer
	b.WriteString("\033[H")
	for _, line := range strings.Split(s, "\n") {
		b.WriteString(line + "\033[K\n")
	}
	b.WriteString("\033[J")
	out.Write(b.Bytes())
}
