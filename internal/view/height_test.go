package view

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// fleetOf builds a fleet with the given band sizes.
func fleetOf(blocked, working, quiet int) board.Fleet {
	f := board.Fleet{Blocked: blocked, Workspaces: 1}
	add := func(n, rank int, label string) {
		for i := 0; i < n; i++ {
			f.Rows = append(f.Rows, board.Row{Key: fmt.Sprintf("K-%s%d", label, i), Label: label,
				Workspace: "WS", Surface: label + string(rune('a'+i%26)),
				Idle: time.Duration(i) * time.Hour, Rank: rank})
		}
	}
	add(blocked, board.RankBlocked, "blocked")
	add(working, board.RankWorking, "working")
	add(quiet, board.RankQuiet, "quiet")
	return f
}

// A frame taller than the tab makes the terminal scroll, and the first thing written
// is the first thing lost: the BOARD header with the clock and the refresh interval.
// The quiet tail exists to absorb exactly this, so the frame must always fit.
func TestFrameNeverOverflowsTheTerminal(t *testing.T) {
	for _, rows := range []int{10, 14, 20, 24, 30, 40, 44, 52, 60, 80} {
		for _, quiet := range []int{0, 3, 12, 26, 40, 120} {
			f := fleetOf(3, 2, quiet)
			out := Frame(f, Screen{Now: frameNow, Interval: 10 * time.Second,
				Threshold: 4 * time.Hour, Rows: rows, Cols: 118}, UI{})
			if h := height(out); h > rows {
				t.Errorf("rows=%d quiet=%d: frame occupies %d lines, %d too many",
					rows, quiet, h, h-rows)
			}
		}
	}
}

// The reported case: 3 blocked, 2 working, 26 quiet in a 52-row tab. The
// header scrolled away because chrome was estimated at 13 lines and is really 15.
func TestHeaderSurvivesAFullFleet(t *testing.T) {
	f := fleetOf(3, 2, 26)
	out := Frame(f, Screen{Now: frameNow, Interval: 10 * time.Second,
		Threshold: 4 * time.Hour, Rows: 52, Cols: 118}, UI{})
	if h := height(out); h > 52 {
		t.Fatalf("frame occupies %d of 52 lines; the header would scroll off", h)
	}
	if !strings.Contains(strings.SplitN(out, "\n", 3)[1], "BOARD") {
		t.Error("header is not the first written line")
	}
	// The clock and interval are the whole point of the header being there.
	if !strings.Contains(out, "every 10s") {
		t.Error("refresh interval missing from the frame")
	}
}

// Collapsing must stay honest: whatever is cut is counted.
func TestOverflowIsReportedNotSilent(t *testing.T) {
	f := fleetOf(3, 2, 40)
	out := Frame(f, Screen{Now: frameNow, Interval: 10 * time.Second,
		Threshold: 4 * time.Hour, Rows: 30, Cols: 118}, UI{})
	if !collapseCount.MatchString(out) {
		t.Error("rows were dropped without saying how many")
	}
}

// A tab too short for even the floor still must not scroll — the KPI strip is the
// glance layer and has to survive.
func TestVeryShortTabKeepsTheGlanceLayer(t *testing.T) {
	f := fleetOf(3, 2, 26)
	out := Frame(f, Screen{Now: frameNow, Interval: 10 * time.Second,
		Threshold: 4 * time.Hour, Rows: 10, Cols: 118}, UI{})
	if h := height(out); h > 10 {
		t.Fatalf("frame occupies %d of 10 lines", h)
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Error("blocked count lost on a short tab")
	}
}
