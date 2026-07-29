# board — design record

**Status: settled.** Every question this document used to leave open is decided
below. It is now a record, not a proposal: read the section that covers what you are
about to change, and if a change settles something new or falsifies something here,
append to §9 (evidence, in `EVIDENCE.md`) or §10 (deferred, below) in the same style.

**Where the sections live.** §1–§8 and §10–§11 are here; **§9.x is in `EVIDENCE.md`**,
the append-only log of what was believed and what falsified it. § numbers are bare
throughout both files and are stable — they are cited from 25 code comments, so they
are never renumbered.

**Read one section, not the file.** This document is ~500 lines; a section is ~30.

```sh
grep -n '^#\+ ' DESIGN.md        # § → line, then Read with offset/limit
```

## Index

| § | settles | read it before changing |
|---|---|---|
| 1 | what board is, and the four non-goals | scope — any new feature or command |
| 2 | the original constraints, each kept, restated or retired | anything that trades one away |
| 3 | four inputs, one file written, pid as join key, the package tree | `board/`, `host/`, adding a source |
| 4 | state model: `blocked`/`running`/`done`, staleness, label precedence | `board/build.go`, `claude/` |
| 5 | why the `ASKED` ledger was removed and cannot come back | proposing any new band |
| 6 | rendering: form, validated colour, encoding, width, header, behaviour | all of `view/` |
| 7 | interaction: jump, keymap, identity-not-position, the explicit pause | `watch/`, `view/order.go`, `cmux/focus.go` |
| 8 | safety invariants — nothing ends a session | any write action |
| 9 | **evidence log — in `EVIDENCE.md`** | the finding that constrains the code you are touching |
| 10 | deferred ideas, each with the trigger that would revive it | before building one of them |
| 11 | the pure seams and their fixture shapes | writing any test |

---

## 1. What board is

One screen showing every live Claude Code session running under cmux, ranked by
whether it needs you.

The bottleneck it addresses is **attention, not capacity**. With ~30 concurrent
sessions the failure is not that too few agents are running; it is that two of them
have been blocked on a question for a day and nothing surfaces that. So board
optimises for one thing: *where do I look first*.

**It is a reporting surface.** It reads four sources, writes one file, and takes
exactly two actions on the world (focus a tab, set a label). That is what makes it
safe to leave installed, and it is the property to protect when adding features.

Non-goals, all decided and not open for relitigation without new evidence:

- **Not a task manager and not a kanban.** Nothing moves through stages — a session
  is blocked, working, or rotting. Structure belongs in Linear, which already exists.
- **No spawning or orchestration.** Making it easier to run *more* agents attacks
  the wrong side of the bottleneck.
- **No rebuilding what cmux or Claude Code already ship.** Not cmux's Feed, not
  `claude agents`' peek/reply/attach. Jump-to-tab is the real gap (§7).
- **No sorting and no filtering.** The bands are the sort. Hiding rows is the same
  failure as muting a notification (§8).

---

## 2. Constraints, settled

The original brief set hard caps. Each is recorded here with the failure it was
protecting against, because that is what determines whether it still applies.

| constraint | protects against | verdict |
|---|---|---|
| no network listeners, no ports, no daemon | a work laptop, and "infrastructure gets muted" | **keep — hard** |
| nothing ends a session automatically | irreversible loss of work (§8) | **keep — hard** |
| no telemetry, no network egress by default | same | **keep — hard** |
| all state in one JSON file under `$HOME` | state sprawl, uninstall debt | **keep** |
| no dependencies — stdlib and `syscall` only | supply chain, and build rot on a locked-down machine | **keep** |
| zero config to first value | abandonment on setup friction | **keep** |
| no spawning or orchestration | attacking the wrong side of the bottleneck | **keep** |
| not a task manager, no kanban | scope creep into Linear's job | **keep** |
| ~300 lines | scope creep | **restated** — see below |
| under 200ms | felt latency on a one-shot command | **restated** — see below |
| three actions, total | modes and menus to learn | **restated** — two actions |
| no TUI | building something you must keep open | **retired** — see below |
| three-week trial before v2 | shipping on speculation | **retired**, superseded |
| notification fan-out in v1 | missing work while away | **parked**, mechanism inert (§10.1) |

**"~300 lines" is restated, not deleted.** Scope did grow, deliberately and with
reasons recorded. A line cap was a proxy for "don't let this become a second
product", and the proxy stopped tracking the thing once the dashboard and ledger
landed on purpose. The restatement, which is checkable:

- no third-party dependencies, one binary, no daemon;
- one job per package and one concern per file; a file past ~250 lines is a signal to
  split;
- a new **command** or a new **band** needs an entry in this document first;
- the pure seams stay pure and stay tested (§11).

**"Under 200ms" is restated as 400ms for the one-shot and 1s for a watch tick.**
`board` costs ~220ms, and ~250ms of any tick is `claude agents`, which is not ours to
optimise. The budget existed so the command felt instant when typed; missing a
session blocked for 44 days is a far worse failure than a 20ms overshoot. The
ambient tab is the primary surface anyway, where 220ms per 10s is a 2% duty cycle.

**"No TUI" is retired in letter, kept in spirit.** The ambient dashboard *is* a TUI
and was asked for directly. What the constraint was actually protecting — no modes,
nothing to learn, and never *requiring* the app to be open — is still enforced: the
keymap is five keys, ctrl-c exits, and the one-shot table stays first-class so board
is useful without a dedicated tab.

**"Three actions" is now two: jump and label.** Dismiss was rejected (§8), intents
were never built (§10.2). A third action needs an entry here.

---

## 3. Architecture

Four inputs, all read-only. One file written.

| what | source | cost |
|---|---|---|
| roster + state | `claude agents --json` — interactive `status` idle/busy/waiting, background `state` blocked | ~250ms |
| tab title, workspace, surface UUID | `cmux --id-format both top --all --json`, joined on **pid** | ~50ms |
| idle clock | cmux `~/.cmuxterm/claude-hook-sessions.json` → `updatedAt`, else transcript mtime | free |
| a background agent's open question | `~/.claude/jobs/<shortId>/state.json` → `needs` | free |
| labels + config | `~/.board.json` — **the only file board writes** | free |

`~/.cmuxterm/workstream.jsonl` was a fifth source, read 8MB deep every tick to feed
the `ASKED` ledger. Both are gone — §9.13.

The two subprocess calls are independent and run concurrently, so a tick costs
~250ms rather than ~300.

**pid is the join key.** cmux surface nodes carry no session UUID, and
`cmux_process_pids` is 1:1 with the agent process. The tree is walked rather than
indexed because its nesting has changed before.

### Packages

The tree is organised so that the parts that were historically wrong are the parts
that are pure and tested. One package per source, one domain package, and the
renderer split from the loop that drives it. Logic in `cmd/board/` is untestable
logic, so that layer is dispatch and printing only.

**The tree itself is not reproduced here.** `go doc ./...` prints every package's job
from the package comments beside the code, which cannot drift from it; `CLAUDE.md` has
the same map as a routing table. This section is why the shape is what it is.

Two boundaries are load-bearing:

- **`board.Build(Snapshot, now) Fleet` is pure.** Every join rule — roster
  membership, label precedence, rank, staleness — is a function
  of data passed in. `Collect` does the reading and nothing else. The rules that
  shipped wrong twice (§9.1, §9.3) all lived in this join and were untestable when
  they did.
- **`view` never reads the world and `watch` never formats.** `Frame` is a pure
  function of a snapshot plus a terminal size, which is why a golden file can pin
  the entire screen (§11).

`Row.Rank` is both the sort key and the band selector: `0` blocked, `1` quiet/done,
`2` working. Within a rank, oldest first. Add a derived quantity to `Fleet`, not to a
renderer — both renderers consume the same snapshot so they can never disagree.

---

## 4. State model

| state | meaning | source |
|---|---|---|
| `blocked →` | genuinely needs an answer | interactive `status: waiting`, or background `state: blocked` |
| `running` | working | `status: busy` |
| `done` | finished its turn, unnoticed | everything else |

`⚠` marks a quiet session past the idle threshold (default 45m). A **working session
is never stale**: elapsed time there is progress, not rot.

**`done`, never `waiting`.** cmux's `needsInput` lifecycle fires ~60s after *any*
finished turn, so it means "sitting at the prompt", not "asked you something" — it
covered 16 of 21 sessions when it was tried (§9.2).

Roster membership, both deliberate:

- **Interactive sessions with no cmux surface are excluded.** They are subagents or
  sessions started outside cmux. A row you cannot jump to is a row you cannot act
  on. Documented rather than silent.
- **Background agents are included even though they cannot be jumped to**, marked
  `background` in the workspace column. Hiding blocked work to keep the table tidy
  would invert the point of the tool. Their label is Claude Code's own `needs`
  string, which beats a slug: "watch CI to green, or leave it here?"

Label precedence: user label → Claude's `needs`/name → cmux tab title → cwd
basename. Labels are keyed on **surface id**, so they survive a session ending and
being resumed in the same tab.

---

## 5. The ledger (`ASKED`) — removed

**There is no ledger.** The band, `internal/workstream`, and the audit-log read were
all deleted on 2026-07-28; §9.13 is the evidence and §10.6 is the trigger that would
revive it. The argument, kept because it is the one that generalises:

`NEEDS YOU` already answers "what is asking me something" — a blocked row carries the
question as its label and a key to jump to it. A ledger row can therefore only be one
of three things, and none of them is actionable:

| the row | can you act on it? |
|---|---|
| an answered ask | no — it is history, the decision is already made |
| an open ask, session alive | it is **already in `NEEDS YOU`**, where it can be jumped to |
| an open ask, session dead | no — there is nothing left to answer |

So the band was structurally incapable of earning its lines, whatever the filter. The
premise it was built on ("nothing answers what asked me today") was a real gap in
cmux's Feed, but board closed the *live* half of it in `NEEDS YOU`, and the historical
half has no action attached to it.

What the ledger did teach is kept: **`toolResult` is not an answer** (§9.1), and the
`needs` string is the best label a blocked background agent has (§4).

---

## 6. Rendering

### Form

Per the data-viz method: >7 classes that all carry meaning is **a table, not a
chart**, and the right form for "one thing matters, the rest are context" is
**emphasis** — one accent, everything else recessive. So: a table, three bands
(`NEEDS YOU` / `WORKING` / `QUIET`) plus `ASKED`, with exactly one element allowed
to shout.

Two layers, so a longer look yields more without touching anything:

- **Glance** — the BLOCKED badge, the KPI strip, and the point where the amber ⚠
  column stops. That waterline *is* the threshold, read positionally.
- **Detail** — labels, bar lengths, exact durations, workspaces, all in recessive
  ink so they don't compete for the first half-second.

### Colour — validated, not chosen

Every value cleared the contrast checks against both the lightest and darkest
plausible terminal backgrounds (`#282c34`, `#040404`). **Do not substitute by eye and
do not add a colour without re-validating.**

| role | value | result |
|---|---|---|
| blocked badge | white on `#d03b3b` | 4.80 against its own fill — theme-independent |
| running | `#0ca30c` | ≥3:1 mark on every candidate background |
| stale ⚠ | `#fab219` | 7.63–11.18 |
| body / dim ink | `#c3c2b7` / `#898781` | 7.81 / 3.90+ |
| idle ramp, 5 steps | `#256abf` → `#cde2fb` | all four ordinal checks pass in both extremes |

Two findings are load-bearing and are pinned by tests (§9.4).

### Encoding rules

- Bar length **and** ramp step both encode idle on one shared **absolute log scale**
  (0→7d). Absolute so bars stay comparable between refreshes; log because linear
  would flatten everything under a day into one cell.
- The bar appears on blocked and quiet rows — both are "time you have owed this
  session attention" — and **not** on working rows.
- **No bars in `ASKED`.** Waits are minutes against a scale topping out at a week,
  so every row rendered a single cell and meant nothing. `never` in amber is the only
  thing in that band that earns colour.
- A value scale without a key is decoration, so the ramp legend is drawn wherever the
  bars are — and **only** there. A key for a scale that is not on screen is noise, so
  the narrow layout drops both together.

### Width, at any size (`view/layout.go`, 2026-07-28)

The frame is elastic in both directions, and for the same reason: **a line wider than
the terminal wraps, and a wrapped line makes the frame occupy more screen rows than
`height()` counted** — the fit loop then under-reports, the terminal scrolls, and the
header goes first (§9.10). Width is therefore not a cosmetic question; it is the same
invariant as height. It was broken for the whole life of the tool (§9.12).

- **Chrome is derived, never guessed.** `rowChrome` is built from the pieces it is made
  of — lead, gutter, bar, warn mark, IDLE, gaps — so it cannot drift out of step with
  the layout the way the fixed reserve of `46` did.
- **Two elastic columns, sized to content**: the label, and the tail (workspace on a
  row, `HH:MM where` in ASKED). The tail was previously unbounded, which is what
  overflowed: workspace names are user data. Squeeze order is meaning first — the label
  shrinks to its floor, then the tail truncates. A resize moves those two and leaves
  every other column where it was.
- **A cut bar is worse than no bar**: the same glyph run reporting a smaller number, on
  a scale that is supposed to be absolute. Below `rowChrome + minLabelW` the bar column
  is dropped whole (`rowChromeBare`), which also buys back the 13 columns that make the
  label and IDLE fit on a narrow tab.
- **Cells shed whole.** The KPI strip drops `oldest`, then `quiet`, then `working` from
  the right rather than being cut mid-word; blocked never goes. The header sheds in its
  own order (§6 The header).
- **`clampLine` is the backstop, not the layout.** It truncates any line to the width,
  escape-aware, resetting colour so nothing bleeds. The arithmetic is what makes lines
  fit; the clamp makes the consequence of getting it wrong impossible instead of
  invisible. Both are pinned: one test asserts the arithmetic, another walks widths
  20→300 across every UI state and fleet fixture and asserts no line exceeds the width.
- **Resize is already live**: `SIGWINCH` redraws, and `draw()` re-reads `termSize()`
  every call, so nothing caches a width.

### The header (`view/header.go`, 2026-07-28)

    BOARD  32 sessions in 9 workspaces                        20:46:15 · every 10s
    BOARD  32 sessions in 9 workspaces     cmux focus refused   paused · esc to resume

It answers **two** questions and no others: *what am I looking at* (identity, then the
fleet's span) and *is this current* (the clock and its cadence, or the mode that
replaced them). Fleet **state** is the KPI strip's job one line below — a header that
also reported state would give the frame two headlines competing for the same
half-second, which is why the reading "lead with `3 NEED YOU`" was rejected.

- **The workspace span is the one fact no band below carries**, so it earned the slot
  the bare session count used to hold alone. It counts real tabs: background agents
  have no workspace and would otherwise inflate it by one shared bucket. It is a
  derived quantity, so it lives on `Fleet`, not in the renderer.
- **Two blocks, pinned to opposite edges.** Clumped left, the line read as a dim
  footnote that happened to be at the top of the screen; split, it reads as a bar, and
  mode changes land where the eye already goes to check freshness. The right block
  stops `headMargin` short of the last column, mirroring the left indent.
- **The clock keeps seconds.** At a 10s cadence a minute-precision clock cannot be
  told apart from a frozen one, which is the only thing that element is there to prove.
  Paused **replaces** the clock rather than sitting beside it: the data has stopped
  refreshing, so a time would be stating something no longer true.
- **A wrapped header is a frame one line taller than measured**, which scrolls the
  whole thing (§9.10). So it sheds in a fixed order of expendability, re-measuring each
  time — refresh interval (static config), then the workspace clause (context), then
  the notice's tail, then the span's, then the clock's. A test walks widths from 20 to
  300 and asserts one line, never wider than the terminal.
- **The notice is amber, not red.** It is bare text, and bare `#d03b3b` measures 2.91
  (§9.4); critical red is reserved for the badge fill. `NEEDS YOU` is still bare red —
  it is a band label backed by the badge beside it, not the only copy of an error.
- The title stays bright ink and **never a fill**: exactly one element shouts, and it
  is BLOCKED.

### Behaviour

- Redraw is cursor-home + per-line `\033[K`, written as one buffer. A full-screen
  clear each tick flashes — **never blank the previous frame before the new one is
  ready.**
- **Height-aware by measurement, not estimate.** The frame is composed, measured, and
  trimmed until it fits: quiet tail first, then a clip of trailing lines. The count of
  what was cut stays visible so the backlog cannot hide. A frame taller than the tab
  scrolls the header off the top — see §9.10.
- **`+N quiet` is a count, not a control.** Nothing expands a band; the chevron that
  used to sit there implied otherwise (§9.14). A hidden row is read by selecting it or
  by growing the tab.
- The label column is sized to the longest label present, so bars sit beside the text
  rather than across a gap of padding.
- Piped or redirected, `watch` prints a **single frame** and exits. This is the main
  manual verification path, and the reason it is safe to run in a script.

---

## 7. Interaction

    board jump <substring>     from any tab, matches label or workspace
    ↑/↓ or k/j then Enter      from inside board watch
    Esc clears · q or ctrl-c exits

**Why this is not redundant with `claude agents`.** Agent View covers a lot of this
ground and covers it well — a native status, plus select → peek → reply → attach,
with better data than board's. But **attach is not focus.** It attaches to a
transcript inside Agent View; it does not bring the cmux tab forward, with its
splits, its scrollback, the layout set up for that work. Only board knows the surface
id, so only board can do the second thing. That is the gap worth filling; rebuilding
peek-and-reply would be the same mistake as rebuilding Feed.

**The refresh-versus-cursor problem, solved by prior art.** Rows re-sort as idle
grows and sessions change band, so a refresh landing mid-navigation slides the list
under the cursor and Enter jumps to the wrong session.

- **Identity, not position.** Selection is keyed on `Row.Key`, the session id. This is
  exactly why htop has a dedicated `F` "Follow" key; index-based selection drifts when
  the sort moves a row. htop makes it opt-in and drops it on the first arrow key —
  there is no reason to ship the broken variant, so here it is always on.
- **Every row is a stop, including the ones with no tab.** Selection does two jobs: it
  picks the jump target, and it lifts a row out of the collapsed quiet tail. Keying it
  on the surface id did both at once and so hid the rows that had no surface (§9.14).
  Enter on one reports `no tab to jump to`.
- **An explicit, visible pause.** While a selection is live the data stops refreshing
  and the header's freshness block reads `paused · esc to resume` in amber, in the
  slot the clock occupies when the frame is live. `less +F`
  establishes the pattern: streaming and interacting are two modes and the boundary
  should be stated, not hidden. A 10s no-keypress timeout returns to ambient, so the
  tab can never get stuck.
- **Stepping clamps, it does not wrap.** Wrapping past the bottom of QUIET lands on a
  blocked session — the one row you must never act on by accident.

**Jumping does not end the session.** Enter focuses the target, clears the selection
and keeps looping in the now-hidden tab (§9.7). Because the board tab is no longer
visible once a jump lands, focus errors render **in the frame header**, not on stderr.

---

## 8. Safety invariants

**board never closes, kills, hibernates, or otherwise ends a session
automatically.** Not on a timer, not past a threshold, not as an opt-in cleanup
mode, not as a batch action. There is no code path that ends a session without a
per-row action taken by the user at that moment.

Further, the manual `close` action is **not shown by default**
(`{"config": {"enable_close_action": false}}`). Default behaviour on a rotting row is
**jump only**. Someone who never sets that flag can never lose a session through this
tool.

**Dismiss was rejected outright.** Dismiss is a lie: it hides the row while the
session stays alive — a live agent you have promised yourself you will never look at,
still holding memory, still capable of sitting there for weeks. The honest actions
are jump (it's rotting because it's waiting for you) or close (you're never going
back — kill the tab and the row disappears because the pid is gone).

Stated plainly: with no close and no dismiss, **QUIET only grows.** That is accepted.
The band collapses and the count carries the signal. Growing honestly beats shrinking
by hiding.

Four more, each with a test or a documented refusal:

- **Fronting a tab goes through `surface.reorder`, and must stay a no-op on the tab
  strip.** cmux has no verb that just selects a tab, so board reorders the surface onto
  the slot it already occupies (§9.15). The anchor is the neighbouring *surface id*,
  never an index, so a tab closing between board's read and its write cannot shift the
  target: the call fails instead of moving somebody's tab. `findSlot` refuses to place a
  surface it cannot locate in a pane, and the selection is confirmed against the tree
  afterwards.

- **`notify` must never fail the agent.** It runs inside the agent's own hook chain,
  so every error path is a silent success — including a broken `notify_cmd`.
- **`install-hooks` refuses to rewrite an unparseable `~/.claude/settings.json`** and
  writes a timestamped `.board-bak-*` before its first change. It is idempotent via
  the `<binary> notify` command marker.
- **cmux env vars are always stripped from child processes.** cmux treats
  `CMUX_SURFACE_ID`/`CMUX_WORKSPACE_ID` as the implicit target of every command, so a
  stale inherited value makes even a global query fail (§9.8).

---

## 10. Deferred, with the trigger that would revive each

Nothing here is "next". Each entry names the evidence that would move it.

### 10.1 Push notifications — parked, mechanism inert

Built, wired to Slack, verified end to end, then pulled. Current state: `Stop` and
`Notification` hooks call `board notify`, which returns immediately while
`notify_cmd` is empty — no network, no credential read, no sink process. The Slack
sink is recoverable at commit `1f24c00`.

Two things to remember if revisited:

- **Gate on absence, not on events.** With 30 live sessions an ungated sink posts on
  every turn end of every agent. macOS `HIDIdleTime` gives real keyboard idle cheaply,
  and "only tell me when I'm actually away" is the difference between a useful channel
  and a muted one.
- **The failure mode is muting, not missing** (§9.2).

*Trigger:* wanting to know about blocked sessions while away from the machine, plus
a willingness to gate on keyboard idle. Absent that, the ledger (§5) is the better
answer.

### 10.2 Intents and derived todos — blocked on evidence

The model: derived todos are one per live session (zero capture cost, cannot go
stale, dies with the session), and an **intent is a session that hasn't started
yet** — text and age only, no status, no priority, no due date, one transition
(intent → session). Age is displayed and never hidden, so an intent sitting for two
weeks reads as a reproach rather than a backlog item.

*Trigger:* §9.9 falsified the assumption this rests on. It needs evidence that
labelling actually happens, or a redesign around auto-names.

### 10.3 Snooze — the right primitive if silencing is ever needed

If use shows you want to keep a session but silence it, snooze (hides for N hours,
then returns) is the honest version, not dismiss or close. It adds per-row state,
which §1 resists. *Trigger:* a specific session you keep re-reading and re-ignoring.

### 10.4 HTML dashboard — superseded

A self-contained HTML file opened in a cmux browser pane was the original
recommendation, chosen for real typography and proportional bars. The ambient TUI
won instead: no listener, no daemon, no file-watching — the tab *is* the process,
which keeps §2's hard constraints intact with nothing to flag.

### 10.5 Workspace-grouped layout — rejected

Dense and pretty, but it answers "how is my fleet arranged", a question you never
have. One workspace held 7 unrelated sessions; grouping would scatter the two things
that need you across five boxes. Workspace stays a column.

### 10.6 `close` action — built as a flag, off by default

See §8. It exists behind `enable_close_action` and is not shown by default.

### 10.7 The `ASKED` ledger — deleted, not parked

Removed with `internal/workstream` (§5, §9.13). Recoverable from git; the parsing was
sound and `Event.Settles` cost three attempts to get right, so do not rewrite it from
scratch.

*Trigger:* an ask dying unanswered often enough to matter — the measurement said 4
times in two months, none inside the window the band showed. If that changes, the
cheap form is **one KPI cell that appears only when non-zero**, not a band; and it
still has to answer "what do I do about it", which for a dead session is nothing.
Reviving the rows needs a new answer to §5's table, not a new filter.

---

## 11. How this is verified

TDD is mandatory for behaviour changes, and §9.1 is the reason: **derived state must be
pinned by a test over a fixture, not by reading the function.**

The pure seams exist for this and must stay pure:

| seam | package | fixture |
|---|---|---|
| `Build(Snapshot, now)` | `board` | a `Snapshot` literal — roster, rank, label, staleness, sort |
| `Frame(fleet, screen, ui)` | `view` | a `Fleet` literal; golden files pin the whole screen |
| `Table(fleet, threshold)` | `view` | same |
| `idleScale` / `bar` / `humanize` / `short` / `pad` | `view` | values |
| `parseTop` / `parseHookClock` / `StripSpinner` | `cmux` | JSON literals |
| `findSlot(top, surface)` | `cmux` | a `top`-shaped literal — placement is what would reorder somebody's tabs (§9.15) |
| `parseAgents` | `claude` | JSON literal |
| `hasCommand` | `hooks` | settings-shaped literals |

`Collect`, `Agents` and `TitlesByPid` touch `$HOME` and subprocesses —
keep logic out of them and test the parsers they feed. **Never write a test that
depends on the live fleet;** capture lines into a fixture instead.

Manual verification, for layout and colour:

    COLUMNS=118 LINES=44 board watch | cat    # one frame, no terminal needed
    LINES=24 COLUMNS=100 board watch          # exercise collapse and narrow widths
