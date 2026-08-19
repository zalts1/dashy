package preview

import (
	"reflect"
	"testing"
)

// The shape portless writes. Three routes: one ordinary, one alias (`portless alias`
// registers a static route with no process, so pid 0), and one whose pid is gone.
const routesJSON = `[
  {"hostname": "api.localhost", "port": 4091, "pid": 501},
  {"hostname": "docs.localhost", "port": 4092, "pid": 0},
  {"hostname": "web.localhost", "port": 4093, "pid": 502},
  {"hostname": "", "port": 4094, "pid": 503}
]`

func TestParseRoutes(t *testing.T) {
	got := parseRoutes([]byte(routesJSON))
	want := []route{
		{Hostname: "api.localhost", Pid: 501},
		{Hostname: "docs.localhost", Pid: 0},
		{Hostname: "web.localhost", Pid: 502},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseRoutes = %+v, want %+v", got, want)
	}
	// Malformed input is silently empty: a preview link is the least load-bearing thing
	// on the screen, and board reports rather than crashing.
	if n := len(parseRoutes([]byte("not json"))); n != 0 {
		t.Errorf("garbage parsed into %d routes", n)
	}
}

// The URL has to be the one portless itself prints, or clicking it reaches nothing.
// This mirrors portless's own formatUrl: the scheme comes from the tls marker and the
// port is omitted when it is that scheme's default.
func TestFormatURL(t *testing.T) {
	cases := []struct {
		host string
		port int
		tls  bool
		want string
	}{
		{"api.localhost", 443, true, "https://api.localhost"},
		{"api.localhost", 80, false, "http://api.localhost"},
		{"api.localhost", 8443, true, "https://api.localhost:8443"},
		{"api.localhost", 3000, false, "http://api.localhost:3000"},
		// A tls proxy on port 80 is not the https default, so the port stays.
		{"api.localhost", 80, true, "https://api.localhost:80"},
	}
	for _, c := range cases {
		if got := formatURL(c.host, c.port, c.tls); got != c.want {
			t.Errorf("formatURL(%q, %d, %v) = %q, want %q", c.host, c.port, c.tls, got, c.want)
		}
	}
}

// lsof's -F output is field-per-line with the pid repeated as a header for each process.
// A process that has exited is simply absent, which is the ordinary case: routes.json
// outlives the dev server it describes, exactly as a maki report outlives its process
// (§17).
func TestParseCwds(t *testing.T) {
	const out = "p501\nfcwd\nn/Users/you/work/repo\np502\nfcwd\nn/Users/you/work/repo/.claude/worktrees/feature\np504\nfcwd\n"
	got := parseCwds([]byte(out))
	want := map[int]string{
		501: "/Users/you/work/repo",
		502: "/Users/you/work/repo/.claude/worktrees/feature",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseCwds = %+v, want %+v", got, want)
	}
	if n := len(parseCwds(nil)); n != 0 {
		t.Errorf("empty output parsed into %d entries", n)
	}
	// A name with no pid header before it belongs to nobody and must not be attributed
	// to the last process seen in some earlier read.
	if n := len(parseCwds([]byte("n/Users/you/orphan\n"))); n != 0 {
		t.Errorf("orphan name parsed into %d entries", n)
	}
}

// The two halves of a read, and what separates them. routes.json outlives the dev servers
// it describes — nothing in portless deletes an entry when a process exits — so "the file
// names three routes and board can place none of them" is a real state and a different one
// from "the file is empty". doctor states which (§14, §18).
func TestRosterHalves(t *testing.T) {
	routes := parseRoutes([]byte(routesJSON))
	if got := (Roster{Listed: len(routes), Routes: join(routes, nil, 443, true)}); got.Listed != 3 || len(got.Routes) != 0 {
		t.Errorf("all pids gone: %+v, want 3 listed and none placed", got)
	}
}

// join is the whole of what this package answers: a URL and the directory it is served
// from. A route board cannot place is dropped rather than shown, because a preview link
// on the wrong row is worse than no link (§18).
func TestJoin(t *testing.T) {
	routes := parseRoutes([]byte(routesJSON))
	cwds := map[int]string{501: "/Users/you/work/repo", 502: ""}
	got := join(routes, cwds, 443, true)
	want := []Route{{URL: "https://api.localhost", Dir: "/Users/you/work/repo"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("join = %+v, want %+v", got, want)
	}
}
