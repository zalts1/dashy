package view

import (
	"testing"
	"time"
)

func TestHumanize(t *testing.T) {
	cases := map[time.Duration]string{
		0:                             "0m",
		30 * time.Second:              "0m",
		59 * time.Minute:              "59m",
		time.Hour:                     "1h00m",
		90 * time.Minute:              "1h30m",
		23*time.Hour + 59*time.Minute: "23h59m",
		24 * time.Hour:                "1d00h",
		50 * time.Hour:                "2d02h",
		52 * 24 * time.Hour:           "52d00h",
	}
	for d, want := range cases {
		if got := humanize(d); got != want {
			t.Errorf("humanize(%v) = %q, want %q", d, got, want)
		}
	}
	// Fixed width per magnitude, so the IDLE column stays aligned.
	if len(humanize(time.Hour)) != len(humanize(50*time.Hour)) {
		t.Error("hour and day forms are different widths")
	}
}

func TestShort(t *testing.T) {
	cases := map[time.Duration]string{
		10 * time.Second:   "10s",
		time.Minute:        "1m",
		45 * time.Minute:   "45m",
		time.Hour:          "1h",
		23 * time.Hour:     "23h",
		24 * time.Hour:     "1d",
		6 * 24 * time.Hour: "6d",
	}
	for d, want := range cases {
		if got := short(d); got != want {
			t.Errorf("short(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestPad(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want string
	}{
		{"abc", 5, "abc  "},
		{"abc", 3, "abc"},
		{"abcdef", 4, "abc…"},
		{"", 3, "   "},
		// Width is counted in runes, not bytes: a multibyte label must not blow the
		// column out by its encoded length.
		{"→ merge", 9, "→ merge  "},
		{"日本語テキスト", 4, "日本語…"},
	}
	for _, c := range cases {
		got := pad(c.in, c.w)
		if got != c.want {
			t.Errorf("pad(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
		}
		if n := len([]rune(got)); n != c.w {
			t.Errorf("pad(%q, %d) is %d runes wide", c.in, c.w, n)
		}
	}
}
