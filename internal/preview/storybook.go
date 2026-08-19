package preview

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/zalts1/dashy/internal/host"
)

// A Storybook is found by its port rather than by asking anything, because there is nothing
// to ask: it registers with no proxy, writes no state file and has no roster command. What
// it does have is a well-known default and a well-known way of moving off it — 6006, then
// 6007, then 6008 — so a bounded range plus the same worktree join every other link uses is
// the whole identification (§18).
const (
	// storybookFirst is Storybook's default port. storybookLast closes the range, and the
	// range is closed deliberately: TensorBoard defaults to 6006 and increments exactly the
	// same way, so an open-ended `6006 and up` would eventually claim one and put a link on
	// a row that does not have a Storybook. Fifteen is more than anyone runs at once.
	storybookFirst = 6006
	storybookLast  = 6020
)

// listener is one listening socket board might care about: the process behind it and the
// port it answers on.
type listener struct {
	Pid  int
	Port int
}

// listeners asks the machine what is listening. One call for the whole machine rather than
// one per row — lsof scoped to listening TCP sockets is ~50ms however many there are, and
// it runs beside the reads that already cost the tick.
func listeners() []listener {
	// -P keeps ports numeric, so 6006 does not come back as some service name from
	// /etc/services. A non-zero exit is ignored for the reason it is elsewhere here: a
	// process exiting mid-scan is ordinary, and a partial answer is still an answer.
	out, _ := host.Output("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-Fpcn")
	return parseListeners(out)
}

// parseListeners reads lsof's -F output, which tags each field by its first byte and repeats
// the process header before each process's sockets. Only listeners that pass both halves of
// the identification survive: the port range, and a JS runtime behind it.
//
// A process can hold several listening sockets, so the command stays set across name lines
// until the next process header.
func parseListeners(b []byte) []listener {
	var out []listener
	pid, cmd := 0, ""
	for _, line := range strings.Split(string(b), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, cmd = 0, ""
			if n, err := strconv.Atoi(line[1:]); err == nil {
				pid = n
			}
		case 'c':
			cmd = line[1:]
		case 'n':
			if pid == 0 || !isJSRuntime(cmd) {
				continue
			}
			if port, ok := portOf(line[1:]); ok && inStorybookRange(port) {
				out = append(out, listener{Pid: pid, Port: port})
			}
		}
	}
	return out
}

// portOf reads the port off an lsof name field. The address half varies — `*:6006`,
// `127.0.0.1:6006`, `[::1]:6006` — and only what follows the last colon is wanted, which is
// also why -P matters: without it this field can carry a service name instead of a number.
func portOf(name string) (int, bool) {
	i := strings.LastIndex(name, ":")
	if i < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(name[i+1:]))
	if err != nil {
		return 0, false
	}
	return n, true
}

func inStorybookRange(port int) bool { return port >= storybookFirst && port <= storybookLast }

// isJSRuntime is the second half of the identification, and it is what keeps the range
// honest: TensorBoard shares both the default port and the increment, and it is Python. A
// prefix match rather than equality because lsof truncates long command names and some
// runtimes decorate theirs.
func isJSRuntime(cmd string) bool {
	for _, rt := range []string{"node", "bun", "deno"} {
		if strings.HasPrefix(cmd, rt) {
			return true
		}
	}
	return false
}

// storybooks turns the surviving listeners into routes, dropping any whose directory board
// could not resolve — the same rule the portless half follows, and for the same reason: a
// link board cannot place is a link on the wrong row.
//
// http and not https: Storybook serves plain http on localhost. Putting it behind TLS is
// what portless is for, and a Storybook run that way arrives through the other half of this
// package with its own hostname.
//
// Sorted by URL, so a row with two candidates resolves the same way on every tick.
func storybooks(ls []listener, dirs map[int]string) []Route {
	var out []Route
	for _, l := range ls {
		dir := dirs[l.Pid]
		if dir == "" {
			continue
		}
		out = append(out, Route{URL: fmt.Sprintf("http://localhost:%d", l.Port), Dir: dir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}
