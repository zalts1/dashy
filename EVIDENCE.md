# board — evidence log

The append-only half of the design record. **`DESIGN.md` is the settled contract; this
file is why it settled that way** — what was believed, what falsified it, what shipped.

**Section numbers are stable and are cited from code.** Grepping `§9` across `*.go`
finds every function a finding below constrains. Append §9.16 and up; never renumber.

Read one section, not the file:

```sh
grep -n '^### 9\.' EVIDENCE.md          # § → line
sed -n '<start>,<end>p' EVIDENCE.md     # or Read with offset/limit
```

## Index

| § | finding | constrains |
|---|---|---|
| 9.1 | Blocked detection: three rules, two wrong — derived state must be pinned by a fixture | `board/build.go`, `claude/claude.go` |
| 9.2 | `needsInput` means "sitting at the prompt", not "asked you something" | state model (`DESIGN.md` §4) |
| 9.3 | The roster was blind to 10 of 31 sessions — cmux is enrichment, never the roster | `cmux/cmux.go`, `board/build.go` |
| 9.4 | Colour: the most important element had the worst contrast; bare red measures 2.91 | `view/palette.go`, `view/header.go` |
| 9.5 | `/dev/null` is a character device | `host/` child processes |
| 9.6 | The ledger header claimed a window the data did not cover | removed with `ASKED` (§9.13) |
| 9.7 | Jump must not exit the process | `watch/`, `cmux/focus.go` |
| 9.8 | cmux inherits its target from the environment — always strip its env vars | `cmux/top.go`, `cmux/focus.go` |
| 9.9 | The labelling assumption did not hold — users do not label sessions | `DESIGN.md` §10.2 (deferred) |
| 9.10 | "Chrome is fixed" was wrong, and it ate the header — **height** is arithmetic | `view/layout.go`, `view/format.go` |
| 9.11 | Restructure into packages | the tree (`DESIGN.md` §3) |
| 9.12 | The frame was wrapping the whole time — **width** is the same invariant as height | `view/layout.go`, `view/format.go` |
| 9.13 | `ASKED` was deleted, in two passes — a band earns its lines by exception | `DESIGN.md` §5 |
| 9.14 | `⌄ 3 more` promised a key that did not exist; every row must be a nav stop | `view/order.go`, `view/frame.go` |
| 9.15 | Jump landed on the right workspace and the wrong tab | `cmux/focus.go` |
| 9.16 | The design record cost 11k tokens to read one section of — split, indexed, deduplicated | `CLAUDE.md`, `DESIGN.md`, `view/doc.go` |
| 9.17 | §9.9 was evidence about labels, not about todos; `TodoWrite` is 3/100 — todos ship capped at 10 | `DESIGN.md` §12, `config/`, `board/build.go` |
| 9.18 | Two entry points for one thing was the friction — capture moved into the frame | `watch/keys.go`, `watch/todo.go`, `view/frame.go` |
| 9.19 | The bar meant two different clocks, and the band sat inside a ranking it is not part of | `view/frame.go`, `view/format.go`, `view/layout.go` |
| 9.20 | The legend was keyed by round numbers, not by rungs — a colour in use had no key. Label restyling tried and reverted: board reports names, it does not edit them | `view/scale.go`, `board/build.go` |
| 9.21 | The quiet band folds on `z`, open by default — the fold is the reader's, and a folded row is not a navigation stop | `view/frame.go`, `view/order.go`, `watch/keys.go` |
| 9.22 | `module board` built for a year and could never be installed — a module path is an address, and "no dependencies" hid it | `go.mod`, every import, `README.md`, `CLAUDE.md` |
| 9.23 | The toolchain already stamps the version — `-ldflags` was cargo; and fixtures passed while the real output stuttered | `version/`, `host/probe.go`, `DESIGN.md` §13 |

---

## 9. Evidence log

What was believed, what falsified it, what shipped. Nothing here is a style
preference; each cost real debugging time, and several looked correct in the code
while being wrong against the data.

### 9.1 Blocked detection: three rules, two wrong

An earlier version concluded from measurement that blocked "will stay mostly empty —
median 0s, p90 7s" and that rot, not live blocking, was where the value lay. **The
conclusion was wrong, from a broken measurement.**

1. *Newest event is an actionable card.* Never fires — cmux logs the card and its own
   `toolUse` in the same second, so the `toolUse` always displaces the card.
2. *Treat the actionable `toolUse` as the request and its `toolResult` as the answer.*
   Also wrong. cmux's Feed bridge emits a `toolResult` when its ~6s semaphore
   expires; Claude then falls back to its own in-terminal picker and waits
   indefinitely, emitting nothing. The 6s figure was the bridge timing out, not a
   human answering.
3. **Shipped:** scan backwards, treat `toolResult` as inconclusive, and let the first
   `stop` / `userPrompt` / non-actionable `toolUse` prove the agent moved on.

Re-measured with rule 3: p90 went from 7s to 511s, max to ~24h, and **321 episodes
ran over a minute, 160 over five.** Blocked is a real, frequent, long-lived state.

Live state now comes from `claude agents` instead (§9.3), but the *ledger* still
needs this rule, and it lives in `Event.Settles`. **Do not simplify it back.** What
killed rules 1 and 2 was watching a real session sit unanswered on screen while the
log claimed it had resolved — derived state must be checked against the thing itself,
and pinned by a test over a fixture.

### 9.2 `needsInput` means "sitting at the prompt"

It fires ~60s after any finished turn and covered 16 of 21 sessions. Never surface it
as `waiting`. The failure mode of a too-eager signal is **muting, not missing**: a
channel that fires constantly gets ignored, which also silences the rare message that
mattered.

### 9.3 The roster was blind to 10 of 31 sessions

Two of them background agents blocked for 44 and 52 days. board's own promise is one
line per session, so this was a correctness bug, not a missing feature.

Cause: roster *and* state both came from cmux's hook file and audit log. **cmux's
hook file does not register every session** — the 8 interactive sessions it missed all
owned real cmux tabs with real titles, and `cmux top` knew their pids the whole time.
Background agents were never in scope for that file at all.

Fix: `claude agents --json` is the roster; cmux is enrichment only. This deleted
`blockedSessions`, its log heuristics, `readSessions`, and the pid-liveness check
(`claude agents` only reports live sessions). **Never use the hook file as the
roster.**

### 9.4 Colour: the most important element had the worst contrast

Bare `#d03b3b` text measures **2.91** on the default terminal background. Rendering
blocked as a filled badge with white text moves the text onto our own fill and
sidesteps the user's theme entirely. Status colour never carries meaning alone
anyway: badge + the word `BLOCKED` + the band header is the required icon+label
pairing.

**The idle ramp had to brighten with age, not darken.** On a dark surface the
sequential anchor flips; the documented dark floor `#184f95` fails the 2:1 ordinal
gate at 1.73. Steps also had to skip every other rung — adjacent ramp steps measured
ΔL 0.049, under the 0.06 minimum. A test asserts the ramp's luminance is strictly
increasing, so this cannot be reverted by accident.

### 9.5 `/dev/null` is a character device

A `ModeCharDevice` TTY check made `board watch >/dev/null` spin forever. Detection
asks the kernel for a window size instead.

### 9.6 The ledger header claimed a window the data did not cover

Two more defects found only by rendering against real data: the same session was
named two different ways (cwd basename in `ASKED`, workspace title everywhere else),
and bars in `ASKED` were meaningless (§6).

### 9.7 Jump must not exit the process

The first version called `return` after focusing, which exited and left a dead BOARD
tab to come back to. "The tab is no longer on screen" is not the same as "board
should stop."

### 9.8 cmux inherits its target from the environment

A stale `CMUX_SURFACE_ID` makes even a global query fail. Also: `surface.focus` takes
**`surface_id`**, not `surface`; the latter returns `invalid_params`.

### 9.9 The labelling assumption did not hold

The derived-todo model in §10.2 assumes you label sessions. After a week of
real use
`~/.board.json` held **one label**. That is why intents are unbuilt rather than
deferred-with-a-plan, and why label precedence falls back through Claude's own
`needs` string and the cmux tab title (§4) instead of relying on user input.

### 9.10 "Chrome is fixed" was wrong, and it ate the header

§6 said the height budget was safe because chrome is fixed and the quiet
tail absorbs
the remainder. The budget hard-coded chrome at **13 lines when it is really 15**, and
the watch loop's trailing newline costs one more. So with a full fleet — 3 blocked, 2
working, 26 quiet, 7 asks in a 52-row tab — the frame came to 55 lines, the terminal
scrolled, and the **first thing written was the first thing lost: the `BOARD` header
with the clock and the refresh interval.** It looked like a missing feature, not an
overflow, which is why it went unreported for a while: with a smaller fleet it fit.

Shipped: **fit is measured, not estimated.** `Frame` composes, calls `height`, and
trims until it fits — quiet tail first (it is the designed absorber), then ASKED rows
(their header keeps the aggregates), then a hard clip of trailing lines as a backstop
on a very short tab. Cutting from the bottom is deliberate: losing the legend costs a
key, losing the header costs the clock and the blocked count.

Two lessons worth keeping:

- **A layout constant that duplicates the layout will drift from it.** The estimate
  was correct when written and silently wrong two features later. Measuring cannot
  drift.
- **One of the four golden frames was already overflowing** (the 24-row case, by 3
  lines) and nobody noticed, because a golden file pins *what is rendered*, not
  *whether it fits*. Both are now asserted, across a matrix of terminal heights and
  fleet sizes.

### 9.11 Restructure into packages (2026-07-28)

Flat single-package code was reorganised into `cmd/` + nine `internal/` packages
(§3). Two notes for anyone auditing that change:

- **The frame is pinned by golden files** captured from the pre-split renderer, so
  the extracted `view` package is provably byte-identical. Re-blessing them is a
  deliberate act: `BLESS=1 go test ./internal/view`.
- **Sorting changed from `sort.Slice` to `sort.SliceStable`.** Rows with identical
  idle times now keep roster order instead of an unspecified permutation. This is the
  only intentional behaviour change in the restructure.

### 9.12 The frame was wrapping the whole time (2026-07-28)

§9.10 fixed height by measuring it. Width was never measured at all, and it was wrong
on the real fleet at every terminal size tried — 17 to 21 columns over at 60, 80, 100
and 118 cols:

    cols=118  widest line=135  ASKED    *** WRAPS by 17 ***
    cols=80   widest line=97   ASKED    *** WRAPS by 17 ***

Three findings, in the order they turned up:

1. **The label column reserved a hard-coded `46` columns for everything to its right.**
   The real chrome is `rowChrome` = 38 **plus the tail**, and the tail is user data:
   `platform-migration` is 18 columns, and an ASKED row's is `HH:MM ` plus that. So
   the reserve was short by the whole tail. Same failure as §9.10, same cause — a
   constant standing in for a measurement — and it was invisible for the same reason:
   `board watch` is normally read, not diffed.
2. **The first measurement was wrong too.** `awk length()` counts bytes, and `▇` is
   three bytes for one column: it reported 150 where the truth was 135. Any width
   assertion has to strip SGR codes *and* count runes. The test helper made the same
   mistake a second time, on `strings.Index`, and reported columns 26 apart that were
   in fact identical.
3. **ASKED's wait column sat one column left of `IDLE`** — `barCells+3` where a row
   spends `barCells+4`. Two renderings of the same quantity, one column apart, for as
   long as the band has existed. Both now derive the gap from one `midCols`.

The lesson is the §9.10 lesson again: **the frame's fit is arithmetic, and arithmetic
gets pinned by a test over a matrix, not by reading the function.** So there is now a
belt as well as braces — `clampLine` makes wrapping impossible even if the arithmetic
is wrong again.

### 9.13 ASKED was deleted, in two passes (2026-07-28)

**Pass one — rendering faults.** Read live, the band was eight lines of noise:

1. **It showed the header chip, not the question.** `Header`, `Constraints`,
   `Placement`, `Next step` — `AskUserQuestion` headers are ≤12-char tags designed to
   be read *beside* the question, so alone they name no decision. The `prompt` field
   with the actual question was in the same payload the whole time. Worse, a question
   whose payload had no header fell through to the `"plan review"` default and was
   labelled as something it was not.
2. **Every ask took a row, so the one that mattered was buried.** Six rows, five of
   them answered in 1–5 minutes. "You replied in 2m" is a receipt, not
   accountability; the single interesting cell — `5h01m`, one workspace — sat
   among five identical-looking siblings.
3. **Ambient noise outranked live sessions.** The §9.10 trim order sacrifices the
   quiet tail *before* ASKED rows, so a short tab hid real sessions to keep showing
   asks you had already answered.

Filtering to exceptions (open, or waited >15m) fixed all three and the band still read
as worthless. That is the tell: the problem was never the filter.

**Pass two — the measurement that ended it.** Over the **whole** 62MB log, not the
tail board reads:

    172 asks over ~2 months
      4 never settled — all in June, none in July
     31 settled but waited ≥15m   (~0.5/day)
    median wait 230s · p90 2361s · max 85567s

So the payoff §5 claimed as unique — `never`, "nothing else in the stack
surfaces
those" — fired **4 times in two months and zero times in the 6-day window the band
actually displayed.** The band was drawing 8 lines and re-parsing an 8MB tail every
10s to report a number that was 0.

And `internal/workstream` fed nothing else: no other consumer of `Snapshot.Events`
existed. So the whole package went with the band.

Two rules this settles:

- **A band earns its lines by exception.** A row reporting the system working as
  intended is a row that hides one that isn't. The original "rows are the last 24h"
  rule was the wrong axis — recency is not relevance.
- **Ask what action a row enables before asking how to render it.** Three renderings
  of ASKED were argued about; none of them could have worked, because every row it
  could contain was either history or a duplicate of `NEEDS YOU` (§5).
  Measuring the
  payoff's frequency over real data ended in one query what redesigning could not.

### 9.14 `⌄ 3 more` promised a key that did not exist (2026-07-28)

Asked of the live board: *"there's the `3 more`, but no way to see the 3 more — is
that on purpose?"* The collapse is on purpose (§8: QUIET only grows, the
count carries
the signal). The chevron was not: it is a disclosure control, and nothing expands the
band. **An affordance is a claim about what the tool can do; drawing one it cannot
honour is the same class of defect as reporting a wrong number.** The line now reads
`+N quiet`.

**Two ways in already existed, both undocumented.** `↓` past the last visible row
walks into the collapsed tail — `DisplayOrder` is built from `Bands()`, not from what
was drawn, and `pickQuiet` pins the selected row into view wherever it sits. And a
taller tab expands the band on its own. The README now says the first one.

**The real defect was underneath.** `DisplayOrder` skipped rows with no surface, so a
tab-less background agent sitting in the collapsed tail was counted twice on screen —
in `QUIET · N` and in the hidden count — and could never be drawn by any key. Found by
fixture probe, not by looking: the live fleet's two background agents were both
`blocked`, and `NEEDS YOU` never collapses, so nothing was visibly wrong.

The fix separates two things the surface id was doing at once:

- **`Row.Key` (the session id) is what selection is keyed on.** Every row has one.
- **`Row.Surface` is only a jump target.** `Jumpable()` is now about Enter alone;
  Enter on a tab-less row sets the notice `no tab to jump to`.

Selection was never only a jump cursor — it is also the one thing that lifts a row out
of the collapsed tail — so keying it on the jump target excluded exactly the rows that
could not be seen another way.

**Expand and scroll were both rejected.**

- **Expand collapses into scroll.** A band that does not fit cannot be expanded in
  place without overflowing the frame (§9.10), so expand needs a viewport, and then it
  *is* scroll.
- **A scroll offset re-introduces position.** §7 keys selection on identity
  precisely
  because rows re-sort under the cursor every tick; an offset is an index by another
  name and would drift the same way.
- **The payoff is inverted.** QUIET is sorted by idle descending and the collapse
  always cuts from the fresh end, so **every hidden row is less rotten than every
  visible one.** Paging through them is paging through the sessions least likely to
  need you.
- **Growing the tab already does it**, with no state that a resize could invalidate.

**One claim withdrawn.** It looked like the pinned row was drawn out of order — a 5h
bar under 1d bars reads as a broken scale. It is not: the tail is the fresh end, so
appending the pinned row is its correct position. The bars jump because rows are
skipped, not because the order is wrong. Noted here because it survived a reading of
the code and died against a fixture, which is §11's whole argument.

### 9.15 Jump landed on the right workspace and the wrong tab (2026-07-28)

Reported of the live board: *"it jumps to the workspace but the last used tab in this
workspace and not the exact tab."*

**cmux keeps two states where board assumed one.** Which tab a pane is showing
(`surface.selected`) and who owns the keyboard (`surface.focused`) are separate, and
the event log emits both names. `surface.focus` only ever did the second: it raises the
window and selects the workspace, then the pane re-asserts whichever tab it last
showed. Proven by calling it on a background tab with its workspace **already
selected** — the pane's `selected_surface_id` still did not move, so this was never a
race with workspace selection. `focus-panel`, documented as an alias over surface
focus, behaves identically.

**The surface id was right the whole time.** The join is fine; the method was wrong.
That is worth stating because §9.8 already recorded a bad *parameter* on this same
call, and the next reader will assume the same cause.

**It was silent because the reply lied.** `surface.focus` returns a success payload
echoing the very surface it declined to select, so `strings.Contains(out, "error")`
saw nothing. `selectTab` now confirms against the tree instead of the reply — the only
check that would have caught this. Every band in §9.1 taught the same lesson about
derived state; this is the same lesson about a write.

**cmux has no verb for "front this tab."** `tab-action` covers rename, close, pin and
`mark-unread`; there is no select. Three calls do it, all verified live:

| call | selects | cost |
|---|---|---|
| `surface.reorder` onto its own slot, `focus:true` | yes | 1 call, no-op on the strip |
| `surface.move` onto its own pane/index, `focus:true` | yes | same, but takes an index |
| `notify` → `open-notification` → `dismiss` | yes | 4 calls, flashes a notification |

**Placement is by neighbour identity, not by index.** `surface.move` wants an index,
and every index after a closed tab shifts. board reads the tree up to a tick before it
writes, so a stale index would silently *reorder somebody's tabs* — the one failure
mode worth designing out, since §8's whole claim is that board is safe to
install.
`surface.reorder` takes `after_surface_id` / `before_surface_id` instead: if the anchor
is gone the call fails loudly rather than moving a tab somewhere wrong. A tab that is
first in its strip anchors on its successor instead.

**Both lookups now come from one walk.** `parseNodes` returns every surface with the
context its ancestors carry, and `parseTop` and `findSlot` derive from it. The tree
shape has changed before (§9.3); a second hand-written walk is a second thing to get
wrong.

Reported upstream against cmux 0.64.11. If `surface.focus` starts selecting, delete
`selectTab` and keep the confirmation.

### 9.16 The design record cost 11k tokens to read one section of (2026-07-29)

`DESIGN.md` was 830 lines, and 25 code comments cited sections inside it. Nothing
resolved a `§` to a location, so "read the section covering what you are changing"
was in practice a whole-file read — ~11k tokens to reach a ~30-line section, on every
task. Three changes, none to behaviour:

- **§9 was split into `EVIDENCE.md`.** It was 313 of 830 lines and it grows on every
  finding, so the append-only half was inflating the file every change has to read.
  The settled contract is now ~500 lines and the log stands alone.
- **`§` numbers were kept exactly, and stayed bare.** They are cited from code, so
  renumbering would break 25 comments the way the 2026-07-28 renumbering did — the
  old mapping table is gone from the header because the numbers are now stable. Bare
  rather than file-qualified, with the resolution rule stated once per header, so a
  reference does not have to be rewritten if a section ever moves file again.
- **Both files carry a `§` index.** ~35 lines that name what each section settles and
  what to read it before changing, so the section is chosen without opening any of
  them.

Two things were deduplicated at the same time, because a fact in three places is
three things that can disagree:

- **The package tree** lived in `CLAUDE.md`, in §3, and in the package comments.
  The package comments cannot drift from the code, so `go doc ./...` is now the
  authority — 1.2k tokens for all eight packages, and it needed a package comment on
  `view`, the only package missing one and the one with the most fit bugs behind it.
  §3 keeps the reasoning; `CLAUDE.md` keeps a routing table.
- **The invariants** were stated in `CLAUDE.md`, §2 and §8, with `#282c34`/`#040404`
  and a `view/frame.go` line count copied into the always-loaded file. The line count
  was already stale by one. `CLAUDE.md` now carries the rule and a `§` pointer, and
  nothing else: rules are imperative and stable, specifics live in the section where a
  change to them gets recorded.

The tests did not change and could not have caught any of this, which is the point —
this was a documentation-addressability change, not a behaviour change.

### 9.17 §9.9 was evidence about labels, not about todos (2026-07-29)

Todos were refused twice on this file's own authority before the refusal was checked,
and two of the three arguments did not survive the check.

**What held.** "Not a task manager" (§1) still rules out status, priority, due dates
and stages. §9.13's rule still rules out a band of rows that report nothing wrong.

**What did not.** §9.9 — one label in a week of real use — was read as evidence that
capture does not happen. It is not: a label is decoration on a row that renders either
way, so skipping it costs nothing and measures nothing. The case it was cited against
is a customer request with no session behind it, where not capturing it means it is
gone. Same keystrokes, opposite stakes. A refusal that transfers evidence between
different stakes is a refusal that has stopped reading.

**What the measurement actually killed** was the other reading of "derived". Parsing the
agents' own todo lists would have been the zero-capture-cost source §10.2 wanted:

    ~/.claude/todos/            does not exist
    3 of 100 transcripts        touched in the last 7 days contain a TodoWrite call

So the derived-from-`TodoWrite` design would have rendered blank. The derived list that
does work needed no code at all: a live session already is a row with a name, a rank
and an age, and reading the fleet as a list of things to do is what §1 produces.

**What shipped** is §12 — the intent half of §10.2, capped at 10, captured from the CLI
and removed from the frame. The cap is the part to defend: it is what keeps the band a
reminder rather than a backlog, and it is enforced by refusing the 11th rather than by
trimming the oldest, because the refusal is what forces a decision.

*What would falsify this, measured over two weeks:* todos added, todos removed as done,
median age at removal. A growing count with a two-week median means the band is a
backlog, and §12 goes back to being §10.2.

### 9.18 Two entry points for one thing was the friction (2026-07-29)

§12 shipped capture as `board todo "<text>"` and removal as `d` in the frame, arguing
that in-frame text entry is an insert mode and §2 kept "nothing to learn". The argument
was about the wrong cost. First use, unprompted: *"adding a todo using the command line
is a friction I don't want, I want all in one, not different entries for the same
thing."*

The mode was never the expensive part. **Splitting one action across two surfaces was** —
you are looking at the list, and finishing one is a keystroke while adding one means
leaving the tab you are looking at. That asymmetry is the friction, and no amount of
mode-avoidance buys it back.

So `a` opens a prompt in the frame. What kept §2 intact is the mode's shape, not its
absence: one key in, two keys out, no cursor movement, and the prompt takes the legend's
line rather than adding one, so entering it cannot change the frame's height (the fit
loop would otherwise collapse the quiet tail under the typist — §9.10's mistake with a
new cause). The CLI verbs all stayed; nothing was removed to make room.

**Two decisions this settled, both against the obvious implementation:**

- **No timeout while typing.** §7 returns to ambient after 10s so a stray arrow key
  cannot strand the tab. Applying that to capture would mean a timer that deletes
  half-typed text — the only destructive act in the loop, on a clock. Typing is not
  stray, and ctrl-c is still there (ISIG is left on in `rawMode`).
- **A mode-blind decoder.** Every keystroke carries both the command it names and the
  text it would type; the loop decides which reading applies. The alternative — telling
  the decoder the mode — puts the mode in two places.

**The bug that justified testing a package that had none.** `watch` had no test files, on
the grounds that it is the impure half. The decoder is not: it reads a file, so an
`os.Pipe` is the whole fixture. The first test found that an arrow key typed `[A` into
the prompt — ESC is not printable but `[` and `A` are, so a rune-wise filter passes the
tail of every escape sequence through. Verified by mutation: with the guard disabled the
test reports `"\x1b[A" typed "[A", want ""`.

### 9.19 The bar meant two different clocks (2026-07-29)

First look at the shipped band, reported as "feels off, and not like a separate thing":

> The bar means two different things. For a session, length is idle time — rot. For a
> todo, it's age since you jotted it. Same visual variable, two incompatible concepts, so
> a todo at `0m` reads as "just finished working" when it means "just written down."

Correct, and the sharper version is that the two are different *clocks*, not just
different labels: **a session's idle time is a gap that resets** the moment the session is
touched, **a todo's age is a lifetime that only grows.** Nothing about the encoding said
which one a row was showing, and every other band's `0m` means "active this minute", so
the note borrowed the vocabulary of a running process.

§12 had asserted the opposite on purpose — "a fortnight of not starting reads next to a
fortnight of silence" — which was an instinct for a comparison that is not valid, written
down as a rule. This codebase already had the rule it should have followed, on WORKING
rows: *no bar, for a working agent elapsed time is progress, not rot.* A bar means rot on
the idle scale. A lifetime is not rot either.

Three changes, and the second one was hiding behind the first:

- **No bar, no workspace, `12d ago` instead of `12d00h`.** "ago" names the quantity, and
  the coarse one-unit form is honest — minutes never matter on something written
  yesterday. It fits the 7-column slot exactly, so the column arithmetic did not move.
- **The band moved last, after the whole fleet.** It had been drawn between WORKING and
  QUIET, which put a thing that is not a process state inside a ranking of process
  states. That position was never a reading decision: it was chosen because the fit loop
  trims the tail and clips from the bottom, so the lowest band is the first casualty. An
  implementation detail had been setting the reading order.
- **So the fit loop grew a second absorber**, which is what let position become a
  reading decision: quiet to its floor, then the list to its own, then either to nothing.
  A band collapsed to its count still reports, so shedding rows beats letting `clip` take
  a whole band off the bottom. Below ~20 rows the list does go, which is accepted: that
  tab is too short for it. *If that bites, the cheap fix is one KPI cell, not a taller
  floor.*

What the goldens proved on re-blessing: the four pre-existing frames differ by exactly one
line each — the legend gaining `a new todo` — so nothing about session rendering moved
while the todo band was rebuilt twice.

### 9.20 The legend was keyed by round numbers, not by rungs (2026-07-29)

A design review of the live frame, asked for as feedback rather than as a bug hunt. Two of
the four findings held up, and the one that mattered was in the key to the scale.

**The legend advertised five rungs and painted four.** `scaleLegend` picked round marks —
1h, 6h, 1d, 3d, 7d — and then asked `idleScale` which rung each landed on. The log
boundaries are not round: they fall at ~1h48m, 6h47m, 20h42m and 2d12h. So 1d was already
past the third boundary, and the five marks mapped to rungs **0, 1, 3, 4, 4**. `#cde2fb`
was drawn twice and `#6da7ec` was drawn nowhere — while three rows on screen were painted
with it. A colour in use with no entry in its own key is the exact failure §6 forbids when
it says a value scale without a key is decoration.

The bug survived because `TestScaleLegendCoversTheRamp` asserted the *labels* were present,
never the rungs — a test whose name claimed the coverage it did not check. It now counts
painted swatches per ramp entry, so the marks cannot silently re-point again if the scale
or the ramp moves. Marks are 1h, 3h, 12h, 2d, 7d: still round, one inside each rung.

**Labels were briefly calmed, and that was reverted.** A tab named `DLP/EMAIL MGMNT
REVIEW` sat beside `Claude design capabilities for CLI tools` in the same band, and
uppercase reads as loud, so the argument was that whoever named the tab outranked the rank
ordering. A `calm` helper title-cased any label with no lowercase in it, applied below the
user's own label and never to it, on the §9.9 reasoning that everything under that level
is inherited rather than chosen.

Rejected on sight of the result — `TASKS` had become `Tasks`:

> if someone labels all caps, there might be a reason for that

Which is the stronger position, and it generalises past case: **an inherited name is still
somebody's name.** §9.9 says users rarely label in `~/.board.json`, and the conclusion
drawn from that was that the fallbacks are therefore not deliberate — but a cmux tab title
*is* typed by hand, so the precedence's lower levels are inherited by board and authored by
the user all the same. The distinction `calm` relied on does not exist.

The reporting boundary is the real rule here, and it is the same one that makes §8 safe:
board states what is, and restyling a name is editing the fleet's data to suit the frame.
If shouting labels ever do break the hierarchy, the fix belongs in what board controls —
weight, colour, band — not in the string. `TestLabelsKeepTheirTone` pins the pass-through
at both inherited levels so this cannot come back by accident.

**Two findings did not survive their own record, and one was simply wrong.**

- *Group the quiet band by workspace* — §10.5 rejected this already, with the better
  argument: one workspace held 7 unrelated sessions, so grouping scatters the two rows
  that need you across five boxes. The live frame agrees; `INTERGARTION CAP` holds four.
- *The `⚠` is a third encoding of idle, on 83% of rows* — **wrong on the facts.** The bar's
  ramp is absolute (0→7d); `Stale` is relative to the user's configured threshold. The `⚠`
  is the only threshold-relative mark on a row, so it is not a duplicate of anything. What
  is true is narrower and unresolved: magnitude and threshold are shown as two separate
  marks, and at a 4h threshold the second one fires on almost every quiet row. Recorded
  here rather than acted on, because the fix worth having puts the threshold *into* the
  bar, and that changes an encoding §6 pins.

### 9.21 The quiet band folds, and the fold is the reader's (2026-07-30)

QUIET is the largest block on a real frame — 13 of ~20 content rows — and the least
actionable: nothing in it is happening. Asked for as "collapse it", specified as **open by
default, with one key to toggle**, which is the right way round. The band's whole value is
being seen without being asked for; a default-folded backlog is one nobody reads again.

`z`, and `ז` at the same physical position (§7). Not `c`: this tool has exactly one
dangerous documented action and it is called `close` (§10.6), so a key that reads as
"collapse" to the author and "close" to the finger is not worth the mnemonic. `z` is vim's
fold prefix, which is the prior art for exactly this gesture.

Three things fell out of the existing record rather than being designed:

- **The band keeps its header and its count** — `QUIET · 32 · collapsed`. §9.14 removed a
  `⌄ 3 more` chevron for promising a key that did not exist; the same rule forwards means
  the count must stay and the key must be named, so the legend says `z fold` / `z unfold`
  and names it in both states. `collapsed` is load-bearing: it separates rows the reader
  folded from rows the fit loop trimmed, which are two different facts about the same
  absence.
- **Folded rows leave `DisplayOrder`.** Navigation follows the screen (§7), so a row the
  frame is not drawing cannot be a stop — otherwise the caret sits somewhere invisible with
  Enter pointed at it and the refresh paused for it. Folding also clears a selection inside
  the band, because `Step` over an order that no longer holds the selection returns the
  first row, which would teleport the cursor to a blocked session.
- **The fit loop needed the fold zeroed, not just skipped.** A folded band draws no rows,
  so trimming its tail sheds no lines; the absorber stages would have spun ten times and
  fallen through to `clip`. Setting `keepQuiet = 0` makes both quiet stages inert by their
  own `> floor` test, so the trim order did not have to learn about the fold.

The fold is not persisted to `~/.board.json`. It is state about this sitting at the tab,
and the config file is for what should survive one (§8: it stays the only file written).

What the goldens proved: the five pre-existing frames differ by exactly one line each —
the legend gaining `z fold` — and `frame-empty` did not move at all, because a fleet with
no quiet band is offered no fold key. So nothing about row rendering changed while a band
learned to disappear.

### 9.22 `module board` built for a year and could never be installed (2026-07-30)

A Go module path is a download address, not a name. `module board` resolved every
`board/internal/...` import correctly for as long as the build started *inside* the
directory — Go sees the module declaring itself `board`, and the folder is right there.
`go install <url>@latest` runs the resolution backwards: it starts from the remote, opens
`go.mod`, finds a module calling itself `board`, and has nowhere to look, because `board`
is not a place. Hard stop, not a warning.

**§2's "no dependencies" is what hid it.** A module that never fetches anything never
exercises module resolution, so nothing in a year of green builds and passing tests ever
required the path to be real. The one command that would have caught it is the one command
only a stranger runs. That generalises past this repo: an invariant no local path
exercises is not verified by the suite being green, and this one was load-bearing for
every other publishing step.

The path is `github.com/zalts1/dashy` — the remote's actual URL. The repo keeps `dashy`
and the binary stays `board`; renaming the GitHub repo was considered and declined, so the
mismatch is deliberate and now confined to `go.mod` and the import prefix. `cmd/board/` is
what names the binary, so `go install github.com/zalts1/dashy/cmd/board@latest` still puts
`board` on the PATH.

Verified by what does not move: `gofmt -l` clean, `go vet` clean, all eight packages
passing, and the five golden frames byte-identical. A rename that changed a frame would
have meant it was not a rename. End-to-end `@latest` cannot be verified until the branch is
pushed — the proof available locally is that the module path resolves and `go install
./cmd/board` produces `board`.

### 9.23 The toolchain already knew the version; `-ldflags` was cargo (2026-07-30)

`board version` was planned around the standard recipe: a `var version = "dev"` and
`-ldflags "-X main.version=..."` to stamp it, because a plain `go build` is famously
`(devel)`. Probed before being written, on Go 1.26:

| built by | `debug.ReadBuildInfo().Main.Version` |
|---|---|
| `go build` in the repo | `v0.0.0-20260730194508-eae1cf35ad39+dirty` |
| `go build` in a copy with no `.git` | `(devel)` |
| `go install ...@latest` | the pseudo-version (§9.22 verified the path) |

So the famous `(devel)` is only the no-VCS case, and every way board is actually
installed self-reports with no build flags at all. The recipe would have added a
constant to forget to bump and a flag to forget to pass, in exchange for nothing. **The
tag is the release** (§13). Recorded because an absent `-ldflags` looks like an omission
to the next reader, and the argument against it is not visible in the code.

Two smaller corrections from probing rather than assuming:

- **`cmux --version` exists**, printing `cmux 0.64.16 (96) [5321becb6]`. The plan was to
  read `CFBundleShortVersionString` out of `/Applications/cmux.app`, which would have
  hardcoded an install path and an app-bundle layout to learn the same string.
- **The fixtures passed and the output was still wrong.** cmux names itself in its own
  version string and claude does not, so the first real run printed `cmux   cmux
  0.64.16`. Nothing in a table of made-up version strings suggests that; it took
  building the binary and looking. The fix is pinned by a test now, but the fixtures did
  not find it — §11's manual verification path is doing work the suite cannot.
