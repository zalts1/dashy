package view

import (
	"bytes"
	"strings"
)

// The hit map turns a screen position into a session, and it is the only place in this
// package that goes in that direction — everything else here is keyed on identity, for
// the reasons order.go states. So it may not be *derived*: a map computed beside the
// renderer is a second copy of the layout, and the first band to move would leave the two
// pointing at different rows with nothing to notice it. It is written by the same pass
// that writes the lines (DESIGN.md §3, §7).

// clipHits drops the entries clip dropped. A hit map longer than the frame would resolve
// a click on a line the terminal is not showing.
func clipHits(h []string, n int) []string {
	if n < 0 {
		n = 0
	}
	if len(h) > n {
		return h[:n]
	}
	return h
}

// painter accumulates the frame's body and, line for line, whose row each line is. The
// two are written together because that is the only way they cannot disagree. It relies
// on one invariant compose keeps throughout: **every write ends on a line boundary**, so
// a write's newline count is how many lines it added.
type painter struct {
	b    bytes.Buffer
	hits []string
}

// put writes chrome: lines nothing can be clicked on.
func (p *painter) put(s string) { p.write("", s) }

// row writes one row's line and records the key a click there resolves to.
func (p *painter) row(key, s string) { p.write(key, s) }

func (p *painter) write(key, s string) {
	p.b.WriteString(s)
	for range strings.Count(s, "\n") {
		p.hits = append(p.hits, key)
		key = ""
	}
}
