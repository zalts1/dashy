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

// The three glyphs, and why these three.
//
// ⧉ is the conventional "open in a new window", which is what the folder link does — and it
// names the *action* rather than the app, so it does not go stale when the editor behind it
// changes. ⧫ is a solid mark for the live thing: no interior detail to lose at 12px, and a
// diamond against squares, so it is told apart by shape before colour is read at all. ⧆ is a
// boxed mark for the workbench a Storybook is — deliberately the folder's square again, so
// the two squares read as "somewhere you go to look at this code", with the asterisk and the
// colour separating them.
//
// All three come from one Unicode block — Miscellaneous Mathematical Symbols-B — which is the
// point rather than a coincidence: one block is one font's design family, so the set keeps its
// relative weight wherever it renders. An arrow was tried first for the preview and was too
// light beside ⧉, which is a thing you can only see at size (§9.36).
//
// All are single cell under the East Asian Width table, which is what decides the width
// Ghostty allocates. That is the check that matters, not the font: a glyph board thinks is one
// column and the terminal draws as two wraps the row, and the fit is a hard rule (§6).
const (
	previewGlyph   = "⧫"
	storybookGlyph = "⧆"
	folderGlyph    = "⧉"
)

// actionCell is the row's trailing links, each on its own column so they line up down the
// band whichever of them a row happens to have. Empty when the row has none — an empty cell
// would pad every row of a fleet that has nothing to point at.
//
// Order is the two running things first, then the folder: the folder is on nearly every row,
// so anchoring it at the right-hand end gives the cell a consistent edge, and the two http
// links sit together because that is what they have in common (§18).
//
// Green for the live thing and cyan for the editor, matched in measured contrast so neither
// dominates the other (see linkPreview/linkFolder). Colour is what separates the two glyphs
// at a glance; shape is what separates them when it is not read. Neither is the accent and
// neither is dim: these are affordances rather than data, and exactly one element in the
// frame is allowed to shout (§6). Under cmux the terminal underlines them on hover, which is
// what makes them findable without a legend line promising a click the reader cannot see.
func actionCell(r board.Row, scheme string) string {
	preview, storybook, folder := " ", " ", " "
	if r.Preview != "" {
		preview = link(r.Preview, fg(linkPreview, previewGlyph))
	}
	if r.Storybook != "" {
		storybook = link(r.Storybook, fg(linkStorybook, storybookGlyph))
	}
	if url := editorURL(scheme, r.Folder); url != "" {
		folder = link(url, fg(linkFolder, folderGlyph))
	}
	// Right-trimmed because nothing follows it on the line: a row whose only link is the
	// preview would otherwise end in four columns of padding.
	return strings.TrimRight(preview+" "+storybook+" "+folder, " ")
}
