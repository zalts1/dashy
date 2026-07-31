# board

One screen showing every live Claude Code session running under cmux.

```
$ board
STATE        LABEL                                        WORKSPACE           IDLE
blocked → ⚠  merge app#1497 before branching              APP                3h00m
blocked →    migrate auth handlers to v2                  AUTH                  9m
done ⚠       Review pipeline PR #541                      REVIEWS            2h48m
running      build board: cmux fleet status CLI           INFRA                 0m

22 sessions · 2 blocked · 1 running · 20 quiet >45m
```

## Commands

    board                     print the screen once
    board watch [interval]    ambient dashboard, redraws in place (default 10s)
    board jump <substring>    focus the cmux tab whose label or workspace matches
    board label "<text>"      label the current session (no text clears it)
    board todo                list the todos, with ages and the cap
    board todo "<text>"       add one (max 10)
    board todo done <text|id> finish one, matched like jump
    board install-hooks       merge Stop + Notification hooks into ~/.claude/settings.json
    board version             board's version, and claude's and cmux's
    board doctor              what board can and cannot read on this machine

`board watch` is meant to live in its own cmux tab. It needs no daemon and opens no
port — the tab is the process. Piped or redirected it prints one frame and exits,
so `board watch > frame.txt` works.

Inside `board watch`: `↑`/`↓` (or `k`/`j`) select a row, `Enter` focuses that
session's cmux tab **and leaves board running**, `a` adds a todo, `t` jumps to the top
of the list, `d` finishes the selected one, `z` folds the quiet band to its count,
`Esc` clears, `q` or `Ctrl-C` exits. That is the whole keymap — no
sorting, no filtering.

`z` is the one thing that changes what the frame shows rather than where the cursor is.
QUIET starts **open**; folded, it keeps its header and its count — `QUIET · 13 ·
collapsed` — so the backlog can be out of the way without being out of sight, and the
rows it gives back go to the bands where something is happening. Folded rows are not
selectable, because `↑`/`↓` follow the screen.

The letter keys also work on the Hebrew layout, on the same physical keys — `א` for the
list, `ש` to add, `ג` to finish, `ז` to fold, `ח`/`ל` to move. `q` is the exception: quit
from a non-Latin layout with `Ctrl-C`.

`a` opens a prompt on the bottom line: type, `Enter` adds, `Esc` cancels, and empty text
is a cancel. It is the one mode board has, and it stays deliberately dumb — no cursor
keys, just typing and backspace. **Nothing times out while you type**; the selection's
10s return does not apply, because a timer that discarded half-typed text would be worse
than a paused tab. `Ctrl-C` still exits from inside the prompt.

When QUIET does not fit, the band ends with `+N quiet`. **That is a count, not a
control** — nothing expands a band. To read a hidden row, press `↓` past the last
visible one (it walks into the collapsed rows one at a time, rottenest first) or make
the tab taller. Rows with no cmux tab can be selected and read too; `Enter` on one
says there is nowhere to jump.

**While a row is selected the refresh pauses** and the header says so. Rows re-sort
as idle time grows, so a live refresh would slide the list out from under the
cursor. The selection is tracked by session, not by screen line, and clears itself
after 10s of no keypress so the tab always returns to being ambient.

## States

| state | meaning | derived from |
|---|---|---|
| `blocked →` | genuinely needs an answer | `claude agents`: interactive `status: waiting`, or background `state: blocked` |
| `running` | working | `claude agents`: `status: busy` |
| `done` | finished its turn, unnoticed | everything else |

`⚠` marks a quiet session past the idle threshold (default 45m).

## Todos

A todo is work with **no session behind it** — the customer request you have not
started, so there is no tab for board to find. It carries text and an age and nothing
else: no status, no priority, no due date.

    TODO · 3 of 10
             ▫ reply to the ACME csv export request               12d ago
             ▫ review the export PR                                1d ago
             ▫ book the quarterly review                              now

With nothing on it the band still appears, as `TODO  nothing on your list` — an unused
feature that renders nothing is one you never find. The list is drawn last, after the
whole fleet, because it is not a state a session can be in. It has **no bar and no workspace**: a bar means idle time, which is a gap that
resets when a session is touched, while a todo's age is a lifetime that only grows —
hence `12d ago` rather than `12d00h`. The header states the cap so you can watch it
climb.

Press `a` inside `board watch` to add one without leaving the tab, or from any shell:

    $ board todo "reply to the ACME csv export request"
    todo: reply to the ACME csv export request  (1 of 10)

**The list holds 10, and the 11th is refused rather than trimmed.** That is the point,
not a limitation: a list that can grow without end is a backlog, and a backlog belongs
in your issue tracker. The refusal is what makes you decide.

Nothing links a todo to a session yet, so
if you start the work, remove the todo — board will not notice the overlap for you.
There is no undo: the header reports the text it removed, so a mis-key costs one line
of retyping.

Rows come from `claude agents --json`, which knows every live session. cmux supplies
tab titles, workspace names and the idle clock, joined on pid. Background agents
(`claude --bg`) appear with `background` in the workspace column — they have no tab,
so they cannot be jumped to, and their label is the open question Claude Code
records for them.

Interactive sessions with no cmux surface are subagents or sessions started outside
cmux; they are not rows, because this board is a view of tabs.

**Why `done` and not `waiting`:** cmux's `needsInput` lifecycle fires ~60 seconds
after *any* finished turn, so it means "sitting at the prompt", not "asked you
something" — on a typical fleet it covers most sessions. `blocked` is derived from
unresolved Feed cards instead, so it stays rare and worth reacting to.

**Cost:** `claude agents` takes ~250ms, so `board` runs in ~220ms rather than the
~60ms it managed when it derived state from cmux alone. That buys 10 sessions it
used to miss, including background agents blocked for weeks.

## Install

    go install github.com/zalts1/dashy/cmd/board@latest

Or from a clone:

    go build -o board ./cmd/board

Requires macOS, [cmux](https://github.com/manaflow-ai/cmux), and Claude Code — board
reads the roster from `claude agents --json` and the tabs from cmux, so it has nothing
to report without both.

No dependencies, no daemon, no port. `board watch` in a dedicated tab is the whole
runtime.

Substitute a tag for `@latest` to pin one — `@v0.1.0`. Either way the binary knows what
it is, with no build flags, and reports what it depends on beside it:

    $ board version
    board  v0.1.0
    claude 2.1.220 (Claude Code)
    cmux   0.64.16 (96) [5321becb6]

**Quote all three in a bug report.** board reads its roster from `claude agents --json`
and its tabs from cmux, and neither is a documented contract — the two upstream lines
are usually the answer. A tool that could not be asked reads `not found`; a board built
outside a git tree reads `(devel)`, which says how it was built rather than hiding it.

## When the board looks wrong — `board doctor`

    $ board doctor
    board  v0.1.0
    claude 2.1.220 (Claude Code)
    cmux   0.64.16 (96) [5321becb6]
    roster 16 sessions
    tabs   22 tabs in 7 workspaces
    hooks  Stop, Notification
    config /Users/you/.board.json
    notify off — set notify_cmd to push

Eight lines saying what board can and cannot read here. `roster` is `claude agents
--json`; `tabs` is cmux. **`16 sessions` with `0 tabs` is the interesting failure** —
board joins the two on pid and drops any interactive session with no tab, so a fleet
that looks too small usually means the tabs did not arrive.

It reads and never writes, so it works on a machine where `install-hooks` refuses. It
**never prints `notify_cmd`**, only whether one is set: this output is meant to be
pasted, and that field is a shell command that often carries a webhook URL.

An empty board and an unreadable one are different reports, and the screen says which:

    claude not found · board doctor

That line takes the header's own slot in `board watch` and leads the one-shot table, in
place of the session count. `no sessions` means board looked and the fleet is quiet.

## Config — `~/.board.json`

```json
{
  "config": {
    "idle_threshold_minutes": 45,
    "poll_seconds": 10,
    "notify_cmd": "curl -sS -d @- https://ntfy.sh/my-topic"
  },
  "labels": { "<cmux surface id>": "<label>" },
  "todos": [
    { "id": "2483b5", "text": "reply to the ACME csv export request", "created": "2026-07-29T13:49:09+03:00" }
  ]
}
```

`notify_cmd` runs via `sh -c` on every `Stop` and `Notification` hook and receives
this JSON on stdin. Empty (the default) means notifications are off.

```json
{
  "event": "Notification",
  "state": "needs input",
  "label": "migrate auth handlers to v2",
  "workspace": "AUTH",
  "surface_id": "D39C7A0C-…",
  "cwd": "/Users/you/work/repo",
  "message": "Claude needs your permission to use Bash",
  "text": "needs input — migrate auth handlers to v2 [AUTH]: Claude needs your permission to use Bash"
}
```

Use `.text` for a ready-made one-liner; it always carries the label and workspace
name. Sink failures are swallowed so a broken webhook never blocks an agent.

## Uninstall

Remove the two `board notify` entries from `~/.claude/settings.json` (a timestamped
`.board-bak-*` copy is written before the first change), then delete `~/.board.json`
and the binary. Labels and todos live only in `~/.board.json`, so copy anything on the
list out first; nothing else is written.
