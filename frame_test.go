package main

import (
	"strings"
	"testing"
	"time"
)

func fixture() fleet {
	return fleet{rows: []row{
		{state: "blocked →", label: "merge app#1497", workspace: "APP", surface: "S-BLK", idle: 3 * time.Hour, rank: 0},
		{state: "done", label: "rotting thing", workspace: "REVIEWS", surface: "S-OLD", idle: 50 * time.Hour, rank: 1, stale: true},
		{state: "done", label: "fresh thing", workspace: "TASKS", surface: "S-NEW", idle: 5 * time.Minute, rank: 1},
		{state: "running", label: "busy thing", workspace: "KILL", surface: "S-RUN", idle: 0, rank: 2},
	}}
}

// Navigation must follow on-screen order (blocked, working, quiet), not the
// data's sort order (blocked, quiet, working).
func TestDisplayOrderMatchesScreen(t *testing.T) {
	got := displayOrder(fixture())
	want := []string{"S-BLK", "S-RUN", "S-OLD", "S-NEW"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("display order: got %v want %v", got, want)
	}
}

func TestStepWrapsAndClamps(t *testing.T) {
	o := displayOrder(fixture())
	if s := step(o, "", +1); s != "S-BLK" {
		t.Errorf("first down: %q", s)
	}
	if s := step(o, "S-BLK", +1); s != "S-RUN" {
		t.Errorf("down across bands: %q", s)
	}
	if s := step(o, "S-BLK", -1); s != "S-BLK" {
		t.Errorf("up at top should clamp: %q", s)
	}
	if s := step(o, "S-NEW", +1); s != "S-NEW" {
		t.Errorf("down at bottom should clamp: %q", s)
	}
	// A session that disappeared between ticks must not strand the cursor.
	if s := step(o, "S-GONE", +1); s != "S-BLK" {
		t.Errorf("vanished selection: %q", s)
	}
}

func TestSelectedRowIsMarkedAndPausedIsShown(t *testing.T) {
	plain := frame(fixture(), time.Now(), 10*time.Second, 4*time.Hour, 44, 130, ui{})
	if strings.Contains(plain, "▸") {
		t.Error("caret drawn with no selection")
	}
	if strings.Contains(plain, "paused") {
		t.Error("paused shown when live")
	}
	sel := frame(fixture(), time.Now(), 10*time.Second, 4*time.Hour, 44, 130, ui{sel: "S-OLD", paused: true})
	if !strings.Contains(sel, "▸") {
		t.Error("no caret for selection")
	}
	if !strings.Contains(sel, "paused") {
		t.Error("paused not surfaced; a stale frame would look live")
	}
}

// A selection in the collapsed tail must still be drawn, or the cursor is invisible.
func TestSelectionInCollapsedTailStaysVisible(t *testing.T) {
	f := fixture()
	for i := 0; i < 40; i++ {
		f.rows = append(f.rows, row{label: "filler", workspace: "W",
			surface: "S-F", idle: time.Duration(40-i) * time.Hour, rank: 1})
	}
	out := frame(f, time.Now(), 10*time.Second, 4*time.Hour, 20, 130, ui{sel: "S-NEW", paused: true})
	if !strings.Contains(out, "fresh thing") {
		t.Error("selected row was collapsed out of view")
	}
}

// A failed focus must be reported inside the frame: when the jump happens the
// board tab is typically no longer the visible one, so stderr goes unseen.
func TestNoticeRendersInHeader(t *testing.T) {
	out := frame(fixture(), time.Now(), 10*time.Second, 4*time.Hour, 44, 130,
		ui{notice: "cmux focus refused"})
	if !strings.Contains(out, "cmux focus refused") {
		t.Error("notice not rendered in header")
	}
}
