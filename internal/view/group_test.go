package view

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/zalts1/dashy/internal/board"
)

// §6 says colour is validated and never eyeballed — but a group's colour is the user's, chosen
// in cmux, and board cannot validate a value it does not pick. What it can validate is the
// *function*: every colour board draws is this function's output, so holding the function to the
// floor holds every group to it. That is the whole argument for lifting rather than passing
// through (§18).
func TestGroupColourClearsTheFloorForAnyInput(t *testing.T) {
	// The real values off a live cmux, and the darkest of them — #880E4F — measures 1.48 raw:
	// a third of the bare red §9.4 already rejected. Passing cmux's colours straight through
	// would have drawn a group name nobody could read.
	cases := []string{"#7D6608", "#006B6B", "#880E4F", "#000000", "#010203", "#FFFFFF", "#4C8DFF"}
	for h := 0; h < 360; h += 15 {
		cases = append(cases, hsl(float64(h)/360, 0.12, 1.0)) // very dark, fully saturated
	}
	for _, in := range cases {
		out := groupColour(in)
		for _, bg := range []string{bgLightest, bgDarkest} {
			if r := contrast(out, bg); r < inkFloor-epsilon {
				t.Errorf("groupColour(%s) = %s measures %.2f against %s, below the ink floor %.2f",
					in, out, r, bg, inkFloor)
			}
		}
	}
}

// Lifting must keep the colour recognisable as the one the user chose, or the rail stops being
// a way to tell one workspace from another. Lightness moves; hue does not.
func TestGroupColourKeepsItsHue(t *testing.T) {
	for _, in := range []string{"#7D6608", "#006B6B", "#880E4F"} {
		got, want := hue(groupColour(in)), hue(in)
		if d := math.Abs(got - want); d > 2 && d < 358 {
			t.Errorf("groupColour(%s) moved the hue from %.0f° to %.0f°", in, want, got)
		}
	}
	// A workspace with no colour is not a colour: it must not become black lifted to grey.
	if got := groupColour(""); got != "" {
		t.Errorf("groupColour(\"\") = %q, want empty", got)
	}
}

// grouped is a fleet with one workspace holding two sessions and one holding a single session —
// the two mental models on one screen, which is the whole point (§18).
func grouped() board.Fleet {
	return board.Fleet{Workspaces: 2, Rows: []board.Row{
		{Key: "K-A", State: "done", Label: "backfill refunds", Repo: "app", Surface: "S-A",
			Group: "Payments rework", GroupColour: "#7D6608", Idle: 5 * time.Minute, Rank: board.RankQuiet},
		{Key: "K-B", State: "done", Label: "webhook retries", Repo: "app", Surface: "S-B",
			Group: "Payments rework", GroupColour: "#7D6608", Idle: 20 * time.Minute, Rank: board.RankQuiet},
		{Key: "K-C", State: "done", Label: "wizard copy", Repo: "date-invite", Surface: "S-C",
			Group: "Wizard copy and UX fixes", GroupColour: "#880E4F", Idle: 40 * time.Minute, Rank: board.RankQuiet},
	}}
}

func TestAGroupWithSeveralSessionsIsNamed(t *testing.T) {
	out := plain(Frame(grouped(), screen(44, 130), UI{}))
	if !strings.Contains(out, "Payments rework") {
		t.Error("a workspace holding two sessions was not named")
	}
	// The solo workspace spends no line: naming it would restate the row's own label, which is
	// what took the workspace out of the frame in the first place (§9.39).
	if strings.Contains(out, "Wizard copy and UX fixes\n") {
		t.Error("a workspace holding one session drew a header; it should spend no line")
	}
}

// The rail is what carries the grouping on every member, including the solo rows that have no
// header — and it lives in a column the frame was already leaving blank, so it costs nothing.
func TestGroupRailIsDrawnAndCostsNoColumns(t *testing.T) {
	with := Frame(grouped(), screen(44, 130), UI{})
	if !strings.Contains(with, railGlyph) {
		t.Error("no group rail drawn on a fleet with coloured workspaces")
	}
	// Same fleet, colours removed: the labels must land in exactly the same columns.
	bare := grouped()
	for i := range bare.Rows {
		bare.Rows[i].GroupColour = ""
	}
	if a, b := labelStart(t, plain(with)), labelStart(t, plain(Frame(bare, screen(44, 130), UI{}))); a != b {
		t.Errorf("the rail moved the label column from %d to %d; it must use the blank lead", b, a)
	}
}

// The selection caret and the rail share the lead without colliding: the caret owns its column
// and the rail owns the one outside it.
func TestRailAndCaretCoexist(t *testing.T) {
	out := Frame(grouped(), screen(44, 130), UI{Sel: "K-A"})
	if !strings.Contains(out, "▸") {
		t.Error("caret lost when a rail is drawn")
	}
	if !strings.Contains(out, railGlyph) {
		t.Error("rail lost when a caret is drawn")
	}
}

// labelColumn is where the first row's label starts, which is what must not move.
func labelStart(t *testing.T, plain string) int {
	t.Helper()
	for _, line := range strings.Split(plain, "\n") {
		// Runes, not bytes: the rail glyph is three bytes wide and one column wide, and this
		// measures columns.
		if i := strings.Index(line, "backfill refunds"); i >= 0 {
			return len([]rune(line[:i]))
		}
	}
	t.Fatal("no row found")
	return -1
}

// hsl builds a hex from hue/lightness/saturation, for sweeping the lift over the whole wheel.
func hsl(h, l, s float64) string {
	f := func(n float64) int {
		k := math.Mod(n+h*12, 12)
		a := s * math.Min(l, 1-l)
		return int(math.Round(255 * (l - a*math.Max(-1, math.Min(math.Min(k-3, 9-k), 1)))))
	}
	return fmt.Sprintf("#%02X%02X%02X", f(0), f(8), f(4))
}

func hue(hex string) float64 {
	r, g, b := rgb(hex)
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	mx, mn := math.Max(rf, math.Max(gf, bf)), math.Min(rf, math.Min(gf, bf))
	d := mx - mn
	if d == 0 {
		return 0
	}
	var h float64
	switch mx {
	case rf:
		h = math.Mod((gf-bf)/d, 6)
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h
}
