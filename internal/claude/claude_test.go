package claude

import "testing"

// The shape `claude agents --json` returns: interactive sessions carry status,
// background ones carry state and a short id naming their jobs dir.
const agentsJSON = `[
  {"sessionId":"s1","kind":"interactive","pid":101,"cwd":"/w/a","status":"busy","name":"tab a"},
  {"sessionId":"s2","kind":"interactive","pid":102,"cwd":"/w/b","status":"waiting","waitingFor":"a question"},
  {"sessionId":"s3","kind":"background","id":"bg1","pid":103,"cwd":"/w/c","state":"blocked"},
  {"sessionId":"s4","kind":"background","id":"bg2","pid":104,"cwd":"/w/d","state":"done"}
]`

func TestParseAgents(t *testing.T) {
	got := parseAgents([]byte(agentsJSON))
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
	// Anything unparseable yields no roster rather than a partial one: an empty board
	// is honest, a half-populated board is not.
	if n := len(parseAgents([]byte("not json"))); n != 0 {
		t.Errorf("garbage parsed into %d agents", n)
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
