package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sinkAt isolates $HOME, points notify_cmd at cmd, and gives notify an empty stdin.
// Every test here must go through it: notify reads the real ~/.board.json and the real
// os.Stdin otherwise.
func sinkAt(t *testing.T, cmd string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	conf := `{"config":{"notify_cmd":` + quote(cmd) + `}}`
	if err := os.WriteFile(filepath.Join(home, ".board.json"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}

	// notify reads os.Stdin directly, and the test binary's stdin is not a hook payload.
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = devnull
	t.Cleanup(func() { os.Stdin = old; devnull.Close() })
	return home
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

// The control for the timeout test below: a sink that answers must still be handed the
// payload on stdin, which is the whole of board's notification contract (§10.1).
func TestNotifyHandsThePayloadToTheSink(t *testing.T) {
	home := sinkAt(t, "cat > "+filepath.Join("$HOME", "sink.json"))
	Notify()

	got, err := os.ReadFile(filepath.Join(home, "sink.json"))
	if err != nil {
		t.Fatalf("sink was never run: %v", err)
	}
	// .text is the ready-made one-liner the README promises; without it the payload is
	// arriving but useless.
	if !strings.Contains(string(got), `"text"`) {
		t.Errorf("payload carried no text field: %s", got)
	}
}

// notify runs inside the agent's own hook chain, so the agent pays for whatever the sink
// costs. §8 promised never to *fail* the agent and said nothing about never *delaying*
// it: a notify_cmd pointing at a dead host blocked every Stop hook until Claude Code's
// own hook timeout fired (EVIDENCE.md §9.30).
func TestNotifyDoesNotWaitForASinkThatHangs(t *testing.T) {
	// Short so the suite does not pay the real budget to prove the mechanism. The default
	// is asserted separately, because a seam a test can widen is one a change can widen.
	notifyTimeout = 100 * time.Millisecond
	t.Cleanup(func() { notifyTimeout = defaultNotifyTimeout })

	sinkAt(t, "sleep 30")
	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		Notify()
		done <- time.Since(start)
	}()

	select {
	case took := <-done:
		if took > 5*time.Second {
			t.Errorf("notify waited %v for a hanging sink", took)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("notify never returned: a hanging sink blocks the agent's hook chain")
	}
}

// The timeout is only worth having if it is short enough to be invisible to the agent.
// A push that arrives after the fact is not a push, so this is a ceiling, not a target.
func TestTheNotifyBudgetIsSmall(t *testing.T) {
	if defaultNotifyTimeout > 5*time.Second {
		t.Errorf("notify budget is %v; the agent waits that long on every hook", defaultNotifyTimeout)
	}
}
