package preview

import (
	"reflect"
	"testing"
)

// lsof's listener output, in the -Fpcn form: pid, command, then one name line per
// listening socket. The shape includes what has to be rejected — a listener outside the
// range, and one inside it that is not a Storybook.
const listenOutput = "p501\ncnode\nf17\nn*:6006\n" +
	"p502\ncnode\nf21\nn127.0.0.1:6007\n" +
	"p503\ncnode\nf9\nn*:3000\n" +
	"p504\ncpython3.11\nf11\nn*:6008\n" +
	"p505\ncbun\nf14\nn[::1]:6020\n" +
	"p506\ncnode\nf14\nn*:6021\n"

// Storybook's default is 6006 and it increments when the port is busy, so the range is
// what identifies it. Closed at 6020 rather than open-ended: TensorBoard defaults to 6006
// and increments the same way, and an unbounded range would eventually claim one (§18).
func TestParseListeners(t *testing.T) {
	got := parseListeners([]byte(listenOutput))
	want := []listener{
		{Pid: 501, Port: 6006},
		{Pid: 502, Port: 6007},
		{Pid: 505, Port: 6020},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListeners = %+v, want %+v", got, want)
	}
	if n := len(parseListeners(nil)); n != 0 {
		t.Errorf("empty output parsed into %d listeners", n)
	}
}

// The range is a closed interval, and both ends are part of the contract.
func TestStorybookPortRange(t *testing.T) {
	cases := map[int]bool{
		6005: false, // just below
		6006: true,  // Storybook's default
		6007: true,
		6020: true,  // the last one board will claim
		6021: false, // just above
		3000: false, // an ordinary dev server
		80:   false,
	}
	for port, want := range cases {
		if got := inStorybookRange(port); got != want {
			t.Errorf("inStorybookRange(%d) = %v, want %v", port, got, want)
		}
	}
}

// The command check is the second half of the identification. Storybook runs under a JS
// runtime; TensorBoard, which shares the port range and the increment behaviour, does not.
func TestStorybookCommands(t *testing.T) {
	for cmd, want := range map[string]bool{
		"node": true, "bun": true, "deno": true,
		"node (v22)": true, // lsof truncates and decorates; a prefix is what matters
		"python3.11": false, "python": false, "Python": false,
		"ruby": false, "": false,
	} {
		if got := isJSRuntime(cmd); got != want {
			t.Errorf("isJSRuntime(%q) = %v, want %v", cmd, got, want)
		}
	}
}

// A Storybook is the same shape as a preview — a URL plus the directory serving it — so it
// joins to a row exactly the same way. http, not https: Storybook serves plain http on
// localhost unless it is put behind something, and something is what portless is for.
func TestStorybookRoutes(t *testing.T) {
	dirs := map[int]string{501: "/Users/you/work/repo", 502: ""}
	got := storybooks(parseListeners([]byte(listenOutput)), dirs)
	want := []Route{{URL: "http://localhost:6006", Dir: "/Users/you/work/repo"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("storybooks = %+v, want %+v", got, want)
	}
}
