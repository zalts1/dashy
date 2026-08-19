# Contributing

Pull requests are welcome, and the bar is unusual enough to state before you spend an
evening on one.

board is a small tool with a long written record. Most of what looks like an obvious
improvement has already been argued, and the argument is in the repo — including the
ones that were tried and reverted. Reading the relevant section first is not deference;
it is the difference between a PR that lands and one that reopens a settled question.

## Read these first, in this order

1. **`CLAUDE.md`** — the rule sheet. It is addressed to an agent, but it is the same
   contract for a person: the hard rules, the current defaults, and which file is the
   authority for what. Everything below is process; that file is the rules.
2. **`go doc ./...`** — the whole architecture in about 1.2k tokens. Start here rather
   than in the tree.
3. **`DESIGN.md`** and **`EVIDENCE.md`** — both are section-addressable. Read the `§`
   index at the top, then the one section you need. Do not read either end to end;
   `grep -n '^#\+ ' DESIGN.md` maps `§` to line.

`§` numbers are cited from code comments, so `grep -rn '§9' --include='*.go' .` finds
the code a finding constrains.

## TDD is mandatory, including for one-liners

Every behaviour change starts with a test that fails. Write it, run it, confirm it
fails **for the reason you expect**, then write the minimum code to pass. A PR whose
first commit is the fix will be asked to start over.

This is not ceremony. `EVIDENCE.md` §9.1 is what happens without it, and §9.24 is a
suite that passed without running, where the pass meant nothing.

Two rules follow from it:

- **Never write a test that depends on the live fleet.** Capture a fixture. `DESIGN.md`
  §11 lists the pure seams and the shape of each fixture.
- **Derived state is pinned by a test over a fixture, not by reading the function.**

## Golden frames

`internal/view/testdata` pins the frame byte for byte, colour included.

    go test ./internal/view -run TestGolden

If your change moves a pixel, re-bless deliberately and say why in the PR:

    BLESS=1 go test ./internal/view

A diff in those files with no explanation is the review comment you will get. CI fails
any PR where the suite itself mutated a tracked file.

## Verify it by hand as well

The suite does not have a terminal, and several of the most recent entries in
`EVIDENCE.md` were found by running the binary rather than by the suite:

    go build -o board ./cmd/board
    COLUMNS=118 LINES=44 ./board watch | cat     # one frame, layout and colour, no TTY

## Record what you learned

When a change settles a question or falsifies something the record claims, append to
`EVIDENCE.md` §9 — what was believed, what falsified it — or to `DESIGN.md` §10 if you
are deferring something, with the trigger that would revive it. Same style as the
entries already there. This is part of the change, not paperwork after it.

## What starts at a disadvantage

Two different things, and the difference matters more than any single row below.

**`CLAUDE.md`'s Hard list is off-limits** — those are why the tool is safe to leave
installed, and a PR that crosses one is declined without an argument.

**Everything else is a current default with a recorded reason.** `DESIGN.md` is a record
of decisions, not a rulebook: an argument loses to a working change. So these are not
refusals, they are the trades you would be making — read the section, then decide. What
gets a PR closed is not disagreeing, it is disagreeing without having read why:

| change | where it was decided |
|---|---|
| any dependency; board is stdlib + `syscall` | `DESIGN.md` §2 |
| a daemon, a port, or a listener | §2 |
| sorting, filtering, or dismissing rows | §1, §8 |
| raising or trimming the todo cap of 10 | §12 |
| abstracting over tmux/zellij | §1 (the `internal/cmux` boundary is already the seam) |
| growing the config surface | §2 |
| anything that ends a session | §8 — and this one is on the Hard list |

## Shape

One job per package, one concern per file; past ~250 lines, consider splitting. `view`
is pure and never reads the world; `watch` is impure and never formats; all join logic
lives in `board.Build`. A derived quantity goes on `Fleet`, not into a renderer, so the
two renderers cannot disagree (`DESIGN.md` §3).

## Commits and PRs

The subject line is a claim about the change, in a sentence, lower case — *an unreadable
world is not an empty one*, *uninstall is the inverse of install*. Not a category prefix
and not an imperative. Branch names are the same sentence, hyphenated. The body carries
the argument and the evidence.

One change per PR. `go vet ./... && go test ./...` clean before you open it.

## CI

macOS only, and that is not an oversight: `watch/term.go` uses `TIOCGETA`, which is
BSD-only, so a linux runner cannot compile board at all.

The runner deliberately has **none of `cmux`, `claude` or `maki` on `$PATH`** — it is the
outsider machine, which is what makes every degraded path a fact instead of an argument.
The job asserts their absence, so an image that started shipping any of them fails
loudly rather than quietly stopping being a test bed.

## The demo GIF

`docs/board.gif` is recorded by `demo/record.sh` (needs `brew install vhs`) from
`internal/demo`, which is board's own renderer over a fixture. Re-record it when the
frame changes. It must never be recorded from a real fleet: labels and workspace names
are someone's work.

## Releases

The git tag **is** the version — there is no constant to bump and no `-ldflags`
(`DESIGN.md` §13, verified in `EVIDENCE.md` §9.25). Tagging is a maintainer step.
