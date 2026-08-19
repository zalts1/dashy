// Package preview reads the local preview URLs a session's work is reachable at.
//
// The mechanism is portless (https://www.npmjs.com/package/portless), which fronts dev
// servers with stable hostnames instead of ports and keeps its live routes in
// ~/.portless/routes.json. board reads that file the way it reads maki's reports: the
// state is somebody else's, and board only joins it.
//
// It is deliberately two reads and not one, for the same reason maki's roster is (§17):
// routes.json says which hostname serves which pid, and nothing in it says where that
// pid is *working*. The directory is what board joins on — a preview belongs to the
// session whose worktree it serves — and only the running process knows it.
//
// portless is optional, like maki. Without it Available reports false, nothing is read
// and nothing complains: board reports on whichever of these are on the machine.
//
// DESIGN.md §18 is why the links exist and why the join is by worktree.
package preview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/zalts1/dashy/internal/host"
)

// ErrUnreadable is a state directory that is there and does not answer in the shape this
// package knows. Kept apart from an absent portless because they are different repairs,
// and because doctor states which one it found (§14).
var ErrUnreadable = fmt.Errorf("portless routes answered in an unknown shape")

// Route is one live preview: the URL it answers on, and the directory the process
// serving it was started in.
type Route struct {
	URL string
	Dir string
}

// Roster is one read of portless's state, in two halves for the reason maki's is (§17):
// Listed is every route the file names, Routes are the ones with a live process behind
// them. The two disagreeing is a diagnosis rather than a fault — routes.json outlives the
// dev servers it describes — and `doctor` is where it is stated.
type Roster struct {
	Listed int
	Routes []Route
}

// route is one entry of routes.json. Only two fields are read: the hostname the proxy
// answers on, and the pid behind it. The target port is portless's own business.
type route struct {
	Hostname string `json:"hostname"`
	Pid      int    `json:"pid"`
}

// StateDir is where portless keeps its routes. PORTLESS_STATE_DIR is portless's own
// override and is honoured, so board looks where portless actually wrote.
func StateDir() string {
	if d := os.Getenv("PORTLESS_STATE_DIR"); d != "" {
		return d
	}
	return host.Home(".portless")
}

// Available reports whether portless has ever run here. The state directory is the tell
// rather than the binary on $PATH: board reads files, never portless itself, and a
// portless installed but never started has nothing to say.
func Available() bool {
	fi, err := os.Stat(StateDir())
	return err == nil && fi.IsDir()
}

// Read reads the live previews. Like the rosters, the error is half the answer: no routes
// and no error is a machine with nothing running, and an error is a machine board could
// not read (§9.26).
func Read() (Roster, error) {
	b, err := os.ReadFile(filepath.Join(StateDir(), "routes.json"))
	if err != nil {
		if os.IsNotExist(err) {
			// portless has run here and nothing is up. Not a fault: the ordinary state.
			return Roster{}, nil
		}
		return Roster{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	routes := parseRoutes(b)
	if len(routes) == 0 {
		if !json.Valid(b) {
			return Roster{}, ErrUnreadable
		}
		return Roster{}, nil
	}
	return Roster{Listed: len(routes),
		Routes: join(routes, cwds(routes), proxyPort(), proxyTLS())}, nil
}

func parseRoutes(b []byte) []route {
	var out []route
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	// A route with no hostname is nothing board can link to.
	kept := make([]route, 0, len(out))
	for _, r := range out {
		if r.Hostname != "" {
			kept = append(kept, r)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// join turns the two reads into the answer. A route board cannot place is dropped rather
// than shown: a preview link on the wrong row is worse than no link, and the two ways a
// route has no directory are both real. `portless alias` registers a static route with no
// process at all (pid 0), and routes.json outlives the dev server it describes — the same
// reason a maki report is not a row until a process backs it (§17).
//
// Sorted by URL so a row with two candidate previews resolves the same way on every tick.
func join(routes []route, dirs map[int]string, port int, tls bool) []Route {
	var out []Route
	for _, r := range routes {
		dir := dirs[r.Pid]
		if r.Pid == 0 || dir == "" {
			continue
		}
		out = append(out, Route{URL: formatURL(r.Hostname, port, tls), Dir: dir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

// formatURL is portless's own formatUrl, reimplemented rather than asked for: `portless
// list` prints these but only as painted human text, with no --json to parse. The rule is
// small and the consequence of it drifting is visible immediately — a link that reaches
// nothing — which is the trade §2 makes everywhere else too.
func formatURL(hostname string, port int, tls bool) string {
	proto, def := "http", 80
	if tls {
		proto, def = "https", 443
	}
	if port == def {
		return proto + "://" + hostname
	}
	return fmt.Sprintf("%s://%s:%d", proto, hostname, port)
}

// proxyPort and proxyTLS are the two markers portless leaves beside its routes. Both
// default to the plain-http reading, because a missing marker means no proxy has claimed
// the port and a wrong scheme is the one error a reader cannot diagnose from the link.
func proxyPort() int {
	b, err := os.ReadFile(filepath.Join(StateDir(), "proxy.port"))
	if err != nil {
		return 80
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return 80
	}
	return n
}

func proxyTLS() bool {
	_, err := os.Stat(filepath.Join(StateDir(), "proxy.tls"))
	return err == nil
}

// cwds asks where each route's process is working. One lsof for every pid at once rather
// than one per route: this runs on every tick, and lsof scoped to the cwd descriptor of
// named pids costs ~50ms however many there are.
func cwds(routes []route) map[int]string {
	var pids []string
	for _, r := range routes {
		if r.Pid > 0 {
			pids = append(pids, strconv.Itoa(r.Pid))
		}
	}
	if len(pids) == 0 {
		return nil
	}
	// A dead pid is not an error here — lsof exits non-zero when any of the pids it was
	// given is gone, which on a machine with a stale route is the normal case — so the
	// output is used whatever the exit status.
	out, _ := host.Output("lsof", "-a", "-d", "cwd", "-p", strings.Join(pids, ","), "-Fpn")
	return parseCwds(out)
}

// parseCwds reads lsof's -F output: one field per line, tagged by its first byte, with
// the pid repeated as a header before each process's fields. A name with no pid ahead of
// it belongs to nobody and is dropped rather than attributed to the last process seen.
func parseCwds(b []byte) map[int]string {
	out := map[int]string{}
	pid := 0
	for _, line := range strings.Split(string(b), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid = 0
			if n, err := strconv.Atoi(line[1:]); err == nil {
				pid = n
			}
		case 'n':
			if pid != 0 {
				out[pid] = line[1:]
				pid = 0
			}
		}
	}
	return out
}
