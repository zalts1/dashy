package maki

import (
	"os"
	"path/filepath"

	"github.com/zalts1/dashy/internal/host"
)

// Where maki looks for the one Lua file board has anything to say to. This is somebody
// else's precedence and it is mirrored rather than chosen: maki loads the first init.lua
// it finds and stops, so a hook written to the other candidate would never fire while
// board reported it as installed.

// configDirs is maki's global config search order: the legacy ~/.maki when that
// directory exists — it takes over config, state and cache the moment it does — and then
// the XDG config directory.
func configDirs() []string {
	var dirs []string
	if legacy := host.Home(".maki"); isDir(legacy) {
		dirs = append(dirs, legacy)
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = host.Home(".config")
	}
	if x := filepath.Join(xdg, "maki"); len(dirs) == 0 || dirs[0] != x {
		dirs = append(dirs, x)
	}
	return dirs
}

// InitPath is the init.lua maki will run: the first candidate that exists, or the first
// candidate when none does — which is where board creates one.
func InitPath() string {
	dirs := configDirs()
	for _, d := range dirs {
		p := filepath.Join(d, "init.lua")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return filepath.Join(dirs[0], "init.lua")
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
