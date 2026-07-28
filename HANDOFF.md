# board — handoff

Entry point for a fresh session. `DESIGN.md` holds the full reasoning per section;
this file is the map and the traps.

Repo: `~/workspace/sys-tools/dashy` (dir is named `dashy`, the tool is `board`).
Installed at `~/.local/bin/board`. Go, stdlib only, ~1460 lines across 5 files.

## What it does today

    board                     one-shot table of every live session
    board watch [interval]    ambient dashboard for a dedicated cmux tab (default 10s)
    board jump <substring>    focus a session's cmux tab from anywhere
    board label "<text>"      label the current session
    board install-hooks       merge Stop + Notification hooks (installed, currently inert)

Inside `watch`: `↑↓`/`kj` select, `Enter` focuses that tab and keeps board running,
`Esc` clears, `q`/`Ctrl-C` exits. Bands are `NEEDS YOU` / `WORKING` / `QUIET` /
`ASKED`. State lives in `~/.board.json` (labels + config).

Current fleet for scale: 31 sessions, 3 blocked, 21 quiet past the 4h threshold,
oldest 52 days.

## Where the data comes from

| what | source |
|---|---|
| roster + state | `claude agents --json` — interactive `status` idle/busy/waiting, background `state` blocked |
| tab title, workspace, surface UUID | `cmux --id-format both top --all --json`, joined on **pid** |
| idle clock | cmux `~/.cmuxterm/claude-hook-sessions.json` → `updatedAt`, else transcript mtime |
| background agent's open question | `~/.claude/jobs/<shortId>/state.json` → `needs` |
| the ledger | tail of `~/.cmuxterm/workstream.jsonl` |

## Traps that already cost time — do not rediscover these

- **`toolResult` is not the answer to a question.** cmux's Feed bridge emits one when
  its ~6s semaphore expires; Claude then falls back to its own picker and waits
  indefinitely, logging nothing. Two blocked-detection rules died on this. State now
  comes from `claude agents` instead. (§12, §15)
- **cmux's `needsInput` means "sitting at the prompt", not "asked you something".**
  It fires ~60s after any finished turn and covered 16 of 21 sessions. Never surface
  it as `waiting`.
- **cmux treats `CMUX_SURFACE_ID`/`CMUX_WORKSPACE_ID` as the implicit target of every
  command.** A stale value makes even a global query fail. `run()` strips them.
- **`surface.focus` takes `surface_id`**, not `surface`. It accepts UUIDs or refs.
- **cmux's hook file misses live sessions.** 8 real tabs were absent from it. Never
  use it as the roster — enrichment only.
- **`/dev/null` is a character device**, so a `ModeCharDevice` TTY check made
  `board watch >/dev/null` spin forever. Detection asks the kernel for a window size.
- **Bare `#d03b3b` fails contrast (2.91) on the default terminal background.** Blocked
  renders as a filled badge whose white text measures 4.80 against its own fill, so it
  is theme-independent. Don't turn it back into coloured text. (§11)
- **The idle ramp brightens with age.** On a dark surface the sequential anchor flips,
  and steps must skip every other rung or adjacent ΔL falls under 0.06.
- **Jump must not exit the process.** It focuses another tab and keeps looping; the
  board tab stays alive. Errors render in the frame header, because once you jump the
  board tab is not the visible one.

## Decisions already made — don't relitigate without new evidence

- **Nothing closes a session automatically**, ever. Manual `close` is gated behind
  `enable_close_action`, false by default. Accepted consequence: `QUIET` only grows. (§8)
- **No sorting, no filtering.** The bands are the sort; hiding rows defeats the tool.
- **Push notifications parked.** Hooks installed but inert (`notify_cmd` empty). Slack
  sink built, reverted, recoverable at commit `1f24c00`. If revisited: gate on keyboard
  idle, not on events. (§10)
- **No spawning or orchestrating sessions** — attacks the wrong side of the bottleneck.
- **Todo model is derived-from-sessions plus "intents"**, where an intent is a session
  that hasn't started yet: text and age only, no status or priority. (§2)
- **Ledger is questions and plan exits only**; auto-approved permissions resolve
  without a human and are volume, not accountability. (§13)
- **Don't rebuild what cmux or Claude Code already ship** — cmux's Feed, and
  `claude agents`' peek/reply/attach. Jump-to-tab is the real gap, because attach
  opens a transcript and does not bring the cmux tab forward. (§14)

## Open items, in priority order

1. **Reshape the original constraints (§16).** Line cap, latency budget, and the
   no-TUI rule are all overtaken. The user wants to do this together — ask what
   failure each constraint was protecting against.
2. **Intents/todos (§2) are blocked on evidence.** `~/.board.json` holds **1 label**.
   The derived-todo model assumes labelling happens. Don't build on it yet.
3. `board` costs ~230ms against a 200ms brief. Caching the roster would fix the
   one-shot; irrelevant for the ambient tab.
4. 8 interactive sessions with no cmux surface are deliberately excluded as subagents.
   Revisit if any turn out to be real work.

## Verifying changes

    go test ./...                          # frame layout, navigation, selection
    board watch > /tmp/f.txt               # piped = one frame, no TTY needed
    LINES=24 COLUMNS=100 board watch       # exercise collapse and narrow widths

`frame()` is a pure function of a `fleet`, so render bugs are testable without a
terminal. Colour must be validated, not eyeballed — the dataviz skill's
`validate_palette.js` was used for every value in `watch.go`; re-run it for any change.
