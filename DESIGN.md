# board v2 — design

Status: proposal, no code. React to this before anything gets built.

v1 (`board`) prints one line per live agent session. v2 adds a visual surface, a
todo layer, and a notifications ledger.

---

## 1. What the data can actually support

Everything below is verified against the live fleet, not assumed.

| Signal | Source | Cost |
|---|---|---|
| session lifecycle (`running`/`idle`/`needsInput`) | `~/.cmuxterm/claude-hook-sessions.json` | free, file read |
| genuinely blocked | newest Feed event is `question`/`permissionRequest`/`exitPlan` | 11ms, 8MB log tail |
| last activity, cwd, pid, transcript path | same sessions file | free |
| session + workspace titles | `cmux top --all --json`, joined on pid | 50ms |
| notification history | `cmux rpc notification.list`, `~/.cmuxterm/workstream.jsonl` | 324ms / free |
| jump to a session | `cmux surface focus` / `cmux workspace select` | — |

Observed fleet shape: **22 sessions, 8 workspaces, 20 quiet past 45m, 1 running,
0 blocked, oldest untouched 6d21h, 7 sessions in a single workspace.**

Two consequences for design:

- **Nothing moves through stages.** A session is blocked, working, or rotting.
  There is no pipeline, so there is no kanban.
- **The distribution is the story.** A tiny head that needs you, a long tail
  rotting. The visual job is making that asymmetry impossible to ignore.

---

## 2. Todo model: derived + intents

Two kinds, deliberately unequal.

### Derived todos — one per live session

Auto-generated, not editable as todos. `label` is the purpose, state and idle come
from process reality. Zero capture cost, cannot go stale, dies when the session
dies. This is the whole list on a normal day.

### Intents — the "optional unlinked todo"

The containment rule that keeps this from becoming a task manager:

> **An intent is a session that hasn't started yet. It is not a task.**

Consequences, all enforced by having no other fields:

- An intent has **text and an age. No status, no priority, no assignee, no due
  date.** It cannot be "in progress" — that state is what a session is for.
- It has exactly **one transition: intent → session.** You spawn the agent
  yourself as you do today, and `board label` binds the intent by text match — the
  intent disappears and a derived todo takes its place.

  ⚠ **Conflict flagged.** My first draft had `board start "<text>"` spawn the agent
  via `cmux workspace create`. That violates two of your non-goals — "spawning,
  orchestrating, or sending input to sessions" and "anything that makes it easier to
  run *more* agents in parallel is working against the goal." Your bottleneck is
  attention, and a one-command agent launcher attacks the wrong side of it. I've
  cut it in favour of the bind-on-label path above. Say so if you want it back.
- **Age is displayed and never hidden.** An intent sitting for two weeks reads as
  a reproach, not a backlog item.
- Intents are **capped and never grouped.** No projects, no nesting, no tags. If
  you want structure, you want Linear, which you already have.

What this buys: you can jot "profile detections hot path" before you have a free
slot, which is the real use case behind wanting unlinked todos. What it refuses:
becoming the place non-session work lives.

**Decided:** intents age visibly and are never auto-deleted. Silent deletion loses
information, and a list of five old intents is useful signal about over-committing.
Auto-expiry is deferred as a possible future feature, not built now.

### Where finished work goes

A session that ends **leaves the todo list entirely** and lands in the ledger
(§4). The todo list is live sessions only. This is what prevents the graveyard —
the list can only be as long as your actual concurrency.

---

## 3. The primary view

One screen, three bands, sorted by urgency. Bar length encodes idle time — one
visual variable for one dimension, so length is comparable everywhere.

```
 22 sessions · 2 need you · 1 working · 19 rotting · oldest 6d21h

 NEEDS YOU ───────────────────────────────────────────────────────────────
  ●  merge app#1497 before branching        DLP PHISHING       ▇▇       3h00m
  ●  which caching direction?               DETECTIONS         ▇          12m

 WORKING ─────────────────────────────────────────────────────────────────
  ◐  build board: cmux fleet status CLI     KILLSWITCH                     0m

 ROTTING ─────────────────────────────────────────────────────────────────
  ○  Review Claude code artifact            REVIEWS      ▇▇▇▇▇▇▇▇▇▇   6d21h
  ○  PT.7                                   INTERGARTION ▇▇▇▇▇▇▇▇▇    5d20h
  ○  Investigate ticket with Linear CLI     CASE SYNC    ▇▇▇▇         2d04h
  ○  Investigate PLA-66 DLP and phishing…   DLP PHISHING ▇▇▇▇         2d01h
  ○  Plan part 8 task explanation           INTERGARTION ▇▇▇          1d22h
                                                          ⌄ 14 more

 QUEUED ──────────────────────────────────────────────────────────────────
  ·  profile detections hot path                              queued 2d ago
```

Design decisions and why:

- **Three bands, not five.** The only question you're asking is "where do I look
  first". Bands are the answer, ordered top to bottom.
- **`ROTTING` collapses after five rows.** Twenty rotting sessions is true but
  unreadable. The count stays visible so the number can't hide.
- **Bars, not just numbers.** `6d21h` next to `2h27m` doesn't register as 70×.
  Log-scaled bar length does.
- **Workspace is a column, not a grouping.** Workspaces are storage — one of yours
  holds 7 unrelated sessions. Grouping by workspace would scatter the two things
  that need you across five boxes. (Grouping stays available as a toggle, not the
  default.)
- **No timestamps, no PIDs, no CPU, no token counts.** None of it changes what you
  do next.

### Rejected: workspace-grouped layout

```
 INTERGARTION CAP  ●○○○○○○   DLP PHISHING  ●○○○   REVIEWS  ○○○○
```
Dense and pretty, but it answers "how is my fleet arranged" — a question you never
have. Keep as a secondary view at most.

---

## 4. Notifications hub

**cmux already ships most of one:** Feed sidebar on `Ctrl-4`, `cmux feed tui`,
`notification.list`, `jump_to_unread`, `feed.jump`, and inline Allow/Deny in native
macOS notifications. Rebuilding that competes with something already wired into the
terminal you live in, and loses.

The two gaps it doesn't fill:

1. **Off-machine.** v1 has the mechanism (`notify_cmd`, a shell sink fed JSON on
   stdin) but **no sink is configured and push is parked** — see §10. Whether
   off-machine delivery is wanted at all is an open question, not a settled gap.
2. **History and accountability.** cmux's Feed is a live queue — it shows what
   needs answering *now*. It does not answer "what asked me something today, and
   what did I ignore." Feed cards expire after 120s, and the agent then waits
   indefinitely with no trace in the queue.

So the hub is a **ledger, not a queue** — unanswered first, aging visible:

```
 LEDGER · today · 3 unanswered
  ✗ 09:21  question    "Which direction should I take?"      DETECTIONS   9h ago
  ✗ 09:23  permission  Bash: git push origin PLA-66          APP          9h ago
  ✓ 11:04  question    "Merge #1497 now?"       → answered   APP
```

Your history has **870 permission requests, 151 questions, 12 plan exits**. The
`✗` rows are the ones that silently expired — that's the number worth surfacing,
and nothing today shows it.

---

## 5. Interaction rules (the retention constraints)

Your stated failure mode is complexity → drop it. So these are hard limits, not
aspirations:

1. **Zero config to first value.** Run it, see your fleet. No setup, no accounts,
   no project creation. Already true of v1.
2. **Three actions, total:** jump to a session, start an intent, dismiss a rotting
   row. Anything else is a feature request to argue about later.
3. **No modes.** No edit vs view, no drag targets, no right-click menus.
4. **Never require the app to be open.** It must stay a thing you invoke and
   close. The moment it needs to be running for notifications to work, it's
   infrastructure, and infrastructure gets muted.
5. **Reading is free, writing is rare.** The only thing you ever type is a label
   or an intent.

---

## 6. Shape: where it renders

Recommendation: **a CLI command that writes a self-contained HTML file and opens it
in a cmux browser pane.**

```
board dash        # writes ~/.board/dash.html, opens it via `cmux open`
```

Why this and not the alternatives:

- **No server, no listener, no daemon** — a static file plus `open`. Satisfies the
  work-laptop constraint from the original handoff.
- **It lives inside cmux.** `cmux open <path>`, `cmux new-pane --type browser`, and
  `cmux markdown open` all exist. The dashboard can sit in a pane beside the
  terminals it describes, which is where it's useful.
- **Real visualization.** Your crucial point #1 is that it has to be genuinely good
  visually. A TUI caps you at box-drawing characters; HTML gives real typography
  and proportional bars for free.
- **Clicking a row can jump to the session** via `cmux surface focus` — the one
  interaction that makes a dashboard better than a printout.

`board` (the one-screen CLI) stays exactly as it is. The dashboard is additive, for
when you want the shape rather than the list.

---

## 7. Load-bearing assumption

The load-bearing assumption is that you actually label sessions. If you don't, the
derived todo list degenerates into cmux's auto-generated tab names — still useful,
but the todo framing is wrong and §2 needs rethinking.

Cheapest way to find out: let v1 run a week. If `~/.board.json` has ten labels in
it, this design holds. If it has one, we design around auto-names instead.

---

## 8. Decisions

**Intent expiry — decided.** Visible aging, no auto-delete. Auto-expiry deferred.

**Ledger window — decided: rolling 7 days, with unanswered items pinned regardless
of age.** Daily is cleaner to read, but it would drop a question that expired three
days ago and is *still unanswered* — exactly the thing the ledger exists to catch.
Age-out applies only to resolved rows. Seven days also makes patterns legible
("I ignore permission requests every Friday afternoon") without becoming an archive.

**Dismissing rotting rows — recommendation: no dismiss action. Offer `close`.**

The case for dismiss is real: 20 rotting rows is permanent noise, and a band you
stop reading is the same failure as a muted notification.

But dismiss is a lie. It hides the row while the session stays alive — right now
your fleet is **30 Claude processes holding 10.5 GB and 23% CPU**. A dismissed row
is a live agent you've promised yourself you'll never look at, still costing
resources and still capable of sitting there for 6d21h.

So the honest actions on a rotting row are:

- **jump to it** — it's rotting because it's waiting for your review
- **close it** — you're never going back; kill the tab, and the row disappears on
  its own because the pid is gone

`close` is self-cleaning: the list shrinks because reality changed, not because we
hid something. cmux's Agent Hibernation (opt-in, off by default) already reclaims
the RAM of idle agents, but it deliberately keeps them restorable — so it does not
shorten this list.

### Hard rule: nothing closes a tab on its own

**board never closes, kills, hibernates, or otherwise ends a session
automatically.** Not on a timer, not past a threshold, not as an opt-in
"cleanup mode", not as a batch action. There is no code path that ends a session
without a per-row action taken by the user at that moment.

Further, per the decision below, the manual `close` action is **not shown by
default**:

```json
{ "config": { "enable_close_action": false } }
```

Default rotting-row behaviour is **jump only**. Someone who never sets that flag
can never lose a session through this tool, and the tool stays purely a reporting
surface — which is what v1 is and what makes it safe to leave installed.

Consequence, stated plainly: with no close and no dismiss, **ROTTING only grows.**
That is accepted. The band collapses after five rows and the count carries the
signal; the tool reports the backlog and the human decides what to do about it.
Growing honestly beats shrinking by hiding.

For context, not as an argument: closing a cmux agent tab is less lossy than it
feels — cmux stores a `resume_binding` with a `checkpoint_id` per surface and can
restore the session with `claude --resume <id>`. Noted only so the option is
understood; the default stays off regardless.

If a week of use shows you want to keep a session but silence it, the right
primitive is **snooze** (hides for N hours, then returns), not dismiss or close.
Deferred until there's evidence it's needed — it adds per-row state, which §5
rule 3 resists.

---

## 9. What I'd still want to know

The load-bearing assumption is that you actually label sessions. Cheapest way to
find out: let v1 run a week and count the labels in `~/.board.json`.
```

---

## 10. Parked: push notifications

The original brief asked for notification fan-out to a pluggable sink. It was
built, wired to Slack, verified end to end, and then **pulled from v1** — Slack
specifically, and possibly the whole push layer.

Current state:

- The hook mechanism stays: `Stop` and `Notification` in `~/.claude/settings.json`
  call `board notify`, which returns immediately while `notify_cmd` is empty. No
  network, no credential read, no sink process.
- The Slack sink is reverted out of the tree, recoverable at commit `1f24c00`.
- `notify_cmd` remains documented in the README as the extension point.

Two things worth remembering if this is revisited:

- **Gate on absence, not on events.** With 22 live sessions an ungated sink posts
  on every turn end of every agent. macOS `HIDIdleTime` gives real keyboard idle
  cheaply, so "only tell me when I'm actually away" is a few lines and is the
  difference between a useful channel and a muted one.
- **The failure mode is muting, not missing.** A push layer that fires too often
  is worse than none, because muting it also mutes the rare message that mattered.
  This is the same argument that made `needsInput` unusable as a `waiting` flag.

Deciding this properly needs the trial-week data in §9: if `blocked` never fires
and finished sessions are what you keep missing, the ledger in §4 is the better
answer than a push channel.

---

## 11. Built: the ambient dashboard (`board watch`)

Shipped. A TUI that redraws in place on an interval, meant to sit in its own cmux
tab. Chosen over an HTML page because it needs **no listener, no daemon, and no
file-watching** — the tab is the process — which keeps the original work-laptop
constraint intact with nothing to flag.

### Form

Per the data-viz method: >7 classes that all carry meaning is **a table, not a
chart**, and the right form for "one thing matters, the rest are context" is
**emphasis** — one accent, everything else recessive. So: a table, three bands
(`NEEDS YOU` / `WORKING` / `QUIET`), with exactly one element allowed to shout.

Two layers, so a longer look yields more without touching anything:

- **Glance layer** — the BLOCKED badge, the KPI strip, and the point where the
  amber ⚠ column stops. That waterline *is* the threshold, read positionally.
- **Detail layer** — labels, bar lengths, exact durations, workspace names, all in
  recessive ink so they don't compete for the first half-second.

### Colour — validated, not chosen

Every value cleared `scripts/validate_palette.js` against both the lightest and
darkest plausible terminal backgrounds (`#282c34`, `#040404`):

| Role | Value | Result |
|---|---|---|
| blocked badge | white on `#d03b3b` | 4.80 text contrast — sits on our own fill, so it is **theme-independent** |
| running | `#0ca30c` | ≥3:1 mark on every candidate background |
| stale ⚠ | `#fab219` | 7.63–11.18 |
| body / dim ink | `#c3c2b7` / `#898781` | 7.81 / 3.90+ |
| idle ramp (5 steps) | `#256abf`→`#cde2fb` | all four ordinal checks PASS in both extremes |

Two findings worth keeping:

- **Bare `#d03b3b` text FAILS at 2.91** on Ghostty's default background — the most
  important element had the worst contrast. Rendering it as a filled badge with
  white text moves the text onto our own fill and sidesteps the user's theme
  entirely. Status colour never carries meaning alone anyway: badge + the word
  `BLOCKED` + band header is the required icon+label pairing.
- **The ramp had to brighten with age, not darken.** On a dark surface the
  sequential anchor flips; the documented dark floor `#184f95` fails the 2:1
  ordinal gate at 1.73. Steps also had to skip every other rung — adjacent
  ramp steps measured ΔL 0.049, under the 0.06 minimum.

### Encoding rules

- Bar length **and** ramp step both encode idle on one shared **absolute log
  scale** (0→7d). Absolute so bars stay comparable between refreshes; log because
  linear would flatten everything under a day into one cell.
- The bar appears on blocked and quiet rows — both are "time you have owed this
  session attention" — and **not** on working rows, where elapsed time is progress,
  not rot.
- A value scale without a key is decoration, so the ramp legend is always drawn.

### Behaviour

- Redraws with cursor-home + per-line erase, written as one buffer. A full-screen
  clear each tick would flash; the anti-pattern is explicit that a refetch must not
  blank the previous render.
- Height-aware: chrome is fixed, the quiet tail absorbs the remainder and collapses
  to `⌄ N more`. Everything fits uncollapsed on a normal-height tab.
- Interval from `poll_seconds` (default 10s) or `board watch 30s`. One cycle costs
  ~60ms, a 0.6% duty cycle at the default.
- No raw-mode keyboard handling, so there are no modes and nothing to learn —
  ctrl-c exits, SIGWINCH redraws.
- Piped or redirected, it prints a single frame instead of looping, which is how
  the layout above was verified.

---

## 12. Correction: blocked detection, and what it means

§11 and an earlier commit recorded that blocked "will stay mostly empty — median
0s, p90 7s, only 6.7% last 10s or longer" and concluded that rot, not live
blocking, was where the value was. **That conclusion was wrong, from a broken
measurement.**

Three successive rules, each falsified by evidence:

1. *Newest event is an actionable card.* Never fires — cmux logs the card and its
   own `toolUse` in the same second, so the `toolUse` always displaces the card.
2. *Treat the actionable `toolUse` as the request, and its `toolResult` as the
   answer.* Also wrong. cmux's Feed bridge emits a `toolResult` when its ~6s
   semaphore expires; Claude then falls back to its own in-terminal picker and
   waits indefinitely, emitting nothing. So the 6s figure was the bridge timing
   out, not a human answering.
3. **Shipped:** scan backwards, skip `toolResult` as inconclusive, and let the
   first `stop` / `userPrompt` / non-actionable `toolUse` prove the agent moved on.

Re-measured against the same log with rule 3:

| | rule 2 (wrong) | rule 3 (shipped) |
|---|---|---|
| median | 0s | 0s |
| p75 | 0s | 99s |
| p90 | 7s | 511s |
| max | 606s | 85567s (~24h) |
| lasts ≥60s | 0.9% | 30.5% |
| lasts ≥5min | 0.5% | 15.2% |

The median stays 0s because `--permission-mode auto` resolves permission requests
without a human — those are correctly invisible. But **321 episodes ran over a
minute and 160 over five, one for nearly a day.** Blocked is a real, frequent,
long-lived state, and `NEEDS YOU` is the most valuable band rather than dead
weight. The v2 priority order in §9 stands corrected accordingly.

Method note: all three rules looked reasonable in the code. What killed the first
two was watching a real session sit unanswered on screen while the log claimed it
had resolved. Derived state needs checking against the thing itself.

---

## 13. Built: the ledger (`ASKED` band)

Shipped inside `board watch` rather than as a separate command, and scoped to
questions and plan exits only — auto-approved permission requests resolve without
a human, so they are volume rather than accountability.

Sizing came from the log, not a guess: **4.0 asks per active day**, 169 over 42
active days. Seven days is ~27 rows, too many to sit permanently on an ambient
screen, so the band lists the **last 24h** (typically 4–6 rows) and its header
carries the aggregates for the whole window. Nothing is silently dropped.

```
  ASKED · 24h · 6   6d: 18 asks · 1 never answered · longest 50m
           · Next task                              never  12:43 TASKS
```

Three defects found by rendering it against real data:

- **The header claimed a 7-day window the data did not cover.** The 8MB tail spans
  whatever it spans, so `coverage()` now reports the oldest event present and the
  header states the real window.
- **The same session was named two different ways** — `integration-redesign` in
  ASKED (cwd basename) versus `TASKS` in the dashboard (workspace title). The
  ledger now resolves the workspace title, falling back to cwd only for sessions
  that no longer exist.
- **Bars were meaningless here.** Waits are minutes against a scale topping out at
  a week, so every row rendered one cell. Dropped; `never` in amber is the only
  thing in the band that earns colour.

The payoff is the `never` rows: asks whose session ended before anyone answered.
Nothing else in the stack surfaces those. One showed up in the first render.

Cost: one shared pass over the log tail now feeds both blocked-detection and the
ledger, so a tick stays ~60ms.

---

## 14. Built: jump to tab, and the interaction that earns its keep

Two ways to get from the board to a session:

    board jump <substring>     from any tab, matches label or workspace
    ↑/↓ then Enter             from inside board watch

`cmux rpc surface.focus {"surface_id": "..."}` does the work, using the surface id
board already stores per session. (The parameter is `surface_id`; `surface` returns
`invalid_params`.)

### Why this is not redundant with Claude Code's Agent View

`claude agents` (shipped May 2026, present in 2.1.220) covers a lot of this ground
and covers it well: 32 sessions where board sees 22, a native `status` of
idle/busy/waiting plus `waitingFor`, and select → peek → reply inline → attach.
Its data is better than board's log-derived state.

But **attach is not the same action as focus.** Agent View attaches to a session's
transcript inside Agent View. It does not bring the cmux tab forward — with its
splits, its scrollback, the layout that was set up for that piece of work. Only
board knows the surface id, so only board can do the second thing. That is the gap
worth filling; rebuilding peek-and-reply on top of it would be the same mistake as
rebuilding cmux's Feed.

### The refresh-versus-cursor problem, solved by prior art

Rows re-sort as idle time grows and sessions move between bands, so a refresh
landing mid-navigation slides the list under the cursor: you press Enter and jump
to the wrong session. Two separate defects, and the established fixes are known:

- **Identity, not position.** Selection is keyed on surface id. This is exactly why
  htop has a dedicated `F` "Follow" key — index-based selection drifts when the
  sort moves a row. htop makes it opt-in and drops it on the first arrow key; there
  is no reason to ship the broken variant, so here it is always on.
- **An explicit, visible pause.** While a selection is live the data stops
  refreshing and the header reads `paused while selecting · esc to resume`. `less
  +F` establishes the pattern: streaming and interacting are two modes and the
  boundary should be stated, not hidden. A 10s no-keypress timeout clears the
  selection so the tab can never get stuck out of ambient mode.

Deliberately absent: sorting and filtering. k9s and htop both have them, but they
list hundreds to thousands of rows; at ~20 the bands already are the sort, and
hiding rows is the same objection that killed dismiss in §8.

### Tested

`frame_test.go` covers the parts that are easy to get quietly wrong: navigation
follows on-screen order (blocked, working, quiet) rather than the data's sort order
(blocked, quiet, working); stepping clamps at both ends; a selection whose session
vanished between ticks does not strand the cursor; the caret and the `paused`
banner appear only when they should; and a selection sitting in the collapsed QUIET
tail is still drawn rather than becoming invisible.

Terminal restore runs through a `sync.Once` from a `defer`, so it also fires on
panic — a process that exits without restoring termios leaves the user's shell with
no echo. `ISIG` is left enabled so ctrl-c still raises a signal and takes the normal
restore path.
