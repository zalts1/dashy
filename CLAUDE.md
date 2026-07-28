# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`board` is a single-package Go CLI (module `board`, no third-party dependencies) that
reports on every live Claude Code session running under cmux. `README.md` is the user
contract; `DESIGN.md` is the decision record — read the relevant section before
changing behaviour it covers.

## Commands

```sh
go build -o board .          # the binary is gitignored; build in place
go vet ./...
go test ./...                # run all tests
go test -run TestFrame ./...  # run a single test (regex on test name)
go test -run 'TestAsks/never_answered' ./...  # single subtest

./board                      # one-shot table
./board watch 5s             # ambient dashboard (alternate screen, ctrl-c exits)
COLUMNS=118 LINES=44 ./board watch | cat   # non-TTY: one frame to stdout, then exit
```

That last form is the main manual verification path — `watch` detects a non-TTY via
`TIOCGWINSZ` and prints a single frame, so layout and colour can be diffed without a
terminal to drive.

## TDD is mandatory

Every behaviour change starts with a failing test. No exceptions, including for
"obvious" one-liners.

1. Write the test first; run it; confirm it fails for the reason you expect.
2. Write the minimum code to pass.
3. Refactor with the test green.

This matters more here than usual because of `DESIGN.md` §12: three successive
blocked-detection rules all looked correct in the code and all were wrong against
real log data. Derived state must be pinned by a test over a fixture, not by
reading the function.

Test the pure seams — they exist for this reason and must stay pure:

| Function | File | Fixture shape |
|---|---|---|
| `frame(fleet, now, interval, thresh, rows, cols)` | `watch.go` | a `fleet` literal; assert on the returned string |
| `blockedSessions(events, want)` | `main.go` | `[]event` sequences |
| `asks(events, since, wsOf)` | `ledger.go` | `[]event` with `line` set to real JSONL bytes |
| `event.settles` / `isActionable` | `ledger.go` | one event |
| `idleScale`, `bar`, `humanize`, `short`, `pad`, `stripSpinner` | — | values |

`readSessions`, `readTail`, and `cmuxTitles` touch `$HOME` and the `cmux` binary —
keep logic out of them and test the parsers they feed instead. Never write a test
that depends on the live fleet; capture JSONL lines into a fixture.

## Architecture

Four inputs, all read-only:

- `~/.cmuxterm/claude-hook-sessions.json` — lifecycle, cwd, pid, `updatedAt`. Liveness
  is `syscall.Kill(pid, 0)`; one session per `surfaceId`, newest `updatedAt` wins.
- `~/.cmuxterm/workstream.jsonl` — the audit log. Only the last 8MB (`tailBytes`,
  ~2 days) is read, scanned with `jsonField` string search rather than full JSON
  decode; payloads are decoded only for the few rows that need it.
- `cmux top --all --json` — surface and workspace titles, joined to sessions **on pid**
  (surface nodes carry no UUID). Always go through the `cmux()` helper: it blanks
  `CMUX_SURFACE_ID`/`CMUX_WORKSPACE_ID`/`CMUX_TAB_ID`/`CMUX_PANEL_ID`, because cmux
  treats an inherited value as the implicit target and a stale one fails global queries.
- `~/.board.json` — the only file written: config plus `surfaceId → label`.

`collect()` in `main.go` produces one `fleet` snapshot — rows, counts, and the ledger —
from a single pass over the log tail. Both renderers consume it (`show()` for the plain
table, `frame()` for the dashboard) so the two can never disagree about state. Add a
derived quantity to `fleet`, not to a renderer.

`row.rank` is the sort key and the band selector: `0` blocked, `1` quiet/done, `2`
running. Within a rank, oldest first.

### Blocked detection (`DESIGN.md` §12)

cmux logs an actionable card and its own `toolUse` in the same second, so "newest event
is a card" never fires. The shipped rule walks each session's events backwards and takes
the first event that *settles* the question: `toolResult` is **inconclusive and skipped**
(cmux's Feed bridge emits one when its ~6s semaphore expires while Claude keeps waiting
at its own picker); a `stop`, `userPrompt`, or non-actionable `toolUse` proves the agent
moved on. This lives in `blockedSessions` and `event.settles` — do not "simplify" it back.

`done` rather than `waiting` is deliberate: cmux's `needsInput` lifecycle fires ~60s
after *any* finished turn, so it means "sitting at the prompt", not "asked you something".

### Ledger (`ledger.go`, `DESIGN.md` §13)

`asks()` extracts `question` and `exitPlan` episodes only — auto-approved permission
requests resolve without a human, so they are volume, not accountability. An episode with
no settling event is `open` and renders as `never`. `coverage()` reports the oldest event
actually in the tail so the header states the real window instead of the requested 7 days.

### Rendering (`watch.go`)

`frame()` is a pure function of the snapshot; `watch()` is the only impure part
(alternate screen, ticker, SIGWINCH). Redraw is cursor-home + per-line `\033[K` written
as one buffer — a full-screen clear each tick flashes, so never blank the previous frame
before the new one is ready.

Colour values are validated against both the lightest and darkest plausible terminal
backgrounds (`#282c34`, `#040404`) per `DESIGN.md` §11 — **do not substitute by eye or
add a colour without re-validating.** Two results are load-bearing: bare `#d03b3b` text
fails at 2.91, which is why blocked is a filled badge with white text (theme-independent);
and the idle ramp *brightens* with age because the sequential anchor flips on a dark
surface.

Bar length and ramp step both encode idle on one shared absolute log scale (0→7d) so bars
stay comparable across refreshes. Bars appear on blocked and quiet rows — time you owe a
session — and never on working rows, where elapsed time is progress.

## Invariants

- **board never ends a session.** No close, kill, or hibernate on a timer, threshold, or
  batch path (`DESIGN.md` §8). It is a reporting surface; that is what makes it safe to
  leave installed.
- **`notify` must never fail the agent.** It is a Claude Code hook; every error path is a
  silent success, including a broken `notify_cmd`.
- **`install-hooks` refuses to rewrite unparseable `~/.claude/settings.json`** and writes
  a timestamped `.board-bak-*` before its first change. It is idempotent via the
  `<binary> notify` command marker.
- Zero config to first value, and no persistent process, port, or daemon — the watch tab
  *is* the process (`DESIGN.md` §5, §11).
- No dependencies. Reach for `stdlib` + `syscall` (`TIOCGWINSZ`, `Kill`) rather than a
  TUI or colour library.

When a change settles a design question or falsifies a recorded conclusion, append to
`DESIGN.md` the way §12 and §13 do — state the wrong belief, the evidence, and the
shipped rule.
