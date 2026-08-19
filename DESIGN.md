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
| 3 | the inputs, one file written, pid as join key, the package tree | `board/`, `host/`, adding a source |
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
| 15 | `uninstall-hooks`: defined as install's inverse, and what it must not touch on the way | `hooks/`, anything editing `~/.claude/settings.json` or a maki `init.lua` |
| 16 | the demo: a fixture fleet through the real join, recorded by hand | `internal/demo/`, `demo/`, the top of the README |
| 17 | maki as the second agent: no roster command, so board installs one; two reads settle liveness | `maki/`, `hooks/maki.go`, `board/build.go` |
| 18 | links: the three things a row points at besides its tab, joined on the worktree, opened by the terminal, and which editor answers | `preview/`, `editor/`, `host/worktree.go`, `view/link.go`, `board/build.go` |
| 19 | the workspace as a **group** rather than a column: two mental models, a header by exception, a rail that costs no columns, and the user's colour lifted until it is legible | `board/board.go`, `view/group.go`, `cmux/sidebar.go`, `host/branch.go` |

---

## 1. What board is

One screen showing every live coding-agent session running under cmux — Claude Code and
maki — ranked by whether it needs you.

The bottleneck it addresses is **attention, not capacity**. With ~30 concurrent
sessions the failure is not that too few agents are running; it is that two of them
have been blocked on a question for a day and nothing surfaces that. So board
optimises for one thing: *where do I look first*.

**It is a reporting surface.** It reads six sources, writes one file, and takes
exactly two actions on the world (focus a tab, set a label). That is what makes it
safe to leave installed, and it is the property to protect when adding features.

**Which agent a row belongs to is not on the screen.** The three states are the fleet's
vocabulary and both agents map onto them exactly, so a column repeating the agent on every
row would be the one thing §9.13 says a band has to earn — and it would answer a question
the reader does not have, since the action is the same either way. `doctor` counts the two
rosters separately, which is where the distinction is worth something (§14, §17).

Four things it deliberately is not. Each is a judgement about where the bottleneck is,
so each is only as good as that judgement:

- **Not a task manager and not a kanban.** Nothing moves through stages — a session
  is blocked, working, or rotting. Structure belongs in Linear, which already exists.
- **No spawning or orchestration.** Making it easier to run *more* agents attacks
  the wrong side of the bottleneck.
- **No rebuilding what cmux or either agent already ships.** Not cmux's Feed, not
  `claude agents`' peek/reply/attach, not maki's own `/sessions` picker — which sees one
  maki process, where board's gap is every tab at once. Jump-to-tab is the real gap (§7).
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

Six inputs, all read-only. One file written.

| what | source | cost |
|---|---|---|
| claude roster + state | `claude agents --json` — interactive `status` idle/busy/waiting, background `state` blocked | ~250ms |
| tab title, workspace, surface UUID | `cmux --id-format both top --all --json`, joined on **pid** | ~50ms |
| idle clock | cmux `~/.cmuxterm/claude-hook-sessions.json` → `updatedAt`, else transcript mtime | free |
| a background agent's open question | `~/.claude/jobs/<shortId>/state.json` → `needs` | free |
| maki roster + state + clock | `~/.board/maki/<surface>.json`, written by the Lua block board installs (§17) | free |
| maki liveness | `pgrep -x maki` — a report outlives the process that wrote it (§17) | ~5ms |
| labels + config | `~/.board.json` — **the only file board writes** | free |

`~/.cmuxterm/workstream.jsonl` was a fifth source, read 8MB deep every tick to feed
the `ASKED` ledger. Both are gone — §9.13.

The subprocess calls are independent and run concurrently, so a tick costs ~250ms rather
than the sum.

**pid is the join key, for both agents.** cmux surface nodes carry no session UUID, and
`cmux_process_pids` names the processes on a surface. The tree is walked rather than
indexed because its nesting has changed before. maki reports are keyed by surface id and
reached through the same pid map, so there is one join and not two (§17).

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

**Ordering, amended 2026-08-19 (§9.46).** Within a band, sessions sort **most recently touched
first**: the top of QUIET is what you were last doing. It was the reverse until a reader read a
column of times counting down from 2d22h as a sorting bug — which it is, to anyone whose other lists
all put the newest first. The old argument was not wrong so much as redundant: neglect is already
stated in the header's `oldest`, in the strip's `N quiet >45m`, and per row by `⧗`, so the band's best
position was spending itself on a third telling.

**Todos are the exception and keep the old order.** §9.19 established that idle time and age are two
quantities in one field: a session's is a *gap* that resets when touched, a todo's is a *lifetime*
that only grows. So the newest session is where the work is, and the oldest todo is the reproach.

The collapse pays for this: `pick` sheds from the end of a band, so a short tab now hides the most
neglected sessions rather than the least. `minQuietRows`, the `+N quiet` count and the two header
statements are what keep that visible rather than silent (`view/layout.go`).

| state | meaning | claude | maki |
|---|---|---|---|
| `blocked →` | genuinely needs an answer | interactive `status: waiting`, or background `state: blocked` | `needs_input` |
| `running` | working | `status: busy` | `working` |
| `done` | finished its turn, unnoticed | everything else | `idle` |

Both agents land on the same three words, which is what makes the second one an addition
rather than a dialect. maki's `needs_input` is the cleaner of the two blocked signals: it
is `permission_prompt.is_open() || pending_input`, so it already means what board wants,
where claude needed the ledger below to get there.

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

Label precedence: user label → the agent's own name for it → cmux tab title → cwd
basename. The middle rung is Claude Code's `needs`/name for a background job and maki's
session title; one rule, one function, so the two agents cannot come to disagree about
what a row is called. Labels are keyed on **surface id**, so they survive a session ending
and being resumed in the same tab.

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
| link set, 4 glyphs | `#ff99bb` `#51cf66` `#3bc9db` `#b0b0ff` | 7.02 / 6.98 / 7.04 / 7.00 — matched, so none dominates (§18) |
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
- **Rows are spaced out when the frame can afford it** (2026-08-19, §9.44). One blank line between
  rows, composed first and whole; if it does not fit the tab, the compact form runs with the
  shedding ladder instead. Air is a luxury and rows are information, so a tab one line too short
  for the airy frame shows every row compact rather than most rows spaced. Terminal line height is
  the other half of this and is not board's — that is Ghostty's `adjust-cell-height`.
- **The state gutter is elastic and its mark is left-aligned** (amended 2026-08-19, §9.41). It
  was a fixed 10 columns, sized for the BLOCKED badge, with the mark right-aligned to hug the
  label — so a quiet row wore eleven blank columns in front of its `○` and every fleet paid for
  a badge most of them did not have. It is now the badge's width when something is blocked and
  four otherwise, with the mark starting one column after the lead. A blocked row arriving does
  shift the table right, which is accepted: a row entering NEEDS YOU already moves every band on
  the screen, and the six columns belong to the label the rest of the time.
- **The stale mark is `⧗`, and it rides beside the state mark** rather than out by the duration.
  A warning triangle claimed something was wrong; a session nobody has looked at for three hours
  is waiting, not broken. Beside the state it qualifies it — `○ ⧗` is one glance — where three
  columns from the IDLE value it read as a property of the number. It comes from the link
  glyphs' Unicode block so it is drawn at their size (§18).
- **No bars in `ASKED`.** Waits are minutes against a scale topping out at a week,
  so every row rendered a single cell and meant nothing. `never` in amber is the only
  thing in that band that earns colour.
- A value scale without a key is decoration, so the ramp legend is drawn wherever the
  bars are — and **only** there. A key for a scale that is not on screen is noise, so
  the narrow layout drops both together.
- **Amended 2026-08-19 (§9.38): the key names the dimension, `?` gives the resolution.**
  The ambient line carries `▇ elapsed`; the five rungs moved behind `?`. The rule survives
  because the bar is still labelled — a reader knows it measures elapsed time without
  pressing anything — and only the rung *values* are a keystroke away. What that bought is
  the width for the link hints and for a legend that can afford to name what each glyph
  opens, neither of which fitted beside the full ramp.

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

- **`notify` must never fail the agent, and must never make it wait.** It runs inside
  the agent's own hook chain, so every error path is a silent success — including a
  broken `notify_cmd` — and the sink gets a 3s budget rather than the agent's patience.
  Discarding the error covered a sink that answers wrongly and did nothing for one that
  does not answer at all; a hook that blocks has failed slowly (§9.30).
- **`install-hooks` and `uninstall-hooks` refuse to rewrite an unparseable
  `~/.claude/settings.json`** and write a timestamped `.board-bak-*` before their first
  change, never overwriting an earlier backup. Both are idempotent via the
  `<binary> notify` command marker. **Neither may change anything it did not write** —
  not another tool's hooks, not an unrelated key, and not the file's permissions (§15).
- **The same line holds on a maki `init.lua`, which board cannot parse at all.** The
  marked block is prepended, never appended — a Lua `return` must end its block, so an
  appended hook would be a syntax error in the file that starts maki. Markers that do not
  make sense (an opening without a closing, or the two the wrong way round) are a refusal:
  board cannot know where a half-deleted block ended, and the wrong guess deletes somebody
  else's program. The block itself is written to fail silently, because it runs inside
  maki (§17).
- **board never rewrites a `plugin.toml` it did not write**, and never deletes one that no
  longer carries its marker. That file is the permission policy for every bit of Lua in
  its directory, most of which has nothing to do with board (§17).
- **cmux env vars are always stripped from child processes.** cmux treats
  `CMUX_SURFACE_ID`/`CMUX_WORKSPACE_ID` as the implicit target of every command, so a
  stale inherited value makes even a global query fail (§9.8).
- **board opens nothing itself.** A row's preview and folder are hyperlinks; the URL goes
  to the terminal and the terminal is what launches a browser or an editor. board gains no
  opener, no `open(1)`, no second write action — `cmux surface.focus` is still the only
  thing it does to the world (§18). This is what keeps the surface that must never end a
  session from growing a way to start arbitrary processes.

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

### 10.5 Workspace-grouped layout — rejected, then shipped as §19 (2026-08-19)

The original entry read:

> Dense and pretty, but it answers "how is my fleet arranged", a question you never
> have. One workspace held 7 unrelated sessions; grouping would scatter the two things
> that need you across five boxes. Workspace stays a column.

**Both halves turned out to be measuring the same mistake.** The workspace was later removed as a
column too (§9.39), because on a one-session-per-workspace fleet it just repeated the label — so
the thing this entry protected did not survive either.

What was actually wrong was the *shape* proposed, not the grouping. "Scatter the two things that
need you across five boxes" is a real cost and §19 does not pay it: the bands stay outermost, so
NEEDS YOU is still one block at the top, and grouping happens **inside** a band. And "one workspace
held 7 unrelated sessions" was a fleet observation, not a law — the fleet that revived this has
workspaces holding one session and workspaces holding three, and §19 draws both with one rule.

*What changed:* the recognition that the two arrangements are two **mental models**, not one
correct answer — see §19.

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

### 10.10 A configurable editor — shipped as §18 (2026-08-19)

Parked for one afternoon and then built, because the first reader of the feature used Cursor.
The reasoning that parked it survived the build and shaped it: the key is a **name**, not a
template. `{"config": {"editor": "cursor"}}` holds one of `editor.Known`'s names and board
owns the URL's shape, because a template with a path substitution in it is one edit away from
being a command, and board opens nothing (§8).

What did not survive: "the URL has to be built before `view` sees it, so a config key means
it moves onto `Fleet`". It moved onto `Screen` instead — beside `Threshold`, which comes from
the same file and is the same kind of fact. `Fleet` never learns what an editor is, so
`internal/board` was untouched by the change.

### 10.11 The demo has no preview — deferred (2026-08-19)

`internal/demo` renders a fleet with no dev servers up, so the recording shows the folder
glyph on no row and the preview glyph nowhere. Adding either to the fixture is a frame
change, and a frame change means re-recording `docs/board.gif` — which needs `vhs` and is
a maintainer step by hand (§16).

Left alone deliberately rather than left undone: a fixture that renders links while the
committed GIF does not is a README that contradicts itself, and the honest intermediate
state is the one where nothing disagrees.

*Trigger:* the next re-record for any other reason. Two rows of the demo fleet want a
`Folder`, and one of them a `Preview`, at which point the GIF shows what the prose above
describes.

### 10.12 The review CTA — deferred, and not for the reason first recorded (2026-08-19)

Two glyphs were asked for: one to open the branch's pull request, one to replace it once somebody
has reviewed since the latest commit. **The link shipped as §18, from cmux. The CTA did not.**

The first version of this entry said the whole thing needed GitHub and deferred the CTA on cost.
That was wrong about the link — cmux had it all along (§9.42) — and it is still right about the
CTA, for a different reason: cmux's badge carries `open | merged | closed` and nothing about
reviews, so the CTA is the one half cmux cannot answer.

Nor is it measurable even with GitHub. "Since the latest commit was **pushed**" needs a push time,
and GitHub's `pushedDate` is deprecated and usually null — so the honest comparison is against
`committedDate`, which differs after a rebase and would put a call to action on a row nobody had
touched. A glyph that shouts on the wrong row is worse than no glyph, which is the rule §18 is
organised around.

*Trigger:* cmux carrying review state in its badge, which would make it free the way the link now
is. Failing that, `reviewDecision` alone would do — CHANGES_REQUESTED is your turn whenever it
arrived — but that needs the network back, and §9.42 is the argument against reaching for it.

### 10.13 Draft pull requests — deferred, cmux does not carry it (2026-08-19)

A muted `⧬` for a draft was asked for alongside the three states that shipped. cmux cannot supply
it: `PullRequestStatus` maps GitHub's `state`, which is `OPEN` for a draft — draft-ness lives in a
separate `isDraft` field cmux does not read. So a draft arrives as open and renders as open.

Not built as dead code waiting for data that never comes, and not built by asking GitHub, which is
the dependency §9.42 just removed.

*Trigger:* cmux reading `isDraft` into its badge. The rendering is already decided — a muted `⧬`,
since a draft is a pull request nobody is being asked to look at yet — so it would be one case in
`prMark` and one line in the legend.

---

### 10.14 ⌘-click a label to focus its tab — blocked upstream (2026-08-19)

The obvious completion of §18: a row points at four things, and the one it *is* should be
reachable the same way. It is not built because **cmux registers no deep link**. OSC 8 hands the
terminal a URL, and cmux's `cmux:` scheme is an auth callback — verified against the app bundle and
its own docs, not assumed (`EVIDENCE.md` §9.49).

board can already focus a tab, on Enter, through `cmux rpc surface.focus`. What is missing is a way
for a *click* to reach that call, and board may not invent one: §8 forbids it opening anything
itself, and running a command from a click is precisely the opener that rule exists to prevent.

*Trigger:* a cmux build that answers something like `cmux://surface/<uuid>/focus`. The row already
carries the surface UUID, so this becomes a two-line change in `view/link.go` and a fifth glyph —
or, better, the label itself becomes the link, since the label already *is* the tab.

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

board is the smallest part of its own bug report. The rosters come from `claude agents
--json` and from a Lua block inside maki, and the tabs from cmux; none of the three is a
documented contract and two have moved before (§9.1, §9.3) — so "board is broken" is
unanswerable without every upstream version beside board's own. That is the whole command:
one line per tool, meant to be pasted.

**There is no version constant and no `-ldflags`.** `runtime/debug.ReadBuildInfo`
already knows: `go install ...@v0.1.0` stamps the tag, and since Go 1.24 a plain
`go build` inside the repo synthesises a pseudo-version from VCS, with `+dirty` when
the tree is modified. **The tag is the release** — nothing to bump, so nothing to
forget to bump. A build with no VCS directory reports `(devel)`, which is a fact about
how that binary was made and is passed through rather than laundered into `unknown`.

The upstream strings are printed **verbatim**. Parsing them to a bare number would
put a parser over three undocumented surfaces on the far side of someone else's machine,
which is §9.1's mistake in a new place; cmux's build number and commit are also the
next thing anyone would ask for. The single exception is a leading token that repeats
the label — `cmux --version` names itself and `claude --version` does not — dropped so
the block reads, at a cost of one duplicated word if either format changes.

**Absence is reported, never raised.** This is the command someone runs when nothing
works, so a missing tool is `not found` and an unreadable board is `unknown`: distinct,
because one sends you to an install and the other does not. It cannot fail and cannot
print an empty field (§8, and §11 for the fixtures that pin it).

**A missing maki is still a line.** Its absence is the ordinary case rather than a fault —
board reports on whichever agents are installed (§17) — but omitting the row would leave
the reader unsure whether board looked, which is the confusion §14 exists to end.

`claude.Version`, `cmux.Version` and `maki.Version` live beside the other calls to their
own binaries, so everything that shells out to a tool is greppable in one package;
`host.Probe` holds the one shared judgement, which line of stdout to believe.

---

## 14. Trouble, and `doctor` — what board could not read

board reads three undocumented surfaces and used to throw away every failure of the first
two.
`claude.Agents` returned a bare `nil` on any error and on any shape it did not
recognise; `show` tested for cmux and `watch` did not. So a missing `claude`, a changed
roster schema, and a genuinely quiet fleet produced **the same screen**, and a
first `board watch` on a machine without cmux painted an empty dashboard explaining
nothing. On the author's machine none of this is visible. On anyone else's it is the
whole support burden (`EVIDENCE.md` §9.26).

**The fix is one concept, not four.** `Fleet.Trouble` is the phrase for what could not
be read, empty when the world was legible, derived in pure `Build` from a `Snapshot`
that carries `RosterErr`, `NoCmux` and `MakiErr`. Both renderers print that one field, for the
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
- **The order is by how much of the board the fact costs.** The claude roster outranks
  everything — with no roster there are no claude rows at all, and it is the bulk of most
  fleets. Then cmux, without which every interactive session loses its tab. Then maki,
  which costs one agent's rows. One line carries the most fundamental fact and names
  `doctor`, which carries them all.
- **A missing `claude` stops being trouble once maki is reporting.** board has a fleet on
  screen and the tool that is absent was a choice. It is the reports that settle that and
  not the binary: an installed maki nobody is running leaves board with nothing to show,
  which is exactly the state §9.26 is about.
- **`maki not reporting` needs both halves — a maki in a tab, and not one report on the
  machine.** cmux counts a surface's whole process tree, so any maki a script or a
  `maki --print` starts inside a tab looks like an unreported one; on a wired machine that
  line would appear every tick, and a signal that cries wolf stops being read. With no
  reports at all there is nothing to cry wolf about, and the phrase means what it says: the
  hook was never installed (§17).

**`doctor` is the escalation, and `version` stays.** They answer different questions —
`version` is four lines to paste, `doctor` is the diagnosis — so this is not §9.18's
two entry points for one thing. It embeds `version.Info` and calls `version.Format`, so
there is exactly one place that knows how to print a version string, and it aligns its
own five rows to `version.LabelWidth`.

Its rows are the wiring: the four versions, then the roster (a count, or the error's
own words — the frame's "· board doctor" would be circular here), the tabs (`0 tabs`
with a healthy roster is the shape of §9.3), the hooks (named individually, because a
half-install is a hook that never fires), the config path (and whether it exists yet),
and whether notifications are on.

**Two agents, and still one row each for `roster` and `hooks`.** Those are the concerns —
what board read, and how it was wired — so each row carries both agents rather than each
agent getting a row. Splitting by agent would put the label `maki` on two different lines
of a nine-line report, once as a version and once as a diagnosis. On a machine without
maki both halves say nothing at all: the version block has already reported it absent, and
nagging about a tool nobody installed is how a diagnostic loses its reader.

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

**The maki half is the same command and three more things to put back**: the block out of
`init.lua`, the `plugin.toml` board wrote — and only while it still carries board's marker,
because a user who has edited it owns it — and the reports directory, which the block wrote
but board is responsible for. An `init.lua` left holding nothing but whitespace is removed
outright: board created it, maki treats an absent file and an empty one alike, and an empty
file is a trace of a tool that has been removed. Same inverse test, on a Lua file this
time (§17).

`doctor` still only reads (§14). Diagnosing these files and changing them stay separate
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

---

## 17. maki — the second agent, and why it is read this way

board reported on one agent because there was one worth reporting on. maki is a second,
and the interesting thing about adding it is how little of board it touched: no new band,
no new state, no new column, no change to `view` at all. The three states are the fleet's
vocabulary and maki's `working`/`needs_input`/`idle` map onto them one for one, so a maki
session is a row like any other (§4). Everything below is about getting the rows.

**maki has no `agents --json`.** Its roster lives inside the running process — the UI
event loop owns every live session — and the only way in is the Lua plugin API. So board
installs a plugin. `hooks/maki.go` holds the whole of it: a marked block prepended to the
`init.lua` maki will load, which registers autocmds and writes what `maki.session.live()`
answers to `~/.board/maki/<cmux surface id>.json`.

That is the same shape as the Claude Code hook and a different mechanism, and the
differences are all consequences of the file being a program rather than data:

- **Prepended, never appended.** A maki `init.lua` returns its config, `return` must end a
  Lua block, and anything after one is a syntax error. Appending would break maki's
  startup on every machine board touched.
- **Markers instead of a parse.** board cannot decide whether an edit is safe by reading
  Lua, so the block carries its own boundaries and unbalanced markers are a refusal (§8).
- **The block must never fail maki**, which is `notify`'s rule one level in (§8). The
  top-level environment reads are inside a `pcall`, and the reporting task is handed to
  `maki.async.run`'s own error sink, so nothing board wrote can surface as a maki error.
- **`atomic_write`, so a reader sees the old file or the whole new one.** board's read is
  every ten seconds and the write is on every turn; without it the overlap is a truncated
  file and an unreadable roster (§14).

**It writes on state changes, not on a timer.** `SessionStatusChanged` fires exactly when
the transition board draws happens — including when a permission prompt opens, which is
maki's `needs_input` and board's `blocked →`. `TurnStart`/`TurnEnd`/`TurnError`,
`SessionFocusChanged` and `SessionReset` are also subscribed, so the set of sessions stays
current and a maki that has just started reports before its first turn.

**The idle clock is maki's own `updated_at`, and no file mtime is consulted.** A session
idle for three hours writes nothing, and its report still carries the second it last
moved. So a stale file is not a stale clock — which is what makes the write-on-change
design work at all.

**Liveness is a second read, and that is the part with no alternative.** maki fires no
shutdown event, so a report outlives the process that wrote it, and a report on disk is
not evidence of a session. `pgrep -x maki` is what says otherwise. Go's standard library
cannot enumerate processes, so this is one more subprocess on the tick — a few
milliseconds beside the roster's ~250 — and an exact-name match is the whole query.

**The two reads join through the pid map board already had.** `Build` walks the process
list, turns each pid into a surface through cmux, and looks the report up by surface. So
liveness, the tab, the workspace and jumpability all fall out of one join, and a report
whose maki has exited simply never matches. The walk is over the sorted pid list rather
than the report map, so two sessions that tie on band and idle time do not swap places
between ticks.

**Several rows can share one tab**, because one maki process holds every session opened in
it and `Enter` reaches the tab, not the session. That is honest rather than ideal: hiding
the sessions that are not focused would hide working ones, which is the inversion §8
rejects for dismiss. maki exposes `session.focus` to Lua and to nothing else, so a
per-session jump is not available to ask for.

**A maki running in no cmux tab is not a row**, exactly as an interactive claude session
with no surface is not one (§4). board is a view of tabs.

**Not installed is not a fault.** board reports on whichever agents are here, so
`maki.Available()` is asked before anything is made of a silent maki, and every row of
`doctor` that would mention it says nothing instead (§13, §14).

### The manifest is half the install

maki denies **every** permission to an `init.lua` that has no `plugin.toml` beside it. The
block installed, ran, read nothing, wrote nothing, and said so nowhere — found by running
maki 0.4.9, not by reading it (`EVIDENCE.md` §9.32). So `install-hooks` writes the
manifest too, and `MakiInstalled` reports the two halves separately: a block with no
manifest is installed and inert, which is not the same state as installed.

The manifest board writes grants the two permissions the block uses — `env` for
`CMUX_SURFACE_ID`, `fs_write` for the report — and **denies nothing**. An absent key means
allowed, so naming only what board needs leaves every decision about Lua the user adds
later to the user. A `plugin.toml` that is already there is never rewritten and never
deleted: it is policy for everything in that directory, and `install-hooks` says what the
block needs instead of taking the decision (§8).

---

## 18. Links — the three things a row points at besides its tab

`Enter` focuses a session's tab, and for two years that was the only place a row went.
Three more are worth reaching, and all three are properties of the same thing — the git
worktree the session is working in:

- **the preview**, a local dev server serving that worktree,
- **the Storybook**, a component workbench listening in it, and
- **the folder**, the worktree itself, opened in an editor to read the branch's diff.

They are rendered as a five-column cell at the right-hand end of the row — `⧆` pink for the
Storybook, `⧇` green for the preview, `⧉` cyan for the folder — and they are hyperlinks, not
keys.

**Order is by how often a row has one, rarest first.** The folder is on nearly every row, so
putting it last anchors the cell's right-hand edge and keeps the trailing trim from firing;
the Storybook is the rarest, so its empty column falls on the left where it costs nothing to
look at. Grouping the two http links together instead — which is the arrangement that reads
better as a sentence — spreads the gaps through the middle of the cell, and down a band of
twenty rows that reads as ragged rather than as a column.

### The worktree is the join, and a path prefix is not

Claude Code creates its worktrees **inside** the main checkout, at
`.claude/worktrees/<branch>`. So "the dev server's directory is below the session's" is
true of a feature branch's server and the main checkout's row at the same time, and a rule
built on it puts one branch's preview on another branch's row. A preview link on the wrong
row is worse than no link: it is a URL that looks like this session's work and is not.

`host.WorkTree` is what settles it — the nearest ancestor holding a `.git` entry, walking
up. A linked worktree's `.git` is a *file* pointing at the repository's gitdir, so it
answers itself and the main checkout answers the repository, and the two compare unequal
even though one contains the other. Both sides of the join go through the same function,
which is the only reason their answers are comparable.

It is a stat walk and not `git rev-parse --show-toplevel`, because this runs on every tick
for every session and a fork per row is the wrong shape for a reporting surface (§2).

Within one worktree there can be several previews — a monorepo running a dev server per
app. The nearest to the session's own directory wins, and the routes arrive URL-sorted so
a tie resolves the same way on every tick rather than drifting with map order. One row
shows one preview; a count of them would be a column reporting somebody's project layout.

### portless is the mechanism, and it is optional

`internal/preview` reads `~/.portless/routes.json`, which
[portless](https://www.npmjs.com/package/portless) writes: a hostname, a target port and a
pid per live route. Nothing in that file says where the pid is *working*, so the directory
comes from a second read — one `lsof -d cwd` over every route pid at once — and that is
what the join needs.

Two reads rather than one is the same shape as maki's roster (§17), and for the same
reason: **a route outlives the process it describes.** portless deletes nothing when a dev
server exits, so a route with no live pid, like a `portless alias` entry with no pid at
all, is not a link. `preview.Roster` therefore carries both halves — every route listed,
and the ones board could place — and `doctor`'s `links` row states them apart. Neither read
costs anything measurable: both hide behind `claude agents`, which is still the tick (§9.34).

portless is optional exactly as maki is. Without it, `Available` is false, nothing is read,
nothing complains, and every row still carries its folder.

### Storybook is found by its port, because there is nothing to ask

portless writes a state file and maki writes reports. Storybook does neither: no roster
command, no proxy registration, nothing on disk. What it has instead is a **well-known default
and a well-known way of leaving it** — 6006, then 6007, then 6008 as each is found busy — so
the identification is a bounded scan of that range plus the same worktree join everything else
here uses.

**The range is closed at 6020, and that is the interesting part.** TensorBoard also defaults to
6006 and also increments, so an open-ended "6006 and up" would eventually claim one and put a
Storybook link on a row that has no Storybook — the failure this whole section is organised
around. Fifteen ports is more than anyone runs at once. The second half of the identification
is the process: a JS runtime (`node`, `bun`, `deno`) by prefix, which is what separates
Storybook from the Python thing sharing its port range.

One `lsof -iTCP -sTCP:LISTEN -P` for the whole machine, not one per row. And it shares the
expensive half with the portless read: neither a route nor a listening socket says where its
process is *working*, so both feed **one** cwd lookup over the union of their pids. That is why
the second source costs nothing measurable on top of the first (§9.37).

`http://localhost:<port>`, not https — Storybook serves plain http. A Storybook put behind TLS
is a Storybook put behind portless, and it arrives through the other half of this package with
its own hostname instead.

### Hyperlinks, not keys — and this is the safety argument, not a shortcut

The cell is OSC 8: the URL goes to the terminal, and the terminal opens it. board therefore
gains **no opener** — no `open(1)`, no `code`, no second write action beyond
`cmux surface.focus` (§8). For a tool whose first invariant is that it can never end a
session, "the process on screen cannot start a browser" is worth more than a keystroke.

It also lands somewhere good. Under cmux an `https://` link opens in a browser tab beside
the fleet, and a `vscode://` link goes out through the OS to the editor — cmux routes any
non-http scheme externally. And the mechanism is not a gamble the way it would be in a
general-purpose tool: board requires cmux, cmux is built on Ghostty, and Ghostty honours
OSC 8. On every machine board supports, the link works.

**No legend line.** The keys are named at the bottom of the frame because a key you do not
know does not exist. A hyperlink advertises itself — Ghostty underlines it on hover and changes
the cursor, with no modifier needed, because board does not enable mouse reporting and links
are therefore evaluated locally — so a legend entry would spend a scarce line restating what
the glyph already does, and on a terminal without OSC 8 it would promise a click that never
happens. That is §9.14 read forwards: name the route that exists, and do not name one that
might not.

**But the gesture is ⌘-click, not click,** and that half is *not* self-advertising: Ghostty
opens a link only with the ctrl/super chord held at press, so a plain click on an underlined
glyph does nothing at all. The first reader of this feature hit exactly that, and the README
said "click them" and was wrong.

So the frame names it after all — `⌘-click opens` on the bottom line, and only while the cell
is on screen, which is the same condition the cell itself uses. Note what it is not: a key that
opens the link. Adding one would mean board launching a browser, which §8 forbids, so the hint
describes a gesture the terminal owns and board cannot perform.

**And `?` spells the rest out.** The meanings could not fit beside the idle scale — `⧆ storybook
⧇ preview ⧉ folder` is 36 columns on top of 87 — so the bottom line gained a third state
instead of a fourth element. `?` swaps it for the legend: the scale's rungs, all three glyphs by
name, the gesture, and `esc`. Same line, so the height cannot change (§12); a display mode like
the quiet fold, so it does not pause the refresh, because nothing on that line goes stale.

The legend shows all three glyphs whether or not the fleet has them. That looks like a breach of
§9.14 — name the route that exists — and is not: `?` is help, and a legend that hid a feature
nobody happened to be using that minute would answer a different question than the one asked.
The ambient line is where the "only what exists" rule applies, and it does.

Worth knowing that cmux fills in the other half unprompted: it renders the hovered link's URL
through its own `linkHoverIndicatorView`, with no modifier needed since board never enables mouse
reporting. So hovering `⧆` names `http://localhost:6006` — more precise than any legend board has
room for.

### Which editor, and why the chooser is a command

An editor is supportable exactly when it registers a URL scheme that takes a path, because
board hands over a URL and runs nothing. Three do, in the same shape — the path is whatever
follows `<scheme>://file`:

| name | bundle | scheme | how the shape is known |
|---|---|---|---|
| `cursor` | `Cursor.app` | `cursor://` | verified by hand; a VS Code fork |
| `vscode` | `Visual Studio Code.app` | `vscode://` | documented by VS Code |
| `zed` | `Zed.app` | `zed://` | `open_listener.rs` strips exactly `zed://file` |

So one template covers all three, and `internal/editor` only has to answer *which*.
`{"config": {"editor": "cursor"}}` pins it and `board editor <name>` writes that. Until then
board picks: the only installed one, or the first of `Known` when several are.

**`Known` is alphabetical, deliberately.** board has no opinion about which editor is better,
and any other order would be one — encoded in a table nobody reads, deciding for everybody
who never runs the command. Alphabetical is arbitrary in a way that is honest about being
arbitrary, and it is stable: a link cannot silently move because you installed something,
unless the new editor sorts first, and `doctor` names the choice either way.

A configured editor wins even when board cannot find its bundle. The user said so, `Lookup`
has already rejected the typos, and an app installed somewhere board does not look is likelier
than a lie. `board editor` and `doctor` both flag the mismatch — `zed, not installed here` —
rather than overruling it.

**The chooser is a command because the moment of the click is unreachable.** The obvious
design is the one macOS itself uses: the first time you open something, pick the app. board
cannot do that. The terminal opens the link and tells board nothing — no callback, no exit
code, no file touched — so there is no "first time this was opened" for board to hook, and an
editor board never launches cannot be chosen at launch time (§9.35). What board *can* observe
is "several editors are installed and you have not chosen", which is a fact available at any
time and therefore belongs in a command that answers it on demand:

    $ board editor
      folder links open  Cursor.app   cursor://   (automatically)

      → cursor   Cursor.app
        vscode   Visual Studio Code.app
        zed      not installed

      change it:  board editor vscode

A prompt inside `board watch` was considered and declined. It would cost the loop a second
mode — `watch` has exactly one, and §12 is the argument for keeping it that way — and it would
interrupt an ambient dashboard on first run to ask a question nobody had yet. `doctor`'s
`links` row carries the same answer without asking anything.

**No editor found means no folder glyph**, not a glyph pointing at `vscode://` on a machine
with no VS Code. `actionCols` and `actionCell` are held to the same predicate by a test, so
the column is not reserved for a cell that will not be drawn.

### The location column is git's, not cmux's

The tail column used to show the cmux workspace. On a real fleet that repeats the row's own
label, because cmux creates a workspace per agent task and titles it after the task, and board's
label precedence falls through to the tab title — four of six rows identical on the fleet that
found it (§9.39). It was spending ~45 columns to restate the label, and capping the label at 30
characters to do it.

It answers the same question in git's terms now: **the repository, and the worktree inside it**
when the session is in a linked one rather than the main checkout.

    app -> acme-1013-dataview-refactor
    date-invite

Both halves were already on the row — `Folder` gives the worktree, and `host.Repository` resolves
it to the repository through the `gitdir:` pointer in the worktree's `.git` file. The repository
is memoised on the worktree, so several sessions in one resolve it once, and there is no new
subprocess.

The worktree half takes the preview's green, not a new colour: green already means "the live
thing" on the preview glyph and a worktree is the live branch, so the two readings agree. No
brackets — the colour is the separation. A main checkout prints one name, because there the
repository *is* the worktree and naming it twice would be the duplication this removed.

`Row.Where()` is the plain-text form both renderers size and print from, so they cannot disagree
about what the column says (§3); `board.TreeArrow` is exported because the frame has to find the
seam to paint the halves while the table prints it whole. `Find` matches the location as well as
the label, since the column is what a reader can see — and still matches the cmux workspace,
which is no longer drawn but is still the fleet's spread in the header.

**The header still counts workspaces**, which is now a fact the column does not show. Left that
way deliberately for one release: it is a true statement about how far the fleet is spread, and
changing it to count repositories is a separate decision about what "spread" means. It is the
first thing to revisit if the header and the column start reading as contradictions.

### The pull request comes from cmux, not from GitHub

`⧬` opens the pull request the row's branch has open. It is the fourth link and the only one that
does not point at this machine — a port, a port and a directory, then github.com — which is why it
sits beyond the other three at the far right rather than where its frequency would put it.

**cmux has already correlated it.** It polls GitHub itself and shows a badge per tab in its
sidebar; `cmux sidebar-state --workspace <uuid>` hands the same thing over. So this is enrichment
from cmux exactly as tab titles and the idle clock are (§3): board makes no request, needs no
credential, holds no cache and gains no config key. One call per workspace the fleet occupies,
run concurrently, hiding behind `claude agents` like everything else.

The flag is `--workspace` and this is worth writing down because getting it wrong cost a day:
`--tab` is not a flag `sidebar-state` accepts, so passing it is silently ignored and the app
answers about the *selected* tab. That makes every workspace look like it returns one tab's data,
which reads as an addressing dead end and led to a whole networked package being built and thrown
away (§9.42).

Keyed by workspace UUID rather than by worktree, because that is the question cmux answers: one
badge per tab. Two maki sessions in one tab therefore share a pull request, correctly — it is the
tab's branch. A background agent has no tab and so has none.

**Three states, and the difference is the point.** cmux reports `open`, `merged` or `closed`, and
the frame draws them apart: `⧬` hollow in blue for open, `⧭` filled in blue for merged, `⧬` in
GitHub's own red for closed. Shape carries "has it landed" and colour carries "did it land
anywhere", so a reader who misses one cue still has the other. An open pull request is something to
go and look at; a merged one is context; a closed one is a branch somebody abandoned. A state cmux
has not had yet draws as open, because a glyph board cannot classify is still a pull request worth
reaching.

### Where a ⌘-click lands is cmux's decision

board writes the URL; cmux routes it. An http link opens in a browser tab inside cmux by default
and everything else goes out through the OS, and whether the http half stays inside is cmux's
preference `browserOpenTerminalLinksInCmuxBrowser`.

board **reports** it and never writes it. Reconfiguring another application is the line §8 already
draws around a `plugin.toml` board did not create, and one tool silently reconfiguring another is
how both become hard to trust. So `doctor`'s `links` row ends with `cmux browser` or `system
browser`, and the README carries the `defaults write` that changes it.

### The frame has links; the one-shot table does not

`board` prints for a pipe, a scrollback and a bug report. Escape sequences do not belong in
a file, and a preview hostname is derived from a branch name, which is work data — the same
reason `doctor` never prints `notify_cmd` (§14) and the same reason the demo is a fixture
(§16). So the derived fields live on `Row`, where both renderers can see them, and only the
frame draws them. That is the rule working, not an exception to it: a derived quantity goes
on `Fleet`/`Row` so the renderers cannot disagree about *what is true*; which of them draws
it is still each one's own business (§3).

### What the cell costs, and what it never costs

`actionCols` is the single place that decides whether the column exists, read by both the
arithmetic and the rendering so they cannot disagree:

- **Nothing, on a fleet with nothing to point at.** No row with a `Preview` or a `Folder`
  means no reserved columns and no padding, and the frame is byte-identical to the one
  before links existed. Every golden frame in the suite still passes unblessed, which is
  what says so.
- **Nothing, on a terminal too narrow for a bare row beside it.** Shed whole, like the KPI
  strip's cells: half a link cell is a glyph that no longer lines up with the one above it,
  and that reads as a rendering fault rather than as an absent link.
- **Never the label's floor.** The cell comes out of the surplus the bar would otherwise
  take, after the label and the workspace column have theirs (§9.29).

Two predicates decide whether the cell exists — `pointsSomewhere` for the arithmetic and
`actionCell` for the rendering — and a test holds them to the same answer. Disagreeing either
reserves a column nothing fills, or draws a glyph nothing reserved room for, and the second one
wraps the frame.

Colour is `inkSecondary`, already validated: these are affordances, not data, and exactly
one element in the frame is allowed to shout (§6). The glyphs stay inside the Unicode
blocks the rest of the frame draws from — a glyph that falls back to another font is a
glyph whose width board guessed wrong, and the fit is a hard rule.

One consequence worth naming: a hyperlink is the first escape sequence in the frame that
is **not** SGR, and it is terminated differently. `printed` and `clampLine` now share one
scanner, and a clamped line closes a link as well as a colour — an open one makes every
cell after it part of the link (§9.34).


## 19. The workspace is a group, not a column

A cmux workspace means two different things to two different people, and board had been built as
though it meant only one.

**Model one: a workspace is a project.** Several agent sessions live in its tabs, on several
branches, alongside whatever else that project needs. The workspace name is the umbrella, and
which sessions share one is real information.

**Model two: a workspace is a task.** One agent session, plus the dev servers and tools that
session needs. The workspace name and the session label are the same string, because cmux titles a
workspace after the task it was created for.

board's history is the two models colliding. The workspace started as a column, for model one. It
was removed as a column (§9.39) because on a model-two fleet it restated the row's own label on
four of six rows — which is true, and was the right call, and quietly dropped the one thing model
one wanted. It comes back here as a **grouping**, which is the form that serves both: model one
gets its umbrella, and model two never sees it.

### A group earns its header by exception

One session in a workspace draws **no header**. Two or more draw one, named after the workspace.

This is §9.13's rule — a band earns its lines by exception — applied to groups, and it is what
makes the feature free for model two. Naming a solo workspace would restate the row's own label,
which is precisely the duplication §9.39 removed; and on a fleet with one session per workspace it
would spend a line on **every row**, roughly doubling the frame's height to say nothing. Two
sessions is the first point at which the name tells you something the rows do not.

The consequence worth stating: a workspace whose sessions are spread across bands is named in the
bands where it has two, and railed but unnamed where it has one. That is the §10.5 trade taken
deliberately — urgency outranks arrangement, so the bands stay outermost and a group never pulls a
blocked row out of NEEDS YOU to sit with its siblings.

### The rail costs no columns, because the lead was already blank

Every row's lead was three columns: a blank, the selection caret's column, a blank. The rail takes
the outermost one. Nothing moves, no column is bought from the label, and the frame's width
arithmetic (§6) is untouched — which is the only reason this was affordable at all.

    ▌  ◐   backfill refunds worker        0m  app -> pla-982-refund-backfill
    ▌▸ ◐   stripe webhook retries         4m  app -> pla-983-webhook-retry
       ○   bump the staging image        26m  infra

The rail is `▌`, a filled block rather than a line-drawing character: it is a colour field, not a
border, and it is what cmux itself draws down the side of its sidebar — so the two surfaces read
alike. A workspace with no colour draws **no** rail, because a mark that is on every row marks
nothing.

The header sits at column 3, one inside the band header at column 2 and four outside the label.
Aligning it to the label column was tried first and is wrong: the gutter is elastic (§9.41), so one
blocked row anywhere on the fleet widens it to twelve and drags every group name out to meet the
labels, stranding it nine columns from the rail that owns it.

**Height needs no new arithmetic.** `Frame` measures the composed string rather than estimating
chrome (§9.10), so header lines are budgeted by construction and the shedding ladder converges on
them like anything else.

### The colour is the user's, so the *function* is what gets validated

§6 says colour is validated and never eyeballed, and every value in `palette.go` was measured by
hand. A group's colour cannot be: it is whatever the user picked in cmux, and board sees it for the
first time at runtime.

So board validates the transform instead. `groupColour` lifts the lightness until the value clears
the ink floor against both documented backgrounds, and a test sweeps the whole hue wheel plus the
real cmux values asserting it. Every colour board draws is that function's output, so holding the
function holds every group (§9.48).

Lifting is necessary, not cosmetic — cmux's palette is built for a filled rail on the app's own
background, so its values are dark. Hue is preserved and only lightness moves, because hue is the
entire signal: a lift that slid magenta toward pink would land it on the storybook glyph's colour.

A colourless group is white and **underlined**. The underline is doing the job the hue would have:
it marks the name as a group name without inventing a colour the user did not choose.

### It costs no subprocess

The colour arrives on the `sidebar-state` call board already makes for the pull request (§18) —
one dump, three facts. Grouping is affordable on a surface that refuses to poll anything (§2)
precisely because it added no read.

### The pull request had to be re-joined on the branch

Grouping is what exposed this. cmux correlates one pull request per **workspace**, keyed off that
workspace's own directory — and board took it unchecked. But Claude Code puts its worktrees
*inside* the main checkout, so a session in a linked worktree sits in a workspace whose directory
is on an entirely different branch.

On the fleet this was built against, one row's location column said `app -> pla-1013-dataview-…`
and its pull-request glyph pointed at #1709, which belongs to `pla-138-final-rollout-tweak`. The
row was asserting two contradictory things at once.

The fix is `host.Branch`, and the rule is the one the previews were already built on: a link that
looks like this session's work and is not is worse than no link (§18, §9.34). The branches must be
known and equal, or nothing is drawn.

### What is deliberately not here

**Groups are not cmux's groups.** cmux has a first-class workspace-group concept —
`workspace.group.list`, `.set_color`, `.collapse` — and it is the natural home for this if people
start using it. Nobody on the fleet this was built against had one defined, so the workspace is the
grouping key today. Revisit when `workspace.group.list` comes back non-empty.

**The one-shot table is ungrouped.** Header rows would break the one thing a table is for, which is
being parsed by something downstream — the same argument that keeps links out of it (§18).

**The header still counts workspaces.** It said so before grouping and it still does, and now the
frame shows the same fact a second way. §18 left that to be revisited and this does not settle it.
