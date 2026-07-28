package config

import (
	"testing"
	"time"
)

func TestDefaultsSurviveAnEmptyOrBrokenFile(t *testing.T) {
	// Load never fails, so the zero value has to be usable: board is a reporting
	// surface and must still report when its config is missing or garbage.
	s := &State{}
	if got := s.Threshold(); got != defaultThreshold {
		t.Errorf("threshold = %v, want %v", got, defaultThreshold)
	}
	if got := s.Poll(); got != defaultPoll {
		t.Errorf("poll = %v, want %v", got, defaultPoll)
	}
}

func TestConfiguredValuesWin(t *testing.T) {
	s := &State{}
	s.Config.IdleThresholdMinutes = 90
	s.Config.PollSeconds = 30
	if got := s.Threshold(); got != 90*time.Minute {
		t.Errorf("threshold = %v, want 90m", got)
	}
	if got := s.Poll(); got != 30*time.Second {
		t.Errorf("poll = %v, want 30s", got)
	}
	// Zero and negative are "unset", not "instant": a 0s poll would spin.
	s.Config.PollSeconds = -1
	if got := s.Poll(); got != defaultPoll {
		t.Errorf("negative poll = %v, want the default", got)
	}
}

func TestSetLabel(t *testing.T) {
	s := &State{Labels: map[string]string{}}
	s.SetLabel("S-1", "merge app#1497")
	if s.Labels["S-1"] != "merge app#1497" {
		t.Fatalf("label not stored: %v", s.Labels)
	}
	// Empty text clears rather than storing a blank, so a cleared label falls back to
	// the tab title instead of rendering as an empty row.
	s.SetLabel("S-1", "")
	if _, ok := s.Labels["S-1"]; ok {
		t.Errorf("empty label stored instead of clearing: %v", s.Labels)
	}
}
