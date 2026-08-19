package main

import (
	"strings"
	"testing"
)

// People type -h reflexively, and until this existed board answered it by adding a todo
// called "-h" or printing usage to stderr with exit 2 — a published tool that looks
// broken the first time it is asked what it does.
func TestHelpIsRecognisedWithoutSwallowingRealArguments(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
		why  string
	}{
		{nil, false, "no arguments is the board, not the usage text"},
		{[]string{"-h"}, true, "the reflex"},
		{[]string{"--help"}, true, "the long form"},
		{[]string{"help"}, true, "the word, for people who do not know the flag"},
		// The trap this closes: `todo` joins its arguments into text, so a -h anywhere
		// used to become a todo item rather than a question.
		{[]string{"todo", "-h"}, true, "a flag after a subcommand is still a question"},
		{[]string{"label", "--help"}, true, "same for label"},
		{[]string{"todo", "help sam with the migration"}, false, "a todo may be about help"},
		{[]string{"todo", "help", "sam"}, false, "and may begin with the word"},
		{[]string{"watch"}, false, "an ordinary subcommand"},
		{[]string{"jump", "app"}, false, "an ordinary subcommand with an argument"},
	} {
		if got := helpRequested(c.args); got != c.want {
			t.Errorf("helpRequested(%q) = %v, want %v — %s", c.args, got, c.want, c.why)
		}
	}
}

// The usage text is the only answer -h gives, so a command missing from it is a command
// nobody finds. Listed by hand: this is the check on the dispatch in main, not a
// restatement of it.
func TestUsageNamesEveryCommand(t *testing.T) {
	for _, cmd := range []string{"watch", "jump", "label", "todo", "editor",
		"install-hooks", "uninstall-hooks", "version", "doctor"} {
		if !strings.Contains(usage, cmd) {
			t.Errorf("usage does not mention %q", cmd)
		}
	}
}
