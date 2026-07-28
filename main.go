// board — one screen showing every live cmux agent session.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	// Tail of the audit log scanned to find each session's newest event.
	// 8MB covers ~2 days; older sessions fall back to their lifecycle state.
	tailBytes        = 8 << 20
	defaultThreshold = 45 * time.Minute
)

// A pending Feed card is the only trustworthy "needs an answer" signal.
// cmux's needsInput lifecycle fires ~60s after any finished turn, so it means
// "sitting at the prompt", not "asked you something".
var actionable = map[string]bool{"question": true, "permissionRequest": true, "exitPlan": true}

func home(p ...string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(append([]string{h}, p...)...)
}

type state struct {
	Config struct {
		IdleThresholdMinutes int    `json:"idle_threshold_minutes"`
		NotifyCmd            string `json:"notify_cmd"`
	} `json:"config"`
	Labels map[string]string `json:"labels"`
}

func loadState() *state {
	s := &state{Labels: map[string]string{}}
	if b, err := os.ReadFile(home(".board.json")); err == nil {
		json.Unmarshal(b, s)
	}
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	return s
}

func (s *state) save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(home(".board.json"), b, 0o600)
}

func (s *state) threshold() time.Duration {
	if s.Config.IdleThresholdMinutes > 0 {
		return time.Duration(s.Config.IdleThresholdMinutes) * time.Minute
	}
	return defaultThreshold
}

type session struct {
	AgentLifecycle string  `json:"agentLifecycle"`
	Cwd            string  `json:"cwd"`
	Pid            int     `json:"pid"`
	SessionID      string  `json:"sessionId"`
	SurfaceID      string  `json:"surfaceId"`
	UpdatedAt      float64 `json:"updatedAt"`
}

// readSessions returns one live session per surface, newest wins.
func readSessions() []session {
	b, err := os.ReadFile(home(".cmuxterm", "claude-hook-sessions.json"))
	if err != nil {
		return nil
	}
	var file struct {
		Sessions map[string]session `json:"sessions"`
	}
	if json.Unmarshal(b, &file) != nil {
		return nil
	}
	live := map[string]session{}
	for _, s := range file.Sessions {
		if s.Pid == 0 || syscall.Kill(s.Pid, 0) != nil {
			continue
		}
		key := s.SurfaceID
		if key == "" {
			key = s.SessionID
		}
		if prev, ok := live[key]; ok && prev.UpdatedAt >= s.UpdatedAt {
			continue
		}
		live[key] = s
	}
	out := make([]session, 0, len(live))
	for _, s := range live {
		out = append(out, s)
	}
	return out
}

type titles struct{ surface, workspace string }

// cmuxTitles maps agent pid -> its surface and workspace titles. cmux surface
// nodes carry no UUID, so pid is the join key; cmux_process_pids is 1:1.
func cmuxTitles() map[int]titles {
	out := map[int]titles{}
	b, err := cmux("top", "--all", "--json")
	if err != nil {
		return out
	}
	var root any
	if json.Unmarshal(b, &root) != nil {
		return out
	}
	walkTree(root, "", out)
	return out
}

func walkTree(n any, ws string, out map[int]titles) {
	switch v := n.(type) {
	case map[string]any:
		kind, _ := v["kind"].(string)
		title, _ := v["title"].(string)
		if kind == "workspace" {
			ws = title
		}
		if kind == "surface" {
			pids, _ := v["cmux_process_pids"].([]any)
			for _, p := range pids {
				if f, ok := p.(float64); ok {
					out[int(f)] = titles{stripSpinner(title), ws}
				}
			}
		}
		for _, c := range v {
			walkTree(c, ws, out)
		}
	case []any:
		for _, c := range v {
			walkTree(c, ws, out)
		}
	}
}

// stripSpinner drops the leading activity glyph Claude Code puts in tab titles
// (✳ or a braille spinner frame) so labels stay stable between renders.
func stripSpinner(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > 0 && (r[0] == '✳' || (r[0] >= 0x2800 && r[0] <= 0x28FF)) {
		return strings.TrimSpace(string(r[1:]))
	}
	return string(r)
}

// newestKinds finds the most recent event kind per session by scanning the
// audit log backwards, stopping as soon as every session is accounted for.
func newestKinds(want map[string]bool) map[string]string {
	out := map[string]string{}
	f, err := os.Open(home(".cmuxterm", "workstream.jsonl"))
	if err != nil {
		return out
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return out
	}
	off := st.Size() - tailBytes
	if off < 0 {
		off = 0
	}
	buf := make([]byte, st.Size()-off)
	n, _ := f.ReadAt(buf, off)
	lines := bytes.Split(buf[:n], []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		id := strings.TrimPrefix(jsonField(lines[i], "workstreamId"), "claude-")
		if id == "" || !want[id] {
			continue
		}
		if _, seen := out[id]; seen {
			continue
		}
		if kind := jsonField(lines[i], "kind"); kind != "" {
			out[id] = kind
		}
		if len(out) == len(want) {
			break
		}
	}
	return out
}

// jsonField pulls one string value without decoding the whole record; the audit
// log is written compact and unindented, so this is safe and much cheaper.
func jsonField(line []byte, key string) string {
	k := []byte(`"` + key + `":"`)
	i := bytes.Index(line, k)
	if i < 0 {
		return ""
	}
	rest := line[i+len(k):]
	j := bytes.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return string(rest[:j])
}

// cmux runs a read-only query. cmux treats CMUX_SURFACE_ID/CMUX_WORKSPACE_ID as
// the implicit target for every command, so a stale inherited value makes even a
// global query fail; strip them and ask for the whole fleet explicitly.
func cmux(args ...string) ([]byte, error) {
	c := exec.Command("cmux", args...)
	c.Env = append(os.Environ(), "CMUX_QUIET=1",
		"CMUX_SURFACE_ID=", "CMUX_WORKSPACE_ID=", "CMUX_TAB_ID=", "CMUX_PANEL_ID=")
	return c.Output()
}

func haveCmux() bool {
	if os.Getenv("CMUX_WORKSPACE_ID") != "" {
		return true
	}
	_, err := exec.LookPath("cmux")
	return err == nil
}

type row struct {
	state, label, workspace string
	idle                    time.Duration
	stale                   bool
	rank                    int
}

func show() {
	if !haveCmux() {
		fmt.Println("board: cmux not detected (no CMUX_WORKSPACE_ID, no cmux on PATH).")
		return
	}
	sessions := readSessions()
	if len(sessions) == 0 {
		fmt.Println("board: no live agent sessions.")
		return
	}
	st := loadState()
	byPid := cmuxTitles()
	want := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		want[s.SessionID] = true
	}
	kinds := newestKinds(want)

	now := time.Now()
	rows := make([]row, 0, len(sessions))
	var blocked, running, stale int
	for _, s := range sessions {
		t := byPid[s.Pid]
		label := st.Labels[s.SurfaceID]
		if label == "" {
			label = t.surface
		}
		if label == "" {
			label = "(unlabeled)"
		}
		ws := t.workspace
		if ws == "" {
			ws = filepath.Base(s.Cwd)
		}
		idle := now.Sub(time.Unix(int64(s.UpdatedAt), 0))
		if idle < 0 {
			idle = 0
		}
		r := row{state: "done", label: label, workspace: ws, idle: idle, rank: 1}
		switch {
		case actionable[kinds[s.SessionID]]:
			r.state, r.rank = "blocked →", 0
			blocked++
		case s.AgentLifecycle == "running":
			r.state, r.rank = "running", 2
			running++
		}
		// A running agent's clock is legitimately old between hook events, so
		// only quiet sessions earn the forgotten-work flag.
		if r.rank != 2 && idle > st.threshold() {
			r.stale = true
			stale++
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return rows[i].idle > rows[j].idle
	})

	fmt.Printf("%s %s %s %6s\n", pad("STATE", 12), pad("LABEL", 44), pad("WORKSPACE", 17), "IDLE")
	for _, r := range rows {
		s := r.state
		if r.stale {
			s += " ⚠"
		}
		fmt.Printf("%s %s %s %6s\n", pad(s, 12), pad(r.label, 44), pad(r.workspace, 17), humanize(r.idle))
	}
	fmt.Printf("\n%d sessions · %d blocked · %d running · %d quiet >%s\n",
		len(rows), blocked, running, stale, humanize(st.threshold()))
}

func pad(s string, w int) string {
	r := []rune(s)
	if len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(r))
}

func humanize(d time.Duration) string {
	m := int(d.Minutes())
	switch {
	case m < 60:
		return fmt.Sprintf("%dm", m)
	case m < 24*60:
		return fmt.Sprintf("%dh%02dm", m/60, m%60)
	default:
		return fmt.Sprintf("%dd%02dh", m/1440, (m/60)%24)
	}
}

func label(text string) error {
	surface := os.Getenv("CMUX_SURFACE_ID")
	if surface == "" {
		return fmt.Errorf("CMUX_SURFACE_ID unset — run this from inside a cmux session")
	}
	st := loadState()
	if text == "" {
		delete(st.Labels, surface)
	} else {
		st.Labels[surface] = text
	}
	if err := st.save(); err != nil {
		return err
	}
	if text == "" {
		fmt.Println("label cleared")
	} else {
		fmt.Printf("labeled: %s\n", text)
	}
	return nil
}

// notify is the Claude Code hook entrypoint. It must never fail the agent, so
// every error path is a silent success.
func notify() {
	st := loadState()
	if st.Config.NotifyCmd == "" {
		return
	}
	var hook struct {
		Event   string `json:"hook_event_name"`
		Message string `json:"message"`
		Cwd     string `json:"cwd"`
	}
	if in, err := io.ReadAll(os.Stdin); err == nil {
		json.Unmarshal(in, &hook)
	}
	surface := os.Getenv("CMUX_SURFACE_ID")
	label := st.Labels[surface]
	if label == "" {
		label = filepath.Base(hook.Cwd)
	}
	ws := workspaceTitle(os.Getenv("CMUX_WORKSPACE_ID"))
	if ws == "" {
		ws = "?"
	}
	verb := "finished"
	if hook.Event == "Notification" {
		verb = "needs input"
	}
	text := fmt.Sprintf("%s — %s [%s]", verb, label, ws)
	if hook.Message != "" {
		text += ": " + hook.Message
	}
	payload, _ := json.Marshal(map[string]string{
		"event": hook.Event, "state": verb, "label": label, "workspace": ws,
		"surface_id": surface, "cwd": hook.Cwd, "message": hook.Message, "text": text,
	})
	c := exec.Command("sh", "-c", st.Config.NotifyCmd)
	c.Stdin = bytes.NewReader(payload)
	c.Run()
}

func workspaceTitle(id string) string {
	if id == "" {
		return ""
	}
	b, err := cmux("workspace", "list", "--json", "--id-format", "both")
	if err != nil {
		return ""
	}
	var f struct {
		Workspaces []struct{ ID, Title string } `json:"workspaces"`
	}
	json.Unmarshal(b, &f)
	for _, w := range f.Workspaces {
		if strings.EqualFold(w.ID, id) {
			return w.Title
		}
	}
	return ""
}

func installHooks() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	path := home(".claude", "settings.json")
	settings := map[string]any{}
	var original []byte
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil {
			return fmt.Errorf("refusing to rewrite unparseable %s: %w", path, err)
		}
		original = b
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	marker := filepath.Base(self) + " notify"
	var added []string
	for _, event := range []string{"Stop", "Notification"} {
		list, _ := hooks[event].([]any)
		if hasCommand(list, marker) {
			continue
		}
		hooks[event] = append(list, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": self + " notify"}},
		})
		added = append(added, event)
	}
	if len(added) == 0 {
		fmt.Println("hooks already installed — nothing to do")
		return nil
	}
	if original != nil {
		bak := fmt.Sprintf("%s.board-bak-%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(bak, original, 0o600); err != nil {
			return err
		}
		fmt.Println("backed up:", bak)
	}
	settings["hooks"] = hooks
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Printf("installed hooks: %s\n", strings.Join(added, ", "))
	if loadState().Config.NotifyCmd == "" {
		fmt.Printf("set notify_cmd in %s to start pushing (it receives JSON on stdin)\n", home(".board.json"))
	}
	return nil
}

func hasCommand(list []any, marker string) bool {
	for _, group := range list {
		g, _ := group.(map[string]any)
		entries, _ := g["hooks"].([]any)
		for _, e := range entries {
			h, _ := e.(map[string]any)
			if c, _ := h["command"].(string); strings.Contains(c, marker) {
				return true
			}
		}
	}
	return false
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		show()
		return
	}
	var err error
	switch args[0] {
	case "label":
		err = label(strings.Join(args[1:], " "))
	case "notify":
		notify()
	case "install-hooks":
		err = installHooks()
	default:
		fmt.Fprintln(os.Stderr, "usage: board | board label \"<text>\" | board install-hooks")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "board:", err)
		os.Exit(1)
	}
}
