package view

import (
	"strings"

	"github.com/zalts1/dashy/internal/board"
)

// Hyperlinks are how a row's two destinations are reachable without a key, and without
// board learning to open anything. OSC 8 hands the URL to the terminal, and the terminal
// is what opens it — so the one process that must never end a session (§8) gains no new
// write action and spawns no opener. Under cmux an https link opens in a browser tab
// beside the fleet; a vscode:// link goes to the editor through the OS (§18).
//
// The sequence is `ESC ] 8 ; params ; URI ST`, the text, then the same with an empty URI
// to close it. ST is written as `ESC \` rather than BEL: BEL is the older form and is
// still accepted by everything, but it is also a bell, and a terminal that does not know
// OSC 8 would beep once per link per redraw.

const (
	linkOpen  = "\033]8;;"
	linkClose = "\033]8;;\033\\"
	st        = "\033\\"
)

// link makes text clickable, or returns it untouched when there is nothing to point at.
// The untouched path matters: a fleet with no previews and no resolvable folders renders
// exactly the frame it did before links existed.
func link(url, text string) string {
	if url == "" {
		return text
	}
	return linkOpen + url + st + text + linkClose
}

// editorURL is the folder link's destination: `<scheme>://file<abs path>`. A URL rather
// than a command, which is the whole reason the editor half works the same way the preview
// half does — the terminal hands it to the OS, and board runs nothing.
//
// One template covers every editor board supports, because all three read the path as
// whatever follows `<scheme>://file`: VS Code documents it, Cursor is a VS Code fork, and
// Zed's open_listener strips exactly that prefix. Which scheme is `internal/editor`'s
// answer and arrives on Screen; an empty one means no editor was found, and then there is
// no link and no glyph — a glyph that opens nothing is worse than an absent one (§18).
func editorURL(scheme, folder string) string {
	if scheme == "" || folder == "" {
		return ""
	}
	// Path components are escaped, not the separators: a directory with a space in it is
	// ordinary on macOS and an unescaped one truncates the URL at the space.
	var b strings.Builder
	b.WriteString(scheme + "://file")
	for _, part := range strings.Split(folder, "/") {
		if part == "" {
			continue
		}
		b.WriteString("/" + escapePath(part))
	}
	return b.String()
}

// escapePath percent-encodes one path component. net/url's PathEscape is what this would
// be, and is deliberately not used: it escapes `/` too, so it cannot be applied to a whole
// path, and applying it per component means owning the component split anyway.
func escapePath(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~', c == '@', c == '+', c == '&', c == '=':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

// The two glyphs, and why these two. ↗ is the universal "this leaves here" mark, which is
// what a preview is: the work, running, somewhere that is not this terminal. ▤ is a box of
// lines — the files — for the folder. Both stay inside the blocks the rest of the frame
// draws from (Arrows, Geometric Shapes), because a glyph that falls back to another font
// is a glyph whose width board guessed wrong, and the frame's fit is a hard rule (§6).
const (
	previewGlyph = "↗"
	folderGlyph  = "⧉"
)

// actionCell is the row's trailing links: the preview first, then the folder, each on its
// own column so the two line up down the band. Empty when the row has neither — an empty
// cell would pad every row of a fleet that has nothing to point at.
//
// inkSecondary, not the accent and not dim: these are affordances rather than data, and
// exactly one element in the frame is allowed to shout (§6). Under cmux the terminal
// underlines them on hover, which is what makes them findable without a legend line
// promising a click the reader cannot see (§18).
func actionCell(r board.Row, scheme string) string {
	preview, folder := " ", " "
	if r.Preview != "" {
		preview = link(r.Preview, fg(inkSecondary, previewGlyph))
	}
	if url := editorURL(scheme, r.Folder); url != "" {
		folder = link(url, fg(inkSecondary, folderGlyph))
	}
	// Right-trimmed because nothing follows it on the line: a row with a preview and no
	// resolvable folder would otherwise end in two columns of padding.
	return strings.TrimRight(preview+" "+folder, " ")
}
