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

1. **Off-machine.** Already solved by v1's `notify_cmd`.
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
hid something. It also turns the ROTTING band into a useful weekly cleanup pass
rather than a wall of shame. cmux's Agent Hibernation (opt-in, off by default)
already reclaims the RAM of idle agents, but it deliberately keeps them restorable —
so it does not shorten this list.

If a week of use shows you want to keep a session but silence it, the right
primitive is **snooze** (hides for N hours, then returns), not dismiss. Deferred
until there's evidence it's needed — it adds per-row state, which §5 rule 3 resists.

---

## 9. What I'd still want to know

The load-bearing assumption is that you actually label sessions. Cheapest way to
find out: let v1 run a week and count the labels in `~/.board.json`.
```
