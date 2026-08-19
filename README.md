# board

One screen showing every live coding-agent session running under cmux — Claude Code and
[maki](https://github.com/tontinton/maki), in one fleet.

![board watch: a session comes back with a question, a todo is captured, the quiet band folds away](docs/board.gif)

That is `board watch`. A session comes back asking something and lifts out of the quiet
tail, a todo is captured without leaving the tab, and `z` folds the backlog away.

The same fleet once, in plain text, for a pipe or a bug report:

```
$ board
STATE        LABEL                                        WORKSPACE            IDLE
blocked → ⚠  merge app#1497 before branching              APP                 3h00m
done ⚠       rotate the staging credentials               OPS                 3d02h
done ⚠       answer the ACME security questionnaire       DOCS                1d04h
done ⚠       backfill the events table                    DATA                7h20m
done ⚠       hunt the flaky payment test                  background          5h00m
done ⚠       fold the quiet band                          TOOLS               4h05m
done ⚠       Review pipeline PR #541                      REVIEWS             2h48m
done ⚠       ship the pricing page copy                   WEB                   55m
done         bump the staging image                       INFRA                 26m
done         migrate auth handlers to v2                  AUTH                   9m
running      build the csv export endpoint                API                    0m
todo         reply to the ACME csv export request                           12d ago
todo         book the quarterly review                                       1d ago

11 sessions · 1 blocked · 1 running · 8 quiet >45m · 2 todo
```

Both are a fixture, not a real fleet: every row on a real board is somebody's work, so
the demo builds a synthetic world and renders it through the same code (`demo/record.sh`,
`DESIGN.md` §16).

## Commands

    board                     print the screen once
    board watch [interval]    ambient dashboard, redraws in place (default 10s)
    board jump <substring>    focus the cmux tab whose label or workspace matches
    board label "<text>"      label the current session (no text clears it)
    board todo                list the todos, with ages and the cap
    board todo "<text>"       add one (max 10)
    board todo done <text|id> finish one, matched like jump
    board editor              which editor the ⧉ link opens (no name lists them)
    board install-hooks       wire both agents up: Claude Code hooks, and maki's init.lua
    board uninstall-hooks     take them back out, leaving every other hook alone
    board version             board's version, and claude's, cmux's and maki's
    board doctor              what board can and cannot read on this machine
    board -h                  this list, from the tool itself (`--help`, `help`)

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

| state | meaning | Claude Code | maki |
|---|---|---|---|
| `blocked →` | genuinely needs an answer | interactive `status: waiting`, or background `state: blocked` | `needs_input` |
| `running` | working | `status: busy` | `working` |
| `done` | finished its turn, unnoticed | everything else | `idle` |

`⚠` marks a quiet session past the idle threshold (default 45m).

Both agents land on the same three words, and **the screen does not say which agent a row
belongs to.** The action is the same either way — `Enter` focuses the tab — so a column
repeating the agent on every row would cost width and answer a question you do not have.
`board doctor` counts the two rosters separately, which is where the difference matters.

## Three links per row — `⧆`, `⧇` and `⧉`

At the right-hand end of a row, `board watch` puts up to three clickable glyphs:

    ○ migrate auth handlers to v2   ▇▇▇       9m  AUTH        ⧇ ⧉
    ○ ship the component library    ▇▇        4m  UI        ⧆ ⧇ ⧉
    ○ backfill the events table     ▇▇▇▇   7h20m  DATA          ⧉

- **`⧆` pink** — a **Storybook** listening in the session's worktree
- **`⧇` green** — the local **preview** serving that branch
- **`⧉` cyan** — its **folder**, the worktree, opened in your editor to see the branch's diff

They are ordered by how often a row has one, rarest first, so the folder anchors the right
edge and the gaps fall on the left.

Click them; they are terminal hyperlinks, so cmux opens the http ones in a browser tab beside
the fleet and hands the folder to your editor. **board itself opens nothing** — it only says
where the three things are.

Both are properties of the same thing, the **git worktree** the session is in. A session in
`.claude/worktrees/csv-export` gets that worktree's folder and that branch's preview; one in
the main checkout gets the repository and the preview running against `main`. A worktree lives
inside the main checkout on disk, so board resolves each side to its nearest `.git` rather than
comparing paths — otherwise a feature branch's dev server would show up on `main`'s row.

### `⧇` — the preview

`⧇` needs a dev server actually up, found through
[portless](https://www.npmjs.com/package/portless): board reads `~/.portless/routes.json` and
asks where each route's process is working. Run your dev server under portless and the link
appears on the matching row within a tick.

    $ cd ~/work/repo && portless run npm run dev
    🌐 https://repo.localhost

portless is optional, exactly as maki is — without it every row still carries its folder. Two
dev servers inside one worktree resolve to the one nearest the session's own directory, and a
row shows one preview, never a list.

### `⧆` — the Storybook

Storybook registers with nothing — no proxy, no state file, no roster command — so board finds
it by its port. It looks for a JS runtime listening anywhere in **6006 to 6020**, which is
Storybook's default and the range it walks into when each port is busy, and joins it to a row
the same way: same worktree.

The range is closed on purpose. TensorBoard defaults to 6006 and increments the same way, so an
open-ended `6006 and up` would eventually put a Storybook link on a row that has no Storybook —
and board pairs the range with a process check for exactly that reason. Fifteen ports is more
than anyone runs at once. The link is `http://localhost:<port>`; a Storybook you have put behind
TLS is one you have put behind portless, and it shows up as `⧇` instead.

Nothing to install and nothing to configure — run `npm run storybook` and the glyph appears on
the matching row within a tick.

### `⧉` — the folder, in your editor

Three editors work, because each registers a URL scheme that takes a path — which is what lets
board hand one over and run nothing:

| name | app | opens |
|---|---|---|
| `cursor` | Cursor | `cursor://file/…` |
| `vscode` | Visual Studio Code | `vscode://file/…` |
| `zed` | Zed | `zed://file/…` |

**You do not have to configure anything.** board uses the one you have installed, and when
several are installed it takes the first of the three above — alphabetically, deliberately,
because board has no opinion about which editor is better. `board editor` says what it picked
and what else is here:

    $ board editor
      folder links open  Cursor.app   cursor://   (automatically)

      → cursor   Cursor.app
        vscode   Visual Studio Code.app
        zed      not installed

      change it:  board editor vscode

`board editor zed` pins your choice in `~/.board.json`, and `board editor auto` hands it back.
A name board does not know is refused with the list, and a name whose app it cannot find is
still honoured — you may have it installed somewhere board does not look — with `zed, not
installed here` on the `board doctor` line so you can see why a click might go nowhere.

It is a command and not a prompt because **board never learns that you clicked a link.** The
terminal opens it and tells board nothing, so there is no "first time this was opened" to hang
an *open with* panel on; the question has to be answerable at any time instead. With none of
the three installed there is no `⧉` at all, rather than a glyph that opens nothing.

The links are in the dashboard only: plain `board` output goes into pipes and bug reports, and
a branch-derived hostname is work data. `board doctor`'s `links` row is where to look when a
glyph is missing.

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

Claude Code rows come from `claude agents --json`, which knows every live session. cmux
supplies tab titles, workspace names and the idle clock, joined on pid. Background agents
(`claude --bg`) appear with `background` in the workspace column — they have no tab,
so they cannot be jumped to, and their label is the open question Claude Code
records for them.

Interactive sessions with no cmux surface are subagents or sessions started outside
cmux; they are not rows, because this board is a view of tabs. The same rule applies to a
maki started outside cmux.

**Why `done` and not `waiting`:** cmux's `needsInput` lifecycle fires ~60 seconds
after *any* finished turn, so it means "sitting at the prompt", not "asked you
something" — on a typical fleet it covers most sessions. `blocked` is derived from
unresolved Feed cards instead, so it stays rare and worth reacting to.

**Cost:** `claude agents` takes ~250ms, so `board` runs in ~220ms rather than the
~60ms it managed when it derived state from cmux alone. That buys 10 sessions it
used to miss, including background agents blocked for weeks.

## maki

maki sessions are rows like any other — same three states, same bands, same `Enter` to
jump. Getting them takes one more step, because **maki has no roster command.** Its
sessions live inside the running process, and the only way in is maki's Lua plugin API.

So `board install-hooks` installs a plugin. It prepends a marked block to the `init.lua`
maki loads (`~/.config/maki/init.lua`, or `~/.maki/init.lua` if you have one) and writes a
`plugin.toml` beside it granting the block the two permissions it uses. From then on, every
time a session changes state maki writes what it knows to
`~/.board/maki/<cmux surface id>.json`, and board reads that.

    $ board install-hooks
    installed hooks: Stop, Notification
    installed maki hook: /Users/you/.config/maki/init.lua
    granted it fs_write and env: /Users/you/.config/maki/plugin.toml

Three things worth knowing about it:

- **The block is prepended, not appended.** A maki `init.lua` ends in `return { … }`, and
  in Lua nothing may follow a `return` — an appended hook would be a syntax error in the
  file that starts maki. Your own Lua survives verbatim below it, a timestamped
  `.board-bak-*` copy is taken first, and running it twice changes nothing.
- **`plugin.toml` is half the install.** maki denies every permission to an `init.lua`
  with no manifest beside it, so without that file the block installs and silently does
  nothing (`EVIDENCE.md` §9.32). If you already have one, board leaves it alone and tells
  you the block needs `fs_write` and `env` in it.
- **maki is optional.** Without it installed, nothing above happens and nothing complains
  — board reports on whichever agents are on the machine.

One maki process holds every session you have opened in its tab, so **one tab can be
several rows.** `Enter` on any of them focuses the tab; maki does not expose a
per-session jump to anything outside itself.

A maki row's idle clock is maki's own `updated_at` for that session, and its label is the
session title maki gave it — the same precedence claude rows follow (your label first, then
the agent's name for it, then the tab title, then the directory).

**maki not reporting · board doctor** on the screen means a maki is running in a tab and
board has no reports at all: run `board install-hooks`.

## Install

    go install github.com/zalts1/dashy/cmd/board@latest

Or from a clone:

    go build -o board ./cmd/board

Requires macOS, [cmux](https://github.com/manaflow-ai/cmux), and at least one of Claude
Code and [maki](https://github.com/tontinton/maki) — board reads its rows from the agents
and its tabs from cmux, so it has nothing to report without cmux and something to report on.
[portless](https://www.npmjs.com/package/portless) is optional and adds only the `⧇` link, one
of Cursor, VS Code or Zed is what `⧉` opens, and `⧆` needs nothing but a Storybook running.

**Verified against Claude Code 2.1.235, cmux 0.64.22 and maki 0.4.9** — the versions this
release was cut against. None of the three is a documented contract, so a patch release on
any of them can move what board reads; `board doctor` reports what is on your machine, and
a mismatch is the first thing to check.

No dependencies, no daemon, no port, no telemetry, and no network of its own — the only
request board can make is a `notify_cmd` you wrote yourself. `board watch` in a dedicated
tab is the whole runtime.

Substitute a tag for `@latest` to pin one — `@v0.2.0`. Either way the binary knows what
it is, with no build flags, and reports what it depends on beside it:

    $ board version
    board  v0.2.0
    claude 2.1.235 (Claude Code)
    cmux   0.64.22 (102) [ddd4a01bc]
    maki   0.4.9

**Quote all four in a bug report.** board reads its rows from the agents and its tabs from
cmux, and none of those surfaces is a documented contract — the three upstream lines are
usually the answer. A tool that could not be asked reads `not found`, which for maki is
the ordinary case rather than a problem; a board built outside a git tree reads `(devel)`,
which says how it was built rather than hiding it.

## When the board looks wrong — `board doctor`

    $ board doctor
    board  v0.2.0
    claude 2.1.235 (Claude Code)
    cmux   0.64.22 (102) [ddd4a01bc]
    maki   0.4.9
    roster 16 claude sessions · 3 maki sessions in 2 tabs
    tabs   22 tabs in 7 workspaces
    links  2 portless routes · 1 storybook · cursor
    hooks  Stop, Notification · maki init.lua
    config /Users/you/.board.json
    notify off — set notify_cmd to push

Ten lines saying what board can and cannot read here. `roster` is both agents' rosters —
`claude agents --json`, then what maki has reported — and `tabs` is cmux. **`16 claude
sessions` with `0 tabs` is the interesting failure**: board joins the two on pid and drops
any interactive session with no tab, so a fleet that looks too small usually means the tabs
did not arrive.

`roster` and `hooks` each carry both agents rather than each agent getting a row, and on a
machine without maki they say nothing about it — the version block above has already. The
maki halves worth recognising:

    roster 16 claude sessions · 2 maki running, no reports    ← install-hooks was never run
    hooks  Stop, Notification · maki init.lua without plugin.toml   ← installed and inert

`links` is the row to check when `↗` or `⧉` is missing from a row you expected it on, and it
carries both halves — the preview read, then the editor the folder opens. The preview half has
the same two-halves shape `roster` does, because `routes.json` outlives the dev servers it
names:

    links  no portless — rows link to folders only · cursor    ← portless is not installed
    links  portless installed, no routes up · cursor           ← nothing is running
    links  3 portless routes, none live · cursor               ← stale file; no row gets ⧇
    links  1 portless route · no editor — board editor         ← no ⧉ on any row
    links  1 portless route · zed, not installed here          ← ⧉ is drawn and may miss
    links  1 route · 2 storybook ports outside every worktree · cursor   ← no ⧆ on any row

The storybook clause appears only when something is listening in the range, since board scans
it whether or not you use Storybook.

It reads and never writes, so it works on a machine where `install-hooks` refuses. It
**never prints `notify_cmd`**, only whether one is set: this output is meant to be
pasted, and that field is a shell command that often carries a webhook URL.

An empty board and an unreadable one are different reports, and the screen says which:

    claude not found · board doctor

That line takes the header's own slot in `board watch` and leads the one-shot table, in
place of the session count. `no sessions` means board looked and the fleet is quiet.

It reports the most fundamental fact it has room for, and there is an order: the claude
roster, then cmux, then maki. A missing `claude` stops being reported once maki is
reporting — board has a fleet on screen, and the tool you did not install was a choice.

## Config — `~/.board.json`

```json
{
  "config": {
    "idle_threshold_minutes": 45,
    "poll_seconds": 10,
    "notify_cmd": "curl -sS -d @- https://ntfy.sh/my-topic",
    "editor": "cursor"
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
name. Sink failures are swallowed and the sink gets **3 seconds**, so neither a broken
webhook nor a hanging one blocks an agent: `notify` runs inside the agent's own hook
chain, where a command that waits costs the same as a command that fails.

## Uninstall

    board uninstall-hooks

Removes board's `Stop` and `Notification` entries from `~/.claude/settings.json` and
nothing else — other tools' hooks, other settings, and the file's permissions all
survive. A timestamped `.board-bak-*` copy is written first, an existing backup is never
overwritten, and running it twice is a report rather than an error. It refuses a
settings file it cannot parse, exactly as `install-hooks` does: board cannot safely edit
what it cannot read.

On the maki side it takes the block back out of `init.lua`, byte for byte, and clears
`~/.board/maki`. The `plugin.toml` goes only while it is still the one board wrote — if you
have edited it, it is yours and it stays. An `init.lua` left holding nothing but whitespace
is removed outright, since board is the one that created it.

Then delete `~/.board.json` and the binary. **Labels and todos live only in
`~/.board.json`**, so copy anything on the list out first; nothing else is written.
