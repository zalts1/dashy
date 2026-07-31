package claude

import (
	"errors"
	"testing"
)

// The shape `claude agents --json` returns: interactive sessions carry status,
// background ones carry state and a short id naming their jobs dir.
const agentsJSON = `[
  {"sessionId":"s1","kind":"interactive","pid":101,"cwd":"/w/a","status":"busy","name":"tab a"},
  {"sessionId":"s2","kind":"interactive","pid":102,"cwd":"/w/b","status":"waiting","waitingFor":"a question"},
  {"sessionId":"s3","kind":"background","id":"bg1","pid":103,"cwd":"/w/c","state":"blocked"},
  {"sessionId":"s4","kind":"background","id":"bg2","pid":104,"cwd":"/w/d","state":"done"}
]`

func TestParseAgents(t *testing.T) {
	got, err := parseAgents([]byte(agentsJSON))
	if err != nil {
		t.Fatalf("well-formed roster did not parse: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("parsed %d agents, want 4", len(got))
	}
	want := []struct {
		blocked, running, background bool
	}{
		{false, true, false},
		{true, false, false}, // interactive: status waiting
		{true, false, true},  // background: state blocked
		{false, false, true},
	}
	for i, w := range want {
		a := got[i]
		if a.Blocked() != w.blocked || a.Running() != w.running || a.IsBackground() != w.background {
			t.Errorf("%s: blocked/running/background = %v/%v/%v, want %v/%v/%v",
				a.SessionID, a.Blocked(), a.Running(), a.IsBackground(),
				w.blocked, w.running, w.background)
		}
	}
}

// Unparseable input yields no roster rather than a partial one — a half-populated board
// is worse than none. But it must also say so: this used to return a bare nil, which is
// the same answer as a quiet fleet, and the caller had no way to tell a broken machine
// from an idle one (EVIDENCE.md §9.26).
func TestParseAgentsSaysWhenItCouldNotRead(t *testing.T) {
	got, err := parseAgents([]byte("not json"))
	if len(got) != 0 {
		t.Errorf("garbage parsed into %d agents", len(got))
	}
	if err == nil {
		t.Fatal("garbage parsed without error; an unreadable roster is indistinguishable from an empty one")
	}
	if !errors.Is(err, ErrUnreadable) {
		t.Errorf("error = %v, want it to wrap ErrUnreadable so the caller can name the kind", err)
	}
}

// An empty roster is a fact, not a failure: a fleet can genuinely have nothing in it,
// and reporting that as trouble would cry wolf on every quiet morning.
func TestParseAgentsAcceptsAnEmptyRoster(t *testing.T) {
	got, err := parseAgents([]byte("[]"))
	if err != nil {
		t.Errorf("an empty roster reported an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("parsed %d agents from an empty list", len(got))
	}
}

// Every session with no jobs dir must return an empty label, not a stray one, or
// interactive rows would be named after a background agent's open question.
func TestJobLabelIsEmptyWithoutAJobsDir(t *testing.T) {
	if l := (Agent{SessionID: "s1"}).JobLabel(); l != "" {
		t.Errorf("JobLabel with no id = %q, want empty", l)
	}
	if l := (Agent{SessionID: "s1", ID: "definitely-not-a-real-job-id"}).JobLabel(); l != "" {
		t.Errorf("JobLabel for a missing dir = %q, want empty", l)
	}
}
