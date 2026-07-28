# CLAUDE.md

`board` is a Go CLI (module `board`, stdlib only) reporting on every live Claude Code
session under cmux. `README.md` is the user contract. **`DESIGN.md` is settled** —
read the section covering what you are changing before changing it, and append there
when a change settles a question or falsifies a recorded conclusion.

## Commands

```sh
go build -o board ./cmd/board    # binary is gitignored; build in place
go vet ./... && go test ./...
go test ./internal/view -run TestGolden        # frame is pinned byte-for-byte
COLUMNS=118 LINES=44 ./board watch | cat       # non-TTY: one frame, then exit
```

That last form is the manual verification path — layout and colour diffed without a
terminal to drive.

## Structure

    cmd/board/       dispatch and printing only
    internal/
      host/          file paths, child processes (cmux env always stripped)
      config/        ~/.board.json — the only file written
      claude/        roster + state: `claude agents --json`, jobs, transcript mtime
      cmux/          tab titles, hook clock, focus — enrichment, never the roster
      board/         Fleet/Row; Build = pure join, Collect = impure gather
      view/          PURE: Frame, header, layout arithmetic, Table, palette, scale
      watch/         IMPURE: alternate screen, termios, signals, ticker
      hooks/         notify, install-hooks

One job per package, one concern per file; a file past ~250 lines is a signal to split
(`view/frame.go` at 197 is the current ceiling). In `view`, rendering lives in
`frame.go`/`header.go` and the fit and column arithmetic in `layout.go` — that seam is
where every fit bug has been. A new command or band needs a `DESIGN.md` entry first.
Add derived quantities to `Fleet`, not to a renderer.

**A band earns its lines by exception** (`DESIGN.md` §9.13). A row reporting the system
working as intended hides one that isn't; that is what removed `ASKED` and with it the
whole audit-log read.

**Keep the two boundaries:** all join logic stays in pure `board.Build`; `view` never
reads the world and `watch` never formats.

## TDD is mandatory

Every behaviour change starts with a failing test — including "obvious" one-liners.
Write it, run it, confirm it fails for the expected reason, then write the minimum
code to pass.

This is not ceremony: three successive blocked-detection rules all looked correct in
the code and all were wrong against real log data (`DESIGN.md` §9.1). **Derived state
must be pinned by a test over a fixture, not by reading the function.** Never write a
test that depends on the live fleet — capture data into a fixture. `DESIGN.md` §11
lists the pure seams and their fixture shapes.

Golden frames re-bless deliberately, with a reason: `BLESS=1 go test ./internal/view`.

## Invariants

- **board never ends a session.** No close, kill, or hibernate on a timer, threshold,
  or batch path. It is a reporting surface; that is what makes it safe to install.
- **`notify` must never fail the agent** — every error path is a silent success.
- **`install-hooks` refuses unparseable `~/.claude/settings.json`** and backs up
  first; idempotent via the `<binary> notify` marker.
- **No dependencies.** stdlib + `syscall` (`TIOCGWINSZ`, termios) over any TUI or
  colour library.
- **No listener, port, or daemon.** The watch tab *is* the process.
- **The frame fits the terminal in both directions.** No line may exceed the width and
  no frame may exceed the height: a wrapped line silently adds a screen row, which
  breaks the height measurement and scrolls the header away. Both are pinned by tests
  over a matrix of sizes (`DESIGN.md` §9.10, §9.12), and `clampLine` is the backstop.
- **Colour is validated, never eyeballed.** Do not substitute or add a value without
  re-validating against `#282c34` and `#040404` (`DESIGN.md` §6).
- **No sorting, no filtering, no dismiss.** The bands are the sort.

## Comments

Short and to the point. Explain *why* — decisions, non-obvious logic, traps — never
what the code plainly does.
