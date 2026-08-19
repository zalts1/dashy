package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalts1/dashy/internal/maki"
)

// board's half of a maki init.lua is one marked block, and these are the rules that make
// editing somebody else's Lua safe: it goes in at the top, it comes out whole, and a file
// whose markers do not make sense is refused rather than guessed at (§8).

const userInit = `-- my own maki config
vim = nil

return {
  model = "anthropic/claude-opus-4-6",
}
`

// The block is prepended, and that is not a style choice. A maki init.lua returns its
// config, and in Lua a `return` must end its block — anything appended after one is a
// syntax error, which would break maki's startup on every machine board touched.
func TestTheBlockGoesInAtTheTop(t *testing.T) {
	got, err := withMakiBlock(userInit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, makiBegin) {
		t.Errorf("block did not land first:\n%s", first(got, 3))
	}
	if !strings.HasSuffix(got, userInit) {
		t.Error("the user's file did not survive verbatim after the block")
	}
	if strings.Index(got, makiEnd) > strings.Index(got, "return {") {
		t.Error("the block straddles the user's return; nothing may sit after it")
	}
}

// Install and uninstall are inverses down to the byte. Anything less and a cycle
// accretes blank lines in a file board only borrows.
func TestUninstallIsTheInverseOfInstallForMaki(t *testing.T) {
	for _, src := range []string{userInit, "", "-- just a comment\n"} {
		with, err := withMakiBlock(src)
		if err != nil {
			t.Fatal(err)
		}
		back, removed, err := withoutMakiBlock(with)
		if err != nil {
			t.Fatal(err)
		}
		if !removed {
			t.Fatalf("nothing removed from a file board had just written")
		}
		if back != src {
			t.Errorf("round trip changed the file:\nwant %q\ngot  %q", src, back)
		}
	}
}

// Idempotence, and the reason it matters here: two copies of the block would register the
// autocmds twice and write the report twice on every turn.
func TestTheBlockIsFoundAfterItIsWritten(t *testing.T) {
	with, err := withMakiBlock(userInit)
	if err != nil {
		t.Fatal(err)
	}
	if !makiBlockPresent(with) {
		t.Error("board's own block was not recognised; a second install would duplicate it")
	}
	if makiBlockPresent(userInit) {
		t.Error("a file board has never touched was reported as having the block")
	}
}

// Somebody else's file, so anything can be in it. A half-deleted block is the one state
// board must not repair by guessing: it cannot know where the block ended, and the
// wrong guess deletes the user's Lua. Same refusal install-hooks makes on an
// unparseable settings.json (§8).
func TestUnbalancedMarkersAreRefused(t *testing.T) {
	cases := map[string]string{
		"begin with no end": makiBegin + "\nwhatever\n",
		"end with no begin": "whatever\n" + makiEnd + "\n",
		"end before begin":  makiEnd + "\nwhatever\n" + makiBegin + "\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := withMakiBlock(src); err == nil {
				t.Error("install rewrote a file with unbalanced markers")
			}
			if _, _, err := withoutMakiBlock(src); err == nil {
				t.Error("uninstall rewrote a file with unbalanced markers")
			}
		})
	}
}

// Nothing to remove is a report, not an error — the same answer uninstall-hooks gives on
// a settings.json with no board entries in it.
func TestRemovingFromAFileWithoutTheBlock(t *testing.T) {
	back, removed, err := withoutMakiBlock(userInit)
	if err != nil {
		t.Fatalf("a file with no block reported an error: %v", err)
	}
	if removed {
		t.Error("reported a removal from a file that never had the block")
	}
	if back != userInit {
		t.Error("a file with nothing to remove was rewritten anyway")
	}
}

// What the block is for. It is Lua board ships and cannot compile, so the parts that
// have to line up with the reader on the Go side are pinned here: the roster directory,
// the tab id the file is named after, and the API calls that produce the report.
func TestTheBlockReportsWhatBoardReads(t *testing.T) {
	for _, want := range []string{
		"/.board/maki",            // the directory internal/maki reads
		"CMUX_SURFACE_ID",         // the tab id, which is the join key and the file name
		"maki.session.live()",     // the roster itself
		"SessionStatusChanged",    // blocked and running arrive on this event
		"maki.fs.atomic_write",    // a reader sees the old file or the whole new one
		"maki.api.create_autocmd", // registration
	} {
		if !strings.Contains(makiBlock, want) {
			t.Errorf("the block does not mention %q", want)
		}
	}
	// Never appended, so it must not be the last thing in the file it is written into,
	// and it must be a statement rather than an expression: `do ... end` is both.
	if !strings.Contains(makiBlock, "do\n") || !strings.Contains(makiBlock, "\nend\n") {
		t.Error("the block is not a self-contained do...end statement")
	}
}

func first(s string, n int) string {
	lines := strings.Split(s, "\n")
	return strings.Join(lines[:min(n, len(lines))], "\n")
}

// fakeMaki puts a maki on PATH so installMaki does not skip, and isolates $HOME so the
// init.lua it writes is a temp one. Returns the path board will edit.
func fakeMaki(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "maki"), []byte("#!/bin/sh\necho maki 0.0.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return filepath.Join(home, ".config", "maki", "init.lua")
}

// The whole install, on a machine that has never run maki: board creates the init.lua
// maki would load, and a second run changes nothing.
func TestInstallMakiCreatesAndIsIdempotent(t *testing.T) {
	path := fakeMaki(t)
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no init.lua was created: %v", err)
	}
	if !strings.Contains(string(first), makiBegin) {
		t.Error("the block is not in the file board wrote")
	}
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(first) {
		t.Error("a second install changed the file; the autocmds would register twice")
	}
	block, manifest, err := MakiInstalled()
	if err != nil || !block || !manifest {
		t.Errorf("MakiInstalled = %v, %v, %v — the checker disagrees with the installer",
			block, manifest, err)
	}
}

// The manifest is half the install. maki denies every permission to an init.lua that has
// no plugin.toml beside it, so without this the block installs, runs, and does nothing at
// all — and says so nowhere (§9.32). Measured against maki 0.4.9, not argued.
func TestInstallMakiGrantsTheBlockThePermissionsItNeeds(t *testing.T) {
	path := fakeMaki(t)
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(path), "plugin.toml"))
	if err != nil {
		t.Fatalf("no plugin.toml was written: %v", err)
	}
	got := string(b)
	// env for CMUX_SURFACE_ID, fs_write for the report. Both are what the block calls.
	for _, want := range []string{"[permissions]", "fs_write = true", "env = true"} {
		if !strings.Contains(got, want) {
			t.Errorf("the manifest does not grant %q:\n%s", want, got)
		}
	}
	// Nothing is denied: an absent key is allowed, so board must not quietly narrow what
	// Lua the user adds to this init.lua later can do.
	if strings.Contains(got, "false") {
		t.Errorf("board denied a permission it was not asked about:\n%s", got)
	}
	if _, manifest, _ := MakiInstalled(); !manifest {
		t.Error("the checker does not see the manifest the installer just wrote")
	}
}

// A block with no manifest is installed and inert, and that is not the same state as
// installed — reporting it as installed is what hid §9.32 in the first place.
func TestMakiInstalledSeparatesTheBlockFromTheManifest(t *testing.T) {
	path := fakeMaki(t)
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(path), "plugin.toml")); err != nil {
		t.Fatal(err)
	}
	block, manifest, err := MakiInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if !block || manifest {
		t.Errorf("block/manifest = %v/%v, want the block alone", block, manifest)
	}
}

// A plugin.toml is policy for every bit of Lua in that directory, so the user's is not
// board's to rewrite — and not board's to delete either.
func TestTheUsersOwnManifestIsLeftAlone(t *testing.T) {
	path := fakeMaki(t)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "[permissions]\nnet = false\n"
	manifest := filepath.Join(dir, "plugin.toml")
	if err := os.WriteFile(manifest, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(manifest); string(b) != mine {
		t.Errorf("board rewrote the user's manifest:\n%s", string(b))
	}
	if err := uninstallMaki(); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(manifest); err != nil || string(b) != mine {
		t.Errorf("board deleted the user's manifest: %v", err)
	}
}

// A file board created and nobody else uses must not be left behind empty: an install
// and an uninstall together leave no trace, which is what §15 asks of the settings file.
func TestUninstallMakiLeavesNoEmptyInitFile(t *testing.T) {
	path := fakeMaki(t)
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	if err := uninstallMaki(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("init.lua survived as %v; board created it and it now holds nothing", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "plugin.toml")); !os.IsNotExist(err) {
		t.Errorf("the manifest board wrote survived as %v", err)
	}
}

// The user's own Lua survives, and a backup is taken before the first change — the same
// two promises install-hooks makes about settings.json.
func TestInstallMakiKeepsTheUsersLuaAndBacksItUp(t *testing.T) {
	path := fakeMaki(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(userInit), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(got), userInit) {
		t.Error("the user's init.lua did not survive verbatim")
	}
	baks, _ := filepath.Glob(path + ".board-bak-*")
	if len(baks) != 1 {
		t.Fatalf("found %d backups, want exactly one", len(baks))
	}
	if b, _ := os.ReadFile(baks[0]); string(b) != userInit {
		t.Error("the backup is not the file as it stood before the change")
	}
	// And the file board only borrows keeps the mode it had.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644 — board tightened a file it does not own", fi.Mode().Perm())
	}
	if err := uninstallMaki(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != userInit {
		t.Errorf("uninstall did not restore the file byte for byte:\n%q", string(b))
	}
}

// The reports are board's litter even though the block writes them, so uninstall clears
// them: a stale roster directory outliving the hook is a trace of a removed tool.
func TestUninstallMakiClearsTheReports(t *testing.T) {
	fakeMaki(t)
	dir := maki.RosterDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "S-1.json"), []byte(`{"surface":"S-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := uninstallMaki(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("roster directory survived as %v", err)
	}
}

// Nothing to wire up is a report, not a failure. install-hooks is one command for two
// agents, and the half that cannot apply must not fail the half that can.
func TestInstallMakiSkipsAMachineWithoutMaki(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PATH", t.TempDir())
	if err := installMaki(); err != nil {
		t.Errorf("installMaki on a machine without maki = %v, want nil", err)
	}
	if _, err := os.Stat(maki.InitPath()); !os.IsNotExist(err) {
		t.Error("board created an init.lua for a maki that is not installed")
	}
}

// A file that held nothing but board's block is board's, so removing it leaves no backup
// either: a `.board-bak-*` copy of board's own Lua is exactly the trace §15 says uninstall
// must not leave. The backup exists to protect the user's file, and there was none.
func TestUninstallMakiDoesNotBackUpAFileOnlyBoardWrote(t *testing.T) {
	path := fakeMaki(t)
	if err := installMaki(); err != nil {
		t.Fatal(err)
	}
	if err := uninstallMaki(); err != nil {
		t.Fatal(err)
	}
	baks, _ := filepath.Glob(path + ".board-bak-*")
	if len(baks) != 0 {
		t.Errorf("left %v behind — a backup of board's own block", baks)
	}
}
