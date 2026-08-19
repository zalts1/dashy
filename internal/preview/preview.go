// Package preview reads the local URLs a session's work is reachable at, from two sources
// that have nothing in common but the answer.
//
// portless (https://www.npmjs.com/package/portless) fronts dev servers with stable hostnames
// instead of ports and keeps its live routes in ~/.portless/routes.json. board reads that
// file the way it reads maki's reports: the state is somebody else's, and board only joins
// it. Storybook registers with nothing at all, so it is found by its port instead — a
// bounded scan of the range it defaults into (storybook.go).
//
// Both need a second read, for the same reason maki's roster does (§17): neither a route nor
// a listening socket says where its process is *working*, and the directory is what board
// joins on — a URL belongs to the session whose worktree it serves. So the expensive half is
// shared: one cwd lookup over every pid from both sources.
//
// Both sources are optional and neither is a fault when absent. Available reports only on
// portless, because that is the half with a state directory to be missing; a machine with no
// portless can still be running a Storybook, so Read is called either way.
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

// Roster is one read of the machine's local URLs.
//
// The portless half comes in two pieces for the reason maki's roster does (§17): Listed is
// every route the file names, Routes are the ones with a live process behind them, and the
// two disagreeing is a diagnosis rather than a fault — routes.json outlives the dev servers
// it describes. `doctor` is where it is stated.
//
// Storybooks are found a different way, because Storybook registers with nothing and has
// nothing to read: a bounded port scan (storybook.go). Different mechanism, same shape, and
// they join to a row identically.
type Roster struct {
	Listed     int
	Routes     []Route
	Storybooks []Route
	// Listeners is how many sockets in the Storybook range were found at all, placed or not.
	// The same two-halves shape as Listed and Routes, and the same diagnosis: something is
	// listening and no row will carry it means it is not in any session's worktree.
	Listeners int
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

// Read reads the machine's local URLs: portless's routes, and the Storybooks listening on
// its ports. Like the rosters, the error is half the answer — no routes and no error is a
// machine with nothing running, and an error is a machine board could not read (§9.26).
//
// The two sources are read together because they share the expensive part. Neither
// routes.json nor an lsof listener scan says where its process is *working*, and the
// directory is what board joins on, so both need a cwd lookup — and one lsof over every pid
// from both sources costs what one over either would (§9.37).
func Read() (Roster, error) {
	// Asked first and unconditionally: a machine with no portless can still be running a
	// Storybook, and the routes read below is the one that can fail.
	found := listeners()

	b, err := os.ReadFile(filepath.Join(StateDir(), "routes.json"))
	var routes []route
	var readErr error
	switch {
	case err == nil:
		routes = parseRoutes(b)
		if len(routes) == 0 && !json.Valid(b) {
			readErr = ErrUnreadable
		}
	case os.IsNotExist(err):
		// portless has run here and nothing is up. Not a fault: the ordinary state.
	default:
		readErr = fmt.Errorf("%w: %v", ErrUnreadable, err)
	}

	// One lookup for both sources.
	pids := make([]int, 0, len(routes)+len(found))
	for _, r := range routes {
		if r.Pid > 0 {
			pids = append(pids, r.Pid)
		}
	}
	for _, l := range found {
		pids = append(pids, l.Pid)
	}
	dirs := cwds(pids)

	rs := Roster{Storybooks: storybooks(found, dirs), Listeners: len(found)}
	if readErr != nil {
		// The Storybooks still stand: they were found without portless and do not depend on
		// it. A half-readable world is reported as half-readable (§9.26).
		return rs, readErr
	}
	rs.Listed = len(routes)
	rs.Routes = join(routes, dirs, proxyPort(), proxyTLS())
	return rs, nil
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

// cwds asks where each process is working. One lsof for every pid at once rather than one
// per pid: this runs on every tick, and lsof scoped to the cwd descriptor of named pids
// costs ~50ms however many there are.
func cwds(pids []int) map[int]string {
	var args []string
	seen := map[int]bool{}
	for _, p := range pids {
		if p > 0 && !seen[p] {
			seen[p] = true
			args = append(args, strconv.Itoa(p))
		}
	}
	if len(args) == 0 {
		return nil
	}
	// A dead pid is not an error here — lsof exits non-zero when any of the pids it was
	// given is gone, which on a machine with a stale route is the normal case — so the
	// output is used whatever the exit status.
	out, _ := host.Output("lsof", "-a", "-d", "cwd", "-p", strings.Join(args, ","), "-Fpn")
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
