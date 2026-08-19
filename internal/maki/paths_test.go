package maki

import (
	"os"
	"path/filepath"
	"testing"
)

// InitPath has to name the file maki will actually run, not a file board would prefer.
// maki searches ~/.maki first and only falls back to the XDG directory, and it stops at
// the first init.lua it finds — a hook written to the other one would never fire, and
// board would report it as installed.

func TestInitPathIsTheXdgOneOnAFreshMachine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	want := filepath.Join(home, ".config", "maki", "init.lua")
	if got := InitPath(); got != want {
		t.Errorf("InitPath = %q, want %q", got, want)
	}
}

func TestInitPathHonoursXdgConfigHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "elsewhere"))
	want := filepath.Join(home, "elsewhere", "maki", "init.lua")
	if got := InitPath(); got != want {
		t.Errorf("InitPath = %q, want %q", got, want)
	}
}

// The legacy directory only counts when it is there, which is maki's own rule: ~/.maki
// takes over config, state and cache the moment it exists.
func TestInitPathPrefersAnExistingLegacyInitFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	legacy := filepath.Join(home, ".maki")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "init.lua"), []byte("-- mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := InitPath(), filepath.Join(legacy, "init.lua"); got != want {
		t.Errorf("InitPath = %q, want the legacy file maki would load: %q", got, want)
	}
}

// An empty ~/.maki with the init.lua in the XDG directory is the shape a half-finished
// `maki migrate xdg` leaves behind, and maki loads the file that exists.
func TestInitPathSkipsALegacyDirectoryWithNoInitFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	if err := os.MkdirAll(filepath.Join(home, ".maki"), 0o755); err != nil {
		t.Fatal(err)
	}
	xdg := filepath.Join(home, ".config", "maki")
	if err := os.MkdirAll(xdg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "init.lua"), []byte("-- mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := InitPath(), filepath.Join(xdg, "init.lua"); got != want {
		t.Errorf("InitPath = %q, want %q", got, want)
	}
}
