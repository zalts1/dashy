package main

// The ledger: what asked you something, how long it waited, and what was never
// answered at all. Questions and plan exits only — auto-approved permission
// requests resolve without a human, so they are not accountability.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ask struct {
	at     time.Time
	what   string // question header, or "plan review"
	where  string // cwd basename
	waited time.Duration
	open   bool // no terminating event: never answered
}

type event struct {
	ws, kind, tool string
	at             time.Time
	line           []byte
}

// readTail returns the parsed skeleton of the audit-log tail. One pass, string
// search only — payloads are decoded later and only for the few rows that need it.
func readTail() []event {
	f, err := os.Open(home(".cmuxterm", "workstream.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil
	}
	off := st.Size() - tailBytes
	if off < 0 {
		off = 0
	}
	buf := make([]byte, st.Size()-off)
	n, _ := f.ReadAt(buf, off)

	lines := bytes.Split(buf[:n], []byte("\n"))
	out := make([]event, 0, len(lines))
	for _, ln := range lines {
		kind := jsonField(ln, "kind")
		if kind == "" {
			continue
		}
		at, _ := time.Parse(time.RFC3339, jsonField(ln, "createdAt"))
		out = append(out, event{
			ws:   strings.TrimPrefix(jsonField(ln, "workstreamId"), "claude-"),
			kind: kind,
			tool: jsonField(ln, "toolName"),
			at:   at,
			line: ln,
		})
	}
	return out
}

func (e event) isActionable() bool {
	return actionable[e.kind] || (e.kind == "toolUse" && actionableTool[e.tool])
}

// settles reports whether this event proves the agent stopped waiting. toolResult
// does not: cmux's Feed bridge emits one when its semaphore expires, while Claude
// keeps waiting at its own picker.
func (e event) settles() bool {
	return e.kind != "toolResult" && !e.isActionable()
}

// coverage reports the oldest event in the tail, so callers can state the window
// they actually have rather than the one they asked for.
func coverage(events []event) time.Time {
	for _, e := range events {
		if !e.at.IsZero() {
			return e.at
		}
	}
	return time.Time{}
}

// asks extracts question and plan-exit episodes at or after since. wsOf maps a
// session id to its cmux workspace title so the ledger names a session the same
// way the rest of the screen does; it falls back to the cwd basename.
func asks(events []event, since time.Time, wsOf map[string]string) []ask {
	bySession := map[string][]int{}
	for i, e := range events {
		if e.ws != "" {
			bySession[e.ws] = append(bySession[e.ws], i)
		}
	}
	var out []ask
	for _, idx := range bySession {
		for n, i := range idx {
			e := events[i]
			if e.kind != "question" && e.kind != "exitPlan" {
				continue
			}
			if e.at.Before(since) {
				continue
			}
			where := wsOf[e.ws]
			if where == "" {
				where = filepath.Base(jsonField(e.line, "cwd"))
			}
			a := ask{at: e.at, what: "plan review", open: true, where: where}
			if e.kind == "question" {
				if h := questionHeader(e.line); h != "" {
					a.what = h
				}
			}
			for _, j := range idx[n+1:] {
				if events[j].settles() {
					a.waited, a.open = events[j].at.Sub(e.at), false
					break
				}
			}
			out = append(out, a)
		}
	}
	return out
}

func questionHeader(line []byte) string {
	var rec struct {
		Payload struct {
			Question struct {
				Questions []struct{ Header string } `json:"questions"`
			} `json:"question"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &rec) != nil {
		return ""
	}
	if q := rec.Payload.Question.Questions; len(q) > 0 {
		return q[0].Header
	}
	return ""
}
