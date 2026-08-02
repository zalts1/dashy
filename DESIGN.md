# board — design record

**Status: what the tool does today, and why.** This is a record of decisions and the
reasoning behind them — not a rulebook, and not a list of things you may not do. Read
the section covering what you are about to change so you know what you would be
trading; then decide. Most of what is here was argued, not measured, and an argument
loses to a working change. Only `CLAUDE.md`'s **Hard** list is off-limits.

When you do change something, append what you learned to §9 (`EVIDENCE.md`) and update
the section here so it keeps describing the tool that exists.

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
| 10 | deferred ideas, each with the trigger that would revive it | before building one of them, and §10.8 before touching the keymap |
| 11 | the pure seams and their fixture shapes | writing any test |
| 12 | todos: a row with no process, the cap of 10, where capture and removal each live | `config/`, `board/build.go`, the `TODO` band |
| 13 | `version`: the tag is the release, upstreams reported verbatim, absence never raised | `version/`, `host/probe.go`, releasing |
| 14 | trouble: an unreadable world is not an empty one, and `doctor` reads without writing | `claude/`, `board/build.go`, `view/header.go`, `doctor/` |
| 15 | `uninstall-hooks`: defined as install's inverse, and what it must not touch on the way | `hooks/`, anything editing `~/.claude/settings.json` |
| 16 | the demo: a fixture fleet through the real join, recorded by hand | `internal/demo/`, `demo/`, the top of the README |

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

Four things it deliberately is not. Each is a judgement about where the bottleneck is,
so each is only as good as that judgement:

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
| three actions, total | modes and menus to learn | **restated** — jump, label, todo |
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
keymap is a handful of single keys that each do one thing, ctrl-c exits, and the
one-shot table stays first-class so board is useful without a dedicated tab.

**"Three actions" is now three: jump, label, and todo.** Dismiss was rejected (§8).
Todos add capture and removal (§12) — the third action, entered here as this rule
requires. A fourth needs the same.

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

- **Chrome is derived, never guessed.** `rowChrome(barW)` is built from the pieces it is
  made of — lead, gutter, bar, warn mark, IDLE, gaps — so it cannot drift out of step with
  the layout the way the fixed reserve of `46` did. It takes the bar's width rather than
  assuming it: the bar is elastic, and a constant that quietly disagreed with it would be
  §9.12 again.
- **Two elastic columns, sized to content**: the label, and the tail (workspace on a
  row, `HH:MM where` in ASKED). The tail was previously unbounded, which is what
  overflowed: workspace names are user data. Squeeze order is meaning first — the label
  shrinks to its floor, then the tail truncates. A resize moves those two and leaves
  every other column where it was.
- **Three elastic columns, and the order is meaning first in both directions.** Squeezing:
  the label gives way to its floor, then the tail truncates. Spending: the label is filled
  out whole first, and the bar takes whatever is left, so the row ends on the frame's right
  edge instead of stopping short of it. The bar is the right place for surplus because the
  gap it closes is the one between a label and its bar — and it is the last to be paid,
  because a wider bar is a bonus and a label is the row's meaning (§9.29).
- **The label column is the whole label while the row can hold it, and the p90 once it
  cannot.** Bars encode one absolute scale, so they have to start on a shared column —
  which meant a single long title pushed every other row's bar across a corridor of
  padding to reach it. p90 is the fallback under pressure, not a policy: truncating on a
  window with columns to spare is choosing to lose text it could have shown (§9.29).
- **The frame has one right edge, and it is the frame's, not the terminal's.** The header
  is written last and sized to the widest line below it, bounded by the terminal and
  floored by what it has to say unshed. Now that the bars spend the surplus the two
  usually meet at the terminal's edge — but they meet because the header follows the
  frame, not because both are measured against the same terminal, which is what was wrong
  before: the header spanned the terminal unconditionally, agreed with the table only at
  the width the goldens were captured at, and put the clock 100 columns from anything it
  described on a real monitor (§9.29).
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
  mode changes land where the eye already goes to check freshness. The right block lands
  on the frame's right edge — `headMargin` short of the last column while the bands below
  reach that far, and short of *that* when they do not (§6 Width, §9.29).
- **`headGap` is what keeps them two blocks.** Five columns, the KPI strip's inter-cell
  gap, and the header sheds rather than closing below it. One space was the floor while
  the header always spanned the terminal, because only a terminal narrow enough to be
  shedding could reach it; sized to the frame it is reachable at any width, and
  `35 sessions in 5 workspaces cmux focus refused` is one sentence, not two blocks.
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
    z folds QUIET to its count · Esc clears · q or ctrl-c exits

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
- **The letter keys are mapped on the Hebrew layout too**, to the same physical keys
  (`א ש ג ח ל ז`). A keymap of single letters is otherwise dead in any non-Latin input
  source — for a bilingual user half the day — and telling someone to switch layouts to
  press one key is not a keymap. Those runes cannot come from a Latin layout, so nothing
  is ambiguous; `q` is the exception, because its Hebrew position emits `/`, which a
  Latin user does have a key for. Quitting from a non-Latin layout is ctrl-c.
- **The fold is the one key that changes what is drawn, not where the cursor is.** QUIET
  opens by default and `z` folds it to its header and count; the rows it frees go to the
  bands where something is happening. It is the reader's, so it holds across refreshes and
  is not persisted — a fold is about this sitting at the tab, not a preference. Folding
  clears a selection inside the band, and folded rows leave `DisplayOrder`: the order
  follows the screen, so a row the frame is not drawing cannot be a stop (§9.21).
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
- **`install-hooks` and `uninstall-hooks` refuse to rewrite an unparseable
  `~/.claude/settings.json`** and write a timestamped `.board-bak-*` before their first
  change, never overwriting an earlier backup. Both are idempotent via the
  `<binary> notify` command marker. **Neither may change anything it did not write** —
  not another tool's hooks, not an unrelated key, and not the file's permissions (§15).
- **cmux env vars are always stripped from child processes.** cmux treats
  `CMUX_SURFACE_ID`/`CMUX_WORKSPACE_ID` as the implicit target of every command, so a
  stale inherited value makes even a global query fail (§9.8).

---

## 10. Deferred, with the trigger that would revive each

Ideas considered and not built yet. Each names the evidence that would most clearly
justify it — a prompt for thinking, not a gate. Wanting one is reason enough to build
it; the trigger just tells you what the earlier reasoning was waiting for.

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

### 10.2 Intents and derived todos — the intent half shipped as §12

**Intents shipped**, on the terms recorded here: text and age only, no status, no
priority, no due date. §9.9 turned out to be evidence about *labels*, not about this —
a label is decoration on a row that renders either way, so not typing one costs
nothing, while a forgotten customer request costs a customer (§9.17).

**Derived todos are not built, and need nothing built.** A live session already *is*
the row — labelled, ranked, aged. Reading the fleet as a list of things to do is what
§1 already produces. The other reading of "derived" — parsing the agents' own
`TodoWrite` lists — is dead on measurement: 3 of 100 transcripts touched in a week
contained one (§9.17).

*Still deferred:* linking a todo to the session that serves it, and undo. §12 names
the trigger for each.

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

### 10.8 Layout-independent keys — considered, declined (2026-07-29)

A keymap of single letters is dead in any non-Latin input source, and the Hebrew layout is
handled by mapping the same physical keys (§7). Two ways to generalise, both declined:

- **The kitty keyboard protocol** reports the base-layout key alongside the typed rune,
  which is the only mechanism that actually recovers "which key was pressed" — a terminal
  otherwise receives characters, not keycodes. It needs `CSI > 1 u` negotiation, a CSI-u
  parser, and a fallback for the terminals that decline. Too much machinery for six keys.
- **Tab for the list and Backspace/Delete for finishing** are layout-invariant, and
  arguably better bindings than `t` and `d` on their own merits. Declined only because the
  letters plus §7's Hebrew mapping already cover the layouts in use here.

*Trigger:* a third input source, or a second person using this. The second bullet is the
cheap half and does not need the first.

*Not to do:* grow the layout table to Russian, Arabic, Greek and so on. It is unbounded,
and every entry is a guess about a layout nobody here can verify.

### 10.9 Locking the config against a lost update — deferred (2026-07-31)

`Save` became atomic in §9.27: a reader sees the old file or the new one, never half of
one. **Atomic is not exclusive.** Every writer does Load → mutate → Save
(`watch/todo.go:20`, `cmd/board/todo.go:21`, `commands.go:46`), so two that overlap can
each read ten todos and each write eleven — the second rename wins, one addition is gone,
and nothing reports it.

Not built, because the window is the microseconds between a Load and its Save on a file a
person edits a few times a day, and each mechanism that closes it costs more than the race:

- **A lock file** needs a stale-lock policy, and a stale lock is worse than a lost todo on
  a reporting surface: it fails the write that would otherwise have succeeded.
- **`flock` held across the read-modify-write** is correct and cheap to write, but `watch`
  loads the config on every tick (`watch.go:92`), and a reader waiting on a writer is a
  frame that stalls for reasons the reader cannot see.
- **The durable fix is not locking.** Appending a todo is the only contended write, and an
  append-only list would make two additions associative — no lock, no lost update. That is
  a file format change and belongs with §12, not here.

*Trigger:* a lost todo somebody actually notices, or a second writer that is not a human
keystroke — a hook, a daemon, or `$HOME` on a file-sync service. The last is the one to
watch: sync makes the window as wide as its own interval, and it can resurrect a deleted
todo as easily as drop a new one.

---

## 11. How this is verified

TDD is mandatory for behaviour changes, and §9.1 is the reason: **derived state must be
pinned by a test over a fixture, not by reading the function.**

The pure seams exist for this and must stay pure:

| seam | package | fixture |
|---|---|---|
| `Build(Snapshot, now)` | `board` | a `Snapshot` literal — roster, rank, label, staleness, sort, todos |
| `Frame(fleet, screen, ui)` | `view` | a `Fleet` literal; golden files pin the whole screen |
| `Table(fleet, threshold)` | `view` | same |
| `idleScale` / `bar` / `humanize` / `short` / `pad` | `view` | values |
| `parseTop` / `parseHookClock` / `StripSpinner` | `cmux` | JSON literals |
| `findSlot(top, surface)` | `cmux` | a `top`-shaped literal — placement is what would reorder somebody's tabs (§9.15) |
| `parseAgents` | `claude` | JSON literal |
| `hasCommand` | `hooks` | settings-shaped literals |
| `AddTodo` / `DeleteTodo` / `FindTodo` | `config` | a `State` literal — the cap, the prefix match, the age |
| `Format(Info)` | `version` | an `Info` literal — the interesting cases are the missing tools, which no fixture-free test could reach (§13) |
| `firstLine` | `host` | stdout-shaped byte literals |
| `trouble(Snapshot)` | `board` | a `Snapshot` with `RosterErr`/`NoCmux` set — an unreadable world is only reachable as a fixture (§14) |
| `Format(Report)` | `doctor` | a `Report` literal; every row's broken case, including the ones that must not print (§14) |
| `installedEvents` | `hooks` | settings-shaped literals — must agree with what `Install` writes |

`Collect`, `Agents` and `TitlesByPid` touch `$HOME` and subprocesses —
keep logic out of them and test the parsers they feed. **Never write a test that
depends on the live fleet;** capture lines into a fixture instead.

`Save` is the one impure function with tests of its own (`config/save_test.go`), because
atomicity and file mode are properties of the write and cannot be reached from a parser.
They isolate `$HOME` with `t.Setenv` — a test that wrote the real `~/.board.json` would
destroy the user's todos, which is precisely what §9.27 is about.

Manual verification, for layout and colour:

    COLUMNS=118 LINES=44 board watch | cat    # one frame, no terminal needed
    LINES=24 COLUMNS=100 board watch          # exercise collapse and narrow widths

CI (`.github/workflows/ci.yml`) runs `gofmt -l`, `go vet`, `go test -count=1`, a build
and a `board version` smoke test on `macos-latest` — macOS because the build is
darwin-only (`watch/term.go` uses `TIOCGETA`). **The runner is the outsider machine: it
has neither `cmux` nor `claude`**, which is what makes every degraded path a fact rather
than an argument. Their absence is asserted, not assumed, because a runner image that
began shipping either tool would silently stop being a test bed. `-count=1` is
deliberate (§9.24), and `git diff --exit-code` after the tests proves the suite mutates
nothing — goldens are re-blessed by hand, with a reason, or not at all.

---

## 12. Todos — a row with no process

A todo is a line of text with an age, occupying a row until you remove it. It exists
for the work board could not otherwise see: **a thing you have not started, so there
is no session, no pid, and no tab.** A customer request you will forget by Thursday is
the case; the two-week-old row that results is the point, not a defect.

This is §10.2's *intent*, shipped on the terms recorded there — text and age, no
status, no priority, no due date. The other half of §10.2 stays unbuilt because it
needs nothing: a live session already is the row you would have derived.

**It overturns one line, deliberately.** `board/build.go` drops sessions with no cmux
surface, because "a row you cannot jump to is a row that cannot be acted on." A todo
has no surface and never will. The rule survives with its action named: **the action
on a todo is to start it**, and until then, to remove it. A row with no action is a
backlog row and belongs in Linear (§1).

**The cap is 10, and it is the feature's immune system.** Uncapped, todos accumulate,
the band collapses the way QUIET does, and board becomes a list you scroll past —
which is exactly the dilution §1 exists to prevent. Adding an 11th fails and says so.
That refusal is the mechanism: it forces the decision the tool exists to force.
Nothing sorts, filters, or hides todos, for the reasons in §8.

**Both capture and removal happen in the frame.** Capture was CLI-only for one release
and that was wrong: two entry points for one thing is the friction, not the keystrokes
(§9.18). The CLI kept every verb anyway, for scripts and for the terminal you are
already in.

- `a` opens the prompt, `Enter` adds, `Esc` cancels. Empty text is a cancel, not an
  error — the mode is one keystroke away, so opening it by accident must cost nothing.
- `t` selects the top of the list. Stepping there means walking past every quiet row —
  fifteen presses on a real fleet — and a list you have to travel to is one you stop
  acting on. Like `d`, it is named in the legend only where it does something (§9.14).
- `d` on a selected todo row removes it. This is the one place removing a row is not
  the lie dismiss was (§8): dismiss hid a session that stayed alive, whereas a todo has
  nothing behind it, so removal is the honest end of it. Done and dropped are the same
  action, and nothing is archived — history is not board's job (§9.13).
- No undo. The header reports the text it removed, so a mis-key costs retyping one
  line. *Trigger:* deleting one by accident and minding.

**The mode is bounded on purpose, because §2 retired "no TUI" only on the condition
that there be nothing to learn.** What keeps it honest: one key in and two keys out; no
cursor movement, so there is nothing to learn beyond typing and backspace; the prompt
takes the legend's line rather than adding one, so entering it cannot change the
frame's height and collapse the tail under the typist; and **no timeout.** §7's 10s
return exists so a stray arrow key cannot leave the tab paused forever, and typing is
not stray — a timer that discarded half-typed text would be the one destructive thing
in the loop. ctrl-c still raises SIGINT, so there is always a way out.

The key decoder is deliberately mode-blind: every keystroke carries both the command it
names and the text it would type, and the loop picks. A decoder that had to be told the
mode would be a second place the mode can be wrong — and the trap it walks into is real,
since the tail of an arrow key is printable and types `[A` if decoded rune-wise (§9.18).

**The list is drawn last, after the whole fleet.** It is not a state a session can be
in, so it does not belong among the bands that rank process states — which is where it
sat for one release, between WORKING and QUIET, reading as a fleet state (§9.19). Last
also matches the rank order: `RankTodo` is 3, and the one-shot table already printed it
there.

**The empty band is drawn, and it is the one exception to §9.13.** `TODO  nothing on
your list` costs two lines and reports no exception, which that rule argues against. It
stays because the alternative was worse: with no todos there was no band, so nothing on
screen said a list existed, and `t` was a key nobody could learn. A feature that renders
nothing until used is a feature nobody discovers. NEEDS YOU already carries the same
empty line for a different reason, and the empty form says a phrase rather than `0 of
10` — a count is something to read, a phrase is something to learn from. On a short tab
those two lines cost the quiet tail a row rather than the legend, which is the right way
round: the legend is where `a` is taught.

**It earns that position with a floor, not by hiding from the collapse.** The reason it
was drawn high was arithmetic — the fit loop trims the quiet tail and then `clip`s from
the bottom, so the lowest band is what a short tab deletes. The fix is a second absorber
rather than a safer address: quiet gives way to its floor, then the list to its own, then
either to nothing at all. **A band collapsed to its count still reports** — `QUIET · 22`
with no rows says the thing that matters — so shedding rows always beats letting `clip`
take a whole band. Below about 20 rows the list does go; that tab is too short for it,
and the fleet's live state is what the frame is for.

**No bar, no workspace, and age stated as a lifetime.** A bar means rot on the idle
scale, and the two quantities are not the same: a session's idle time is a **gap** that
resets the moment the session is touched, while a todo's age is a **lifetime** that only
grows. Drawn as the same glyph they invited a comparison that is not valid, and `0m` read
as "active this minute" on a note written seconds ago (§9.19). So the row is sparse —
mark, text, `12d ago` — which is also what makes it read as a different kind of thing
without spending a second accent colour (§6 keeps exactly one shout, and it is BLOCKED).
WORKING rows already have no bar, for the same class of reason.

**None of it feeds the KPIs.** `Fleet.Oldest`, `Stale`, `Blocked` and `Workspaces` are
statements about *sessions*: a todo in `Oldest` would own the KPI strip and misreport the
fleet, and the staleness `⚠` marks an idle session past a threshold, which is not a thing
a todo can be. The band header carries `· 3 of 10` instead, which is where the cap is
worth stating — you watch the number climb, so the refusal at 11 is not a surprise.

**Not in v1: linking.** A todo and the session that serves it stay separate rows, and
keeping one from duplicating the other is the user's job for now. When it is built it
will be an explicit keystroke, never text matching — a fuzzy link between free text
and a session is the silent-mismatch family of bug §9.15 came from. It anchors on the
surface id, as labels do (§4), so it survives the session ending in that tab. The open
question, recorded before it is answered: when a linked session dies with the work
undone, the todo has to come back, with its original age intact.

*Kill criterion, agreed before the code was written:* two weeks of counting todos
added, todos removed as done, and the median age at removal. A count that grows while
the median sits at two weeks means this is a backlog wearing a dashboard's clothes, and
it goes back to Linear (§9.17).

---

## 13. `version` — what is installed

board is a third of its own bug report. The roster comes from `claude agents --json`
and the tabs from cmux, neither is a documented contract, and both have moved before
(§9.1, §9.3) — so "board is broken" is unanswerable without the two upstream versions
beside board's own. That is the whole command: three lines, meant to be pasted.

**There is no version constant and no `-ldflags`.** `runtime/debug.ReadBuildInfo`
already knows: `go install ...@v0.1.0` stamps the tag, and since Go 1.24 a plain
`go build` inside the repo synthesises a pseudo-version from VCS, with `+dirty` when
the tree is modified. **The tag is the release** — nothing to bump, so nothing to
forget to bump. A build with no VCS directory reports `(devel)`, which is a fact about
how that binary was made and is passed through rather than laundered into `unknown`.

The two upstream strings are printed **verbatim**. Parsing them to a bare number would
put a parser over two undocumented surfaces on the far side of someone else's machine,
which is §9.1's mistake in a new place; cmux's build number and commit are also the
next thing anyone would ask for. The single exception is a leading token that repeats
the label — `cmux --version` names itself and `claude --version` does not — dropped so
the block reads, at a cost of one duplicated word if either format changes.

**Absence is reported, never raised.** This is the command someone runs when nothing
works, so a missing tool is `not found` and an unreadable board is `unknown`: distinct,
because one sends you to an install and the other does not. It cannot fail and cannot
print an empty field (§8, and §11 for the fixtures that pin it).

`claude.Version` and `cmux.Version` live beside the other calls to their own binaries,
so everything that shells out to a tool is greppable in one package; `host.Probe` holds
the one shared judgement, which line of stdout to believe.

---

## 14. Trouble, and `doctor` — what board could not read

board reads two undocumented surfaces and used to throw away every failure of both.
`claude.Agents` returned a bare `nil` on any error and on any shape it did not
recognise; `show` tested for cmux and `watch` did not. So a missing `claude`, a changed
roster schema, and a genuinely quiet fleet produced **the same screen**, and a
first `board watch` on a machine without cmux painted an empty dashboard explaining
nothing. On the author's machine none of this is visible. On anyone else's it is the
whole support burden (`EVIDENCE.md` §9.26).

**The fix is one concept, not four.** `Fleet.Trouble` is the phrase for what could not
be read, empty when the world was legible, derived in pure `Build` from a `Snapshot`
that now carries `RosterErr` and `NoCmux`. Both renderers print that one field, for the
same reason every count is derived there: two renderers must not invent their own words
for the same fact (§3).

Consequences, each falling out rather than designed:

- **It takes the header's span slot.** "no sessions" is a claim about the fleet; an
  unreadable roster is a claim about board, and the span is already the answer to *what
  am I looking at*. Costs no line, so the fit invariant is untouched (§6). Painted
  `statusWarning` — the value the ⚠ and the paused clock already use, never bare
  `statusCritical`, which fails contrast as text (§9.4).
- **The trouble precedes the count rather than replacing it** when rows did arrive. With
  no cmux the background agents still come through, and a header reporting only the
  trouble would trade one silence for another.
- **`watch` needed no guard after all.** The right fix was never a second refusal in a
  second entry point; it was the frame reporting what it is showing. `show` lost its
  cmux test in the same change, so both entry points now read one phrase off one field
  and cannot disagree. The one-shot table leads with it, because that output is piped.
- **The roster outranks the tabs.** With no roster there are no rows at all; with no
  cmux the background agents survive. One line carries the more fundamental fact and
  names `doctor`, which carries both.

**`doctor` is the escalation, and `version` stays.** They answer different questions —
`version` is three lines to paste, `doctor` is the diagnosis — so this is not §9.18's
two entry points for one thing. It embeds `version.Info` and calls `version.Format`, so
there is exactly one place that knows how to print a version string, and it aligns its
own four rows to `version.LabelWidth`.

Its rows are the wiring: the three versions, then the roster (a count, or the error's
own words — the frame's "· board doctor" would be circular here), the tabs (`0 tabs`
with a healthy roster is the shape of §9.3), the hooks (named individually, because a
half-install is a hook that never fires), the config path (and whether it exists yet),
and whether notifications are on.

**It never prints `notify_cmd`.** The output is meant to be pasted into a bug report and
that field is a shell command routinely carrying a webhook URL or a token. Whether it is
set is the diagnostic; the command is the user's secret.

**It reads and never writes** — `hooks.Installed` is deliberately separate from
`Install`, sharing the marker and the event list so a diagnosis cannot disagree with the
installer. It is also the command that runs on machines where `install-hooks` refuses, so
an unparseable `settings.json` is a row of its own rather than "not installed" (§8).

---

## 15. `uninstall-hooks` — leaving no trace in a file board does not own

The README used to end by asking the reader to open `~/.claude/settings.json` and delete
two entries by eye. That is a fine instruction for the person who wrote the format and an
unreasonable one for anybody else: **install being safe while uninstall is manual is not a
tool you can ask a colleague to try.** So uninstall holds every line install holds (§8) —
the refusal on an unparseable file, the backup before the first change, idempotence
through the same `<binary> notify` marker.

**It is defined as install's inverse, and that is the test.** Install into a settings file,
uninstall, and what comes back must be the file that went in. Anything weaker permits a
command that removes the hooks and leaves the file subtly not what it was.

Three things follow from that definition, and each is a way the obvious implementation
gets it wrong:

- **The entry is the unit of removal, not the group.** board writes one command per group,
  so dropping whole groups looks equivalent — until a group holds board's command beside
  another tool's, which this file being the user's makes possible. A group is dropped only
  once board's own removal has emptied it; a group that arrived empty is left, because
  board did not empty it.
- **An emptied container goes.** `"Stop": []` and no `Stop` key are the same instruction to
  Claude Code, so removing it cannot change what runs — and it is the only choice under
  which an install/uninstall cycle does not accrete scaffolding that outlives the tool.
- **The file's mode is preserved explicitly**, which the atomic write made necessary rather
  than optional (§9.28).

`hooks/settings.go` is the whole of board's access to this file — read, back up, write —
and both commands go through it, so care taken on one path cannot go missing from the
other. The write is the same temp-and-rename as `config.Save` (§9.27), for a stronger
reason: a torn `~/.board.json` costs a todo list, a torn `settings.json` breaks Claude
Code.

`doctor` still only reads (§14). Diagnosing this file and changing it stay separate
commands.

## 16. The demo — a fleet nobody has to protect

A terminal tool with no picture at the top gets scrolled past, and board's picture is
the whole argument for it: the bands, the ⚠ waterline, a session lifting out of the
quiet tail. So the README leads with a recording of `board watch`.

**What cannot be recorded is board being used.** Every row on a real board is work — a
label, a workspace name, a customer in a todo — and the README is the one file in the
repo that is read by people the fleet is not theirs to see. The GIF therefore comes from
a synthetic *world*, not a synthetic *screen*: `internal/demo` builds a `board.Snapshot`
of agents, tabs and todos and hands it to the same `board.Build` production calls.

That choice is what makes the picture trustworthy. A hand-drawn frame — the shape the
README's static block used to be — is a claim about the renderer that nothing checks; it
drifts the moment a column moves, and it can show a fleet the join could never produce.
Going through the pure seam (§3) means the recording is board rendering, and the only
fiction is the fleet. It is the same property a golden file uses (§11), pointed at a
GIF instead of a test.

Three constraints hold it in place:

- **It is a `main` under `internal/`, and that is deliberate.** `internal/` is what stops
  `go install .../demo` from existing. board gains commands by §1, not by whatever
  happened to need a binary.
- **The recording is a human step, not a CI step.** `vhs` pulls in `ttyd` and `ffmpeg`,
  and a project with no dependencies (§2) should not acquire three of them in CI for an
  image that changes a few times a year. The cost is that the GIF can go stale while the
  suite stays green; `CONTRIBUTING.md` carries the reminder, which is the honest place
  for a check no machine performs.
- **It records on `#282c34`, the lighter of the two backgrounds every colour was
  validated against (§6), and on at least 32 rows.** A theme nobody validated would put
  an unchecked contrast claim at the top of the README. Below 32 rows the fit loop starts
  trimming the quiet band to `+N quiet` (§9.21) — correct behaviour, and the wrong thing
  to lead with.
