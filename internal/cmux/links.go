package cmux

import (
	"strings"

	"github.com/zalts1/dashy/internal/host"
)

// Where cmux sends a link board hands it, which board does not get to decide.
//
// board writes an OSC 8 hyperlink and cmux routes it: an http URL goes to a browser tab inside
// cmux by default, and anything else out through the OS. Whether the http half stays inside is a
// cmux preference, `browserOpenTerminalLinksInCmuxBrowser`, and it is cmux's to own — board
// reports it and never writes it, the same line §8 draws around a `plugin.toml` board did not
// create.
//
// `defaults` rather than parsing the plist, because a macOS preferences file is a binary plist and
// this runs in `doctor` only, never on a tick.
const (
	linksInBrowserKey = "browserOpenTerminalLinksInCmuxBrowser"
	bundleID          = "com.cmuxterm.app"
)

// OpensLinksInternally reports whether a ⌘-click will land in a cmux browser tab rather than in
// the system browser. True is also the answer when the key has never been set, because that is
// cmux's own default — an absent preference is a preference.
func OpensLinksInternally() bool {
	b, err := host.Output("defaults", "read", bundleID, linksInBrowserKey)
	if err != nil {
		// The key has never been set, or there is no cmux here. Either way the answer is cmux's
		// default, which is to keep the link inside.
		return true
	}
	return parseBoolDefault(string(b), true)
}

// parseBoolDefault reads what `defaults read` prints for a boolean, which is `0`/`1` — and falls
// back to def for anything else, including the empty output of a key that does not exist.
func parseBoolDefault(s string, def bool) bool {
	switch strings.TrimSpace(s) {
	case "0", "false", "no":
		return false
	case "1", "true", "yes":
		return true
	}
	return def
}
