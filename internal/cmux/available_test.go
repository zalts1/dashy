package cmux

import (
	"os"
	"testing"
)

// Available used to answer "am I running under cmux at all", satisfied by an inherited
// CMUX_WORKSPACE_ID. Every caller asks a different question — can I ask cmux anything —
// and board always runs inside a cmux session, so the env branch short-circuited to true
// and the PATH check it falls back on could never be reached (EVIDENCE.md §9.26).
//
// The two answers diverge on exactly one machine: env var present, binary gone. There,
// board reported cmux as available, read no tabs, dropped every interactive session for
// having no tab, and said nothing about any of it.
func TestAvailableAsksWhetherCmuxCanBeQueried(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "some-inherited-workspace")
	t.Setenv("PATH", t.TempDir())
	if Available() {
		t.Error("Available() is true with no cmux on PATH; an inherited env var is not a binary")
	}
}

// The positive case is the binary, wherever the env happens to point. A board run from
// outside any session — a plain shell, a cron job — can still query a cmux that exists.
func TestAvailableFindsCmuxOnPathWithNoSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/cmux", []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CMUX_WORKSPACE_ID", "")
	t.Setenv("PATH", dir)
	if !Available() {
		t.Error("Available() is false with cmux on PATH")
	}
}
