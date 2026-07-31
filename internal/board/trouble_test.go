package board

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zalts1/dashy/internal/claude"
	"github.com/zalts1/dashy/internal/cmux"
)

// Trouble is the fact board used to throw away: an unreadable world and an empty one
// rendered identically, so a missing `claude`, a changed roster schema and a genuinely
// quiet fleet all produced the same screen (EVIDENCE.md §9.26).
//
// It is derived here rather than in a renderer for the same reason every count is: two
// renderers must not disagree about what is wrong (§3).

func TestNoTroubleWhenTheWorldIsLegible(t *testing.T) {
	f := Build(snap(interactive()), now)
	if f.Trouble != "" {
		t.Errorf("Trouble = %q on a healthy snapshot, want empty", f.Trouble)
	}
}

// An empty fleet is not trouble. A quiet morning must not read as a broken machine, or
// the signal is worthless the first time it is true.
func TestAnEmptyFleetIsNotTrouble(t *testing.T) {
	f := Build(Snapshot{}, now)
	if f.Trouble != "" {
		t.Errorf("Trouble = %q for an empty-but-readable world, want empty", f.Trouble)
	}
}

// The three roster failures are different repairs — install claude, look at why the
// query failed, find out what shape it answers in now — so they must not collapse into
// one phrase.
func TestTroubleNamesTheKindOfRosterFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"not installed", claude.ErrNotInstalled, "not found"},
		{"query failed", fmt.Errorf("%w: exit status 1", claude.ErrQueryFailed), "unavailable"},
		{"unreadable", fmt.Errorf("%w: invalid character", claude.ErrUnreadable), "unreadable"},
		{"something else entirely", errors.New("who knows"), "unavailable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := Build(Snapshot{RosterErr: c.err}, now)
			if f.Trouble == "" {
				t.Fatalf("a %s roster reported no trouble", c.name)
			}
			if !strings.Contains(f.Trouble, c.want) {
				t.Errorf("Trouble = %q, want it to contain %q", f.Trouble, c.want)
			}
		})
	}
}

// cmux missing is its own failure, and a quieter one: the roster still arrives, but
// Build then drops every interactive session in it for having no tab. Silently, before
// this — the fleet looked tiny instead of unreported.
func TestTroubleNamesAMissingCmux(t *testing.T) {
	f := Build(Snapshot{NoCmux: true}, now)
	if !strings.Contains(f.Trouble, "cmux") {
		t.Errorf("Trouble = %q, want it to name cmux", f.Trouble)
	}
}

// The roster outranks the tabs: with no roster there are no rows at all, while with no
// cmux the background agents still arrive. One line has room for the more fundamental
// fact, and `doctor` is where both are enumerated.
func TestTheRosterOutranksTheTabs(t *testing.T) {
	f := Build(Snapshot{RosterErr: claude.ErrNotInstalled, NoCmux: true}, now)
	if strings.Contains(f.Trouble, "cmux") {
		t.Errorf("Trouble = %q, want the roster failure to win", f.Trouble)
	}
	if !strings.Contains(f.Trouble, "claude") {
		t.Errorf("Trouble = %q, want it to name claude", f.Trouble)
	}
}

// Trouble names the command that explains it. A report that says only "something is
// wrong" leaves the reader to guess where to look — and `doctor` exists precisely so
// that guess is unnecessary (§9.14 forwards: an available route should say so).
func TestTroublePointsAtDoctor(t *testing.T) {
	f := Build(Snapshot{RosterErr: claude.ErrNotInstalled}, now)
	if !strings.Contains(f.Trouble, "doctor") {
		t.Errorf("Trouble = %q, want it to name board doctor", f.Trouble)
	}
}

// Rows survive their trouble. With no cmux a background agent is still a real row with
// a real state, and hiding it would trade one silence for another.
func TestTroubleDoesNotSwallowTheRowsThatDidArrive(t *testing.T) {
	s := Snapshot{
		Agents: []claude.Agent{background()},
		Titles: map[int]cmux.Titles{},
		NoCmux: true,
	}
	f := Build(s, now)
	if len(f.Rows) != 1 {
		t.Fatalf("got %d rows, want the background agent to survive a missing cmux", len(f.Rows))
	}
	if f.Trouble == "" {
		t.Error("rows arrived and the trouble was dropped; both are true at once")
	}
}
