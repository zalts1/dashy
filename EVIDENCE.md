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
