package github

import (
	"reflect"
	"testing"
)

// The remote is where owner and repository come from, and git writes it in more spellings than
// one. All of these are the same repository.
func TestParseRemote(t *testing.T) {
	cases := map[string][2]string{
		"git@github.com:zalts1/dashy.git":           {"zalts1", "dashy"},
		"git@github.com:zalts1/dashy":               {"zalts1", "dashy"},
		"https://github.com/zalts1/dashy.git":       {"zalts1", "dashy"},
		"https://github.com/zalts1/dashy":           {"zalts1", "dashy"},
		"ssh://git@github.com/zalts1/dashy.git":     {"zalts1", "dashy"},
		"https://user@github.com/zalts1/dashy.git":  {"zalts1", "dashy"},
		"git@github.com:zalts1/dashy.with.dots.git": {"zalts1", "dashy.with.dots"},
		// Not GitHub, so there is no pull request to ask about. Answered as nothing rather
		// than guessed at: a query built from a GitLab path would 404 on every tick.
		"git@gitlab.com:zalts1/dashy.git":      {"", ""},
		"https://example.com/zalts1/dashy.git": {"", ""},
		"/some/local/path":                     {"", ""},
		"":                                     {"", ""},
	}
	for url, want := range cases {
		owner, name := parseRemote(url)
		if owner != want[0] || name != want[1] {
			t.Errorf("parseRemote(%q) = %q/%q, want %q/%q", url, owner, name, want[0], want[1])
		}
	}
}

// The url comes out of a git config file, which has sections and indentation and other remotes.
// origin is the one board asks about, because it is the one a pull request is opened against.
func TestRemoteFromConfig(t *testing.T) {
	const cfg = `[core]
	repositoryformatversion = 0
[remote "fork"]
	url = git@github.com:someone/dashy.git
	fetch = +refs/heads/*:refs/remotes/fork/*
[remote "origin"]
	url = git@github.com:zalts1/dashy.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
	remote = origin
`
	if got := originURL(cfg); got != "git@github.com:zalts1/dashy.git" {
		t.Errorf("originURL = %q, want origin's and not fork's", got)
	}
	if got := originURL("[core]\n\tbare = false\n"); got != "" {
		t.Errorf("originURL with no origin = %q, want empty", got)
	}
	// A url outside any section belongs to nobody.
	if got := originURL("url = git@github.com:x/y.git\n"); got != "" {
		t.Errorf("originURL of a sectionless url = %q, want empty", got)
	}
}

// HEAD names the branch, and a detached HEAD names none — there is no branch for a pull request
// to be open against, so board asks about nothing rather than about a sha.
func TestParseHead(t *testing.T) {
	cases := map[string]string{
		"ref: refs/heads/main\n":                              "main",
		"ref: refs/heads/a-row-points-at-more-than-its-tab\n": "a-row-points-at-more-than-its-tab",
		"ref: refs/heads/feature/nested/name\n":               "feature/nested/name",
		"4390457c0ffee0000000000000000000000000000\n":         "",
		"ref: refs/tags/v1.0\n":                               "",
		"":                                                    "",
	}
	for in, want := range cases {
		if got := parseHead(in); got != want {
			t.Errorf("parseHead(%q) = %q, want %q", in, got, want)
		}
	}
}

// The answer board wants out of the GraphQL reply is one URL. Everything else in it is context
// for `doctor`.
func TestParseAnswer(t *testing.T) {
	const reply = `{"data":{"repository":{"pullRequests":{"nodes":[
	  {"number":20,"url":"https://github.com/zalts1/dashy/pull/20","state":"OPEN"}
	]}}}}`
	got, ok := parseAnswer([]byte(reply))
	if !ok {
		t.Fatalf("parseAnswer found nothing in a reply with a PR in it")
	}
	want := PR{Number: 20, URL: "https://github.com/zalts1/dashy/pull/20", State: "OPEN"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseAnswer = %+v, want %+v", got, want)
	}

	// No open PR for the branch is the ordinary answer, and it is not an error: most branches
	// do not have one, and a row simply carries no glyph.
	for _, empty := range []string{
		`{"data":{"repository":{"pullRequests":{"nodes":[]}}}}`,
		`{"data":{"repository":null}}`,
		`{"data":null}`,
		`{"errors":[{"message":"Could not resolve to a Repository"}]}`,
		`not json at all`,
		``,
	} {
		if _, ok := parseAnswer([]byte(empty)); ok {
			t.Errorf("parseAnswer(%q) claimed to find a PR", empty)
		}
	}
}

// The query asks for one open PR on one branch, and names nothing else: a smaller reply is a
// smaller thing to get wrong, and the review state was deliberately left out (DESIGN.md §10.12).
func TestQueryShape(t *testing.T) {
	q := query()
	for _, want := range []string{"pullRequests", "headRefName", "states: [OPEN]", "first: 1",
		"number", "url", "state"} {
		if !contains(q, want) {
			t.Errorf("query does not ask for %q:\n%s", want, q)
		}
	}
	for _, unwanted := range []string{"latestReviews", "reviewDecision", "commits"} {
		if contains(q, unwanted) {
			t.Errorf("query asks for %q, which §10.12 left out", unwanted)
		}
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Available is the only gate before a network call, so what it does *not* promise is worth
// pinning: gh being on PATH says nothing about being logged in, online, or able to see the
// repository. All three fail inside ask() and all three mean the same thing — no glyph.
func TestAvailableOnlyChecksPath(t *testing.T) {
	// Not asserting the value: the machine running this may or may not have gh, and either is a
	// valid state. Asserting that asking is free of side effects and terminates, which is what a
	// gate on a per-tick path has to be.
	_ = Available()
	// A target board cannot form a question about never reaches the network.
	if got := Read(nil); len(got) != 0 {
		t.Errorf("Read(nil) asked about something: %+v", got)
	}
}

// The TTL is what decouples the poll interval from GitHub's rate limit, so it has to be a real
// duration gh will accept.
func TestTTLIsAGhDuration(t *testing.T) {
	if got := ttl(); got != "180s" {
		t.Errorf("ttl() = %q, want 180s", got)
	}
}
