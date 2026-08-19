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
// changes. ⧆ is a boxed asterisk for the workbench a Storybook is. ⧇ is a boxed circle for the
// live thing, a shape that is neither of the other two rather than a variation on one.
//
// **All three are from one Unicode block, Miscellaneous Mathematical Symbols-B, and that is a
// size guarantee rather than a stylistic preference.** A monospace font draws a block in one
// design, so glyphs from one block share a drawn size; glyphs from different blocks do not,
// whatever the width table says about their cells. This was learned by breaking it — the
// preview was briefly ▣ (Geometric Shapes), which is the same idea as ⧇ at half the drawn
// size, and it looked like a bug beside ⧆ and ⧉ (§9.36).
//
// So a fourth glyph, if one is ever added, comes from U+29xx or it does not match. That is a
// harder constraint than it sounds and it is the reason the arrow the preview started as could
// not be kept: there is no arrow in this block.
//
// All three being squares is the cost of that rule, and it means **colour carries the work of
// telling them apart** — which is why the three values are matched by measurement rather than
// chosen (palette.go). Position helps: the slots are fixed, so which one a glyph sits in is
// itself a cue.
//
// All are single cell under the East Asian Width table, which is what decides the width
// Ghostty allocates. That is the check that matters for the fit, and it is a different question
// from the drawn size above: a glyph can occupy one cell and be drawn small inside it.
// The state marks, and the one that qualifies them. ○ quiet, ◐ working, ▫ a todo — Geometric
// Shapes, unchanged since the frame was drawn. ⧗ is new and replaces ⚠ on a stale row: a warning
// triangle says "something is wrong", and a session nobody has looked at for three hours is not
// wrong, it is *waiting*. An hourglass says that, and it comes from the same block as the link
// glyphs so it is drawn at their size (§9.41).
const (
	quietGlyph   = "○"
	workingGlyph = "◐"
	todoGlyph    = "▫"
	staleGlyph   = "⧗"
)

const (
	previewGlyph   = "⧇"
	storybookGlyph = "⧆"
	folderGlyph    = "⧉"
	// ⧭ is a circle with a down arrow, which is literally a *pull* mark — and a circle, so it is
	// the one link glyph that cannot be mistaken for the three squares at a glance.
	prGlyph = "⧭"
)

// actionCell is the row's trailing links, each on its own column so they line up down the
// band whichever of them a row happens to have. Empty when the row has none — an empty cell
// would pad every row of a fleet that has nothing to point at.
//
// Order is by how often a row has one, rarest first — Storybook, preview, folder — with the pull
// request outside that run at the far right. The folder is on nearly every row, so ending the
// local run with it keeps the trailing trim from firing; the Storybook is the rarest, so its empty
// column falls on the left where it costs nothing to look at.
//
// The PR sits beyond all of them because it is the only one that does not point at this machine.
// The first three are a port, a port and a directory; ⧭ is github.com. Grouping by locality is
// why it is last rather than where its frequency would put it (§18).
//
// Green for the live thing and cyan for the editor, matched in measured contrast so neither
// dominates the other (see linkPreview/linkFolder). Colour is what separates the two glyphs
// at a glance; shape is what separates them when it is not read. Neither is the accent and
// neither is dim: these are affordances rather than data, and exactly one element in the
// frame is allowed to shout (§6). Under cmux the terminal underlines them on hover with no
// modifier — board does not enable mouse reporting, so Ghostty evaluates links locally — which
// is what makes them findable without a legend line. Note that *opening* one is ⌘-click and not
// click: Ghostty wants the ctrl/super chord held at press so a text selection cannot fire a
// link. That half is a platform convention the frame cannot teach, so the README says it (§18).
func actionCell(r board.Row, scheme string) string {
	storybook, preview, folder, pr := " ", " ", " ", " "
	if r.Preview != "" {
		preview = link(r.Preview, fg(linkPreview, previewGlyph))
	}
	if r.Storybook != "" {
		storybook = link(r.Storybook, fg(linkStorybook, storybookGlyph))
	}
	if url := editorURL(scheme, r.Folder); url != "" {
		folder = link(url, fg(linkFolder, folderGlyph))
	}
	if r.PR != "" {
		pr = link(r.PR, fg(linkPR, prGlyph))
	}
	// Right-trimmed because nothing follows it on the line: a row whose only link is the
	// Storybook would otherwise end in six columns of padding.
	return strings.TrimRight(storybook+" "+preview+" "+folder+" "+pr, " ")
}
