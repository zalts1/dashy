# CLAUDE.md

`board` is a Go CLI (module `github.com/zalts1/dashy`, stdlib only) reporting on every live
coding-agent session under cmux — Claude Code and maki, in one fleet.

## Where things are written down

Each file below is the single authority for its column. **Nothing here repeats
another file's detail** — this one holds the rules, and the argument for every rule is
one `§` away, in the section where a change to it gets recorded.

| file | holds | authority for |
|---|---|---|
| `README.md` | the user contract | commands, states, config keys |
| `DESIGN.md` | the settled record: §1–§8, §10–§18, with a `§` index at the top | why anything is the shape it is |
| `EVIDENCE.md` | §9.x, append-only, with a `§` index at the top | what was believed, what falsified it |
| `CONTRIBUTING.md` | the outside contributor's process | how a PR is opened, tested, and declined |
| package docs | `go doc ./...` | what each package's job is |

Both documents are section-addressable: read the `§` index, then that one section —
never the whole file. `§` numbers are bare, stable, and cited from code comments, so
`grep -rn '§9' --include='*.go' .` finds the code a finding constrains, and
`grep -n '^#\+ ' DESIGN.md` maps `§` to line.

When a change settles a question or falsifies a recorded conclusion, append to
`EVIDENCE.md` §9 (evidence) or `DESIGN.md` §10 (deferred), in the same style.

## Commands

```sh
go doc ./...                     # the whole architecture, ~1.2k tokens — start here
go build -o board ./cmd/board    # binary is gitignored; build in place
go vet ./... && go test ./...
go test ./internal/view -run TestGolden        # frame is pinned byte-for-byte
COLUMNS=118 LINES=44 ./board watch | cat       # non-TTY: one frame, then exit
./board version                                # board's, claude's, cmux's and maki's
./demo/record.sh                               # re-record docs/board.gif (needs vhs)

git tag -a vX.Y.Z && git push origin vX.Y.Z     # releasing: the tag IS the version
```

There is no version constant to bump and no `-ldflags`: the tag is what
`go install ...@vX.Y.Z` stamps in, so tagging is the whole release (§13, verified at
`v0.1.0` in §9.25). Annotated, because the message is the only place a release says
what it is.

That last form is the manual verification path — layout and colour diffed without a
terminal to drive. Golden frames re-bless deliberately, with a reason:
`BLESS=1 go test ./internal/view`.

## Structure

Routing only; `DESIGN.md` §3 is why the tree is this shape.

    cmd/board/       dispatch and printing only
    internal/
      host/          file paths, worktree roots, child processes (cmux env always stripped)
      config/        ~/.board.json — the only file written
      claude/        roster + state: `claude agents --json`, jobs, transcript mtime
      maki/          roster + state: the reports its plugin writes, plus live pids (§17)
      cmux/          tab titles, hook clock, focus — enrichment, never the roster
      preview/       portless's routes: the local URL serving a worktree (§18)
      board/         Fleet/Row; Build = pure join, Collect = impure gather
      view/          PURE: Frame, header, layout arithmetic, Table, palette, scale
      watch/         IMPURE: alternate screen, termios, signals, ticker
      hooks/         notify, install-hooks, uninstall-hooks — both agents, two mechanisms
      doctor/        what board could and could not read here
      demo/          the fixture fleet the GIF renders — a main, but never a command
    demo/            board.tape + record.sh: the recording, run by hand (§16)
    docs/            board.gif, and nothing else

In `view`, rendering lives in `frame.go`/`header.go` and the fit and column arithmetic
in `layout.go` — that seam is where every fit bug has been.

## TDD is mandatory

Every behaviour change starts with a failing test — including "obvious" one-liners.
Write it, run it, confirm it fails for the expected reason, then write the minimum
code to pass.

This is not ceremony: it is what `EVIDENCE.md` §9.1 records. **Derived state must be
pinned by a test over a fixture, not by reading the function.** Never write a test that
depends on the live fleet — capture data into a fixture. `DESIGN.md` §11 lists the pure
seams and their fixture shapes.

## Rules

Two kinds, and the difference matters more than any single entry.

### Hard — these are why it is safe to leave installed

- **board never ends a session.** No close, kill, or hibernate on a timer, threshold,
  or batch path. It is a reporting surface (§8).
- **`notify` must never fail the agent** — every error path is a silent success (§8).
- **`install-hooks` refuses an unparseable `~/.claude/settings.json`**, backs up first,
  and is idempotent (§8).
- **The maki block is prepended to `init.lua`, never appended**, and unbalanced markers are
  a refusal. A Lua `return` must end its block, so an appended hook is a syntax error in
  the file that starts maki (§8, §17).
- **board never rewrites or deletes a `plugin.toml` it did not write** — that file is the
  permission policy for every plugin in its directory (§8, §17).
- **cmux env vars are always stripped from child processes** (§8).
- **board opens nothing itself.** A row's preview and folder are hyperlinks handed to the
  terminal; there is no `open`, no `code`, no opener of any kind. `cmux surface.focus`
  stays the only thing board does to the world (§8, §18).
- **The frame fits the terminal in both directions.** A wrapped line silently adds a
  screen row, which breaks the height measurement and scrolls the header away (§6,
  `EVIDENCE.md` §9.10 and §9.12).
- **Colour is validated, never eyeballed** — re-validate against both documented
  terminal backgrounds before substituting a value (§6).

### Current defaults — change them when you have a better idea

These are what the tool does today and the reasoning is one `§` away. Read it so you
know what you are trading, then decide. Disagreeing is not relitigating; if you change
one, record why in `EVIDENCE.md` §9.

- No dependencies; stdlib + `syscall` (§2).
- No listener, port, or daemon — the watch tab *is* the process (§2).
- No sorting, no filtering, no dismiss — the bands are the sort (§1, §8).
- The frame never says which agent a row belongs to; `doctor` counts the rosters apart
  (§1, §17).
- The todo cap of 10 is a refusal, not a trim (§12).
- Capture and removal both live in the frame, and the capture mode stays bounded (§12,
  §9.18).
- A band earns its lines by exception (`EVIDENCE.md` §9.13).
- All join logic stays in pure `board.Build`; `view` never reads the world and `watch`
  never formats (§3).
- Derived quantities go on `Fleet`, not in a renderer, so the two renderers cannot
  disagree (§3).
- Links live in the frame and not in the one-shot table: escapes do not belong in a pipe,
  and a preview hostname is derived from a branch name (§18).
- A row points at one preview, joined on the git worktree — never on directory
  containment, which crosses worktrees (§18, `EVIDENCE.md` §9.34).
- The folder link's scheme is fixed at `vscode://`; §10.10 is the trade.
- One job per package, one concern per file; past ~250 lines, consider splitting (§2).

A new command or band is worth a `DESIGN.md` entry — after it works, not before.

## Comments

Short and to the point. Explain *why* — decisions, non-obvious logic, traps — never
what the code plainly does.
