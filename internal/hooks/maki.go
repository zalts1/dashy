package hooks

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalts1/dashy/internal/maki"
)

// board's maki integration: the Lua that reports the roster, and the marked block it
// lives in inside the user's init.lua.
//
// This is the same job install.go does for Claude Code and it is a different mechanism,
// because maki has no settings file with a hooks table in it. What maki has is a Lua
// plugin API, so board's hook is Lua — and the file it goes into is a program, not data,
// which changes two things. It cannot be parsed to decide whether an edit is safe, so
// the markers carry that instead. And it cannot be appended to: a maki init.lua returns
// its config, `return` must end a Lua block, and anything after one is a syntax error.
//
// It is also two files. maki denies every permission to an init.lua that has no
// plugin.toml beside it, so the block installs, runs, reads nothing and writes nothing,
// and says so nowhere (EVIDENCE.md §9.32). The manifest is half the install.

// The markers delimiting board's block. Fixed, where the Claude Code hook's marker is
// the running binary's name (see marker()): that hook runs `<binary> notify`, so a
// renamed copy must claim its own entry, while this block runs no binary at all. It
// writes to ~/.board/maki, which is board's name in the filesystem and not the
// executable's, so a renamed copy reads the same directory and wants the same block.
const (
	makiBegin = "-- >>> board >>> installed by `board install-hooks`, removed by `board uninstall-hooks`"
	makiEnd   = "-- <<< board <<<"
)

// makiBlock is the whole of board's presence in a maki session, and every line of it is
// load-bearing:
//
//   - the report is written on status changes, not polled, because SessionStatusChanged
//     is exactly the transition board draws — it fires when a permission prompt opens,
//     which is maki's `needs_input` and board's `blocked →`.
//   - one file per cmux tab, named for the tab, because that is the key board joins on
//     and it means a maki restarted in the same tab overwrites its own report.
//   - no report without CMUX_SURFACE_ID: a maki outside cmux is a session board has
//     nowhere to send you, and board is a view of tabs.
//   - `atomic_write`, so board's every-10s read sees the old file or the whole new one.
//   - the top-level reads are wrapped in pcall and the task body is handed to
//     `maki.async.run`'s own error path, because §8 says board must never fail the
//     agent — and here that means a broken block must not stop maki starting.
const makiBlock = makiBegin + `
-- Reports this maki's live sessions to board, which has no other way to see them:
-- ~/.board/maki/<cmux surface id>.json, rewritten whenever a session changes state.
do
  local ok, dir, surface = pcall(function()
    return (maki.uv.os_homedir() or "") .. "/.board/maki",
      maki.uv.os_getenv("CMUX_SURFACE_ID")
  end)
  if ok and surface and surface ~= "" then
    local function report()
      local live = maki.session.live()
      if not live or #live == 0 then
        return
      end
      local sessions = {}
      for _, s in ipairs(live) do
        sessions[#sessions + 1] = {
          id = s.id,
          title = s.title,
          status = s.status,
          updated_at = s.updated_at,
        }
      end
      local body = maki.json.encode({
        surface = surface,
        cwd = maki.uv.cwd(),
        sessions = sessions,
      })
      if body then
        maki.fs.mkdir(dir, { parents = true })
        maki.fs.atomic_write(dir .. "/" .. surface .. ".json", body)
      end
    end
    maki.api.create_autocmd({
      "SessionStatusChanged",
      "SessionFocusChanged",
      "SessionReset",
      "TurnStart",
      "TurnEnd",
      "TurnError",
    }, {
      callback = function()
        -- The second argument is the error sink: it gives the task maki's own pcall and
        -- discards what it catches, so nothing board wrote can surface as a maki error.
        maki.async.run(report, function() end)
      end,
    })
  end
end
` + makiEnd + "\n"

// makiManifestMarker is how board recognises a manifest it wrote, which is the only one
// uninstall will delete. A user who edits this file owns it from then on.
const makiManifestMarker = "# written by board install-hooks"

// makiManifest is the other half of the install. Two grants and no denials: an absent key
// means allowed, so board asks for exactly what its block uses — the environment, for
// CMUX_SURFACE_ID, and the filesystem, for the report — and decides nothing about Lua the
// user adds to this init.lua later.
const makiManifest = makiManifestMarker + `
#
# maki denies every permission to an init.lua with no plugin.toml beside it, so board's
# block would install, run, and quietly do nothing without this file. Only the two
# permissions it uses are named; the keys left out stay allowed, as maki defaults them.
#
# board uninstall-hooks deletes this file only while the first line above is still in it.
[permissions]
fs_write = true
env = true
`

// defaultInitMode applies only when board creates init.lua. 0644, where settings.json
// gets 0600: this file is code and holds no credentials, and 0600 on a config somebody
// will edit by hand is a tightening board has no business making on their behalf.
const defaultInitMode fs.FileMode = 0o644

// installMaki prepends the block to the user's init.lua, creating the file if there is
// none. Reports what it did, in one line, like its Claude Code counterpart.
func installMaki() error {
	if !maki.Available() {
		fmt.Println("maki not found — nothing to wire up for it")
		return nil
	}
	path := maki.InitPath()
	src, original, mode, err := readInit(path)
	if err != nil {
		return err
	}
	if makiBlockPresent(src) {
		fmt.Println("maki hook already installed — nothing to do")
		return nil
	}
	next, err := withMakiBlock(src)
	if err != nil {
		return err
	}
	if err := backup(path, original, mode); err != nil {
		return err
	}
	if err := writeInit(path, next, mode); err != nil {
		return err
	}
	fmt.Println("installed maki hook:", path)
	return installMakiManifest(filepath.Dir(path))
}

// installMakiManifest writes the permission manifest the block cannot work without, and
// never over one that is already there: a manifest is Lua policy for everything in that
// directory, and the user's is not board's to rewrite. Saying what the block needs is
// then the whole of what board can honestly do about it.
func installMakiManifest(dir string) error {
	path := filepath.Join(dir, "plugin.toml")
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("kept your %s — board's block needs fs_write and env in it\n", path)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(makiManifest), defaultInitMode); err != nil {
		return err
	}
	fmt.Println("granted it fs_write and env:", path)
	return nil
}

// uninstallMaki takes the block back out and clears the reports it produced, so nothing
// board wrote outlives board — the same rule §15 holds the settings file to.
//
// It does not ask whether maki is installed. A machine that has removed maki and kept
// the block is exactly the machine that needs this to still work.
func uninstallMaki() error {
	path := maki.InitPath()
	src, original, mode, err := readInit(path)
	if err != nil {
		return err
	}
	next, removed, err := withoutMakiBlock(src)
	if err != nil {
		return err
	}
	switch {
	case !removed:
		fmt.Printf("no board block in %s — nothing to do\n", path)
	case strings.TrimSpace(next) == "":
		// A file that is now only whitespace held nothing but board's block, so board
		// created it and nothing else uses it. It goes, and no backup is taken: maki reads
		// an absent file and an empty one alike, and a `.board-bak-*` copy of board's own
		// Lua is exactly the trace §15 says uninstall must not leave. The backup below
		// exists to protect the user's file, and here there was none.
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Println("removed maki hook and the", path, "board created for it")
	default:
		if err := backup(path, original, mode); err != nil {
			return err
		}
		if err := writeInit(path, next, mode); err != nil {
			return err
		}
		fmt.Println("removed maki hook from", path)
	}
	if err := uninstallMakiManifest(filepath.Dir(path)); err != nil {
		return err
	}
	// Reports are written by the block, so they are board's litter even though board did
	// not write them. RemoveAll on a missing directory is a success, which is the answer
	// wanted on a machine that never ran the hook.
	return os.RemoveAll(maki.RosterDir())
}

// uninstallMakiManifest removes the manifest only while it is still the one board wrote.
// Anything else in that file is the user's policy for their own Lua, and deleting it
// would take permissions away from plugins that have nothing to do with board.
func uninstallMakiManifest(dir string) error {
	path := filepath.Join(dir, "plugin.toml")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !strings.Contains(string(b), makiManifestMarker) {
		fmt.Println("left your", path, "alone")
		return nil
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	fmt.Println("removed", path)
	return nil
}

// MakiInstalled reports the two halves of the install separately: board's Lua block, and
// whether there is a plugin.toml beside it at all. A block with no manifest is installed
// and inert, which is worth its own words (§9.32).
//
// Read-only, like Installed: the command that diagnoses these files must not be the one
// that changes them (§8). No init.lua is not an error — it is the fresh-install answer.
func MakiInstalled() (block, manifest bool, err error) {
	path := maki.InitPath()
	manifest = fileExists(filepath.Join(filepath.Dir(path), "plugin.toml"))
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, manifest, nil
		}
		return false, manifest, err
	}
	src := string(b)
	if _, _, err := findMakiBlock(src); err != nil {
		return false, manifest, err
	}
	return makiBlockPresent(src), manifest, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func makiBlockPresent(src string) bool { return strings.Contains(src, makiBegin) }

// withMakiBlock puts the block at the top of src. The caller has already established
// that it is not there; what this refuses is a file whose markers do not make sense.
func withMakiBlock(src string) (string, error) {
	if _, _, err := findMakiBlock(src); err != nil {
		return "", err
	}
	if src == "" {
		return makiBlock, nil
	}
	return makiBlock + "\n" + src, nil
}

// withoutMakiBlock removes the block and reports whether there was one. The trailing
// blank line withMakiBlock added goes with it, so an install and an uninstall are
// inverses down to the byte and a cycle cannot accrete whitespace.
func withoutMakiBlock(src string) (string, bool, error) {
	start, end, err := findMakiBlock(src)
	if err != nil || start < 0 {
		return src, false, err
	}
	rest := src[end:]
	rest = strings.TrimPrefix(rest, "\n")
	return src[:start] + rest, true, nil
}

// findMakiBlock locates the block: the byte it starts at, and the byte just past its end
// marker's newline. (-1, -1) when there is none.
//
// A begin without an end, an end without a begin, or the two the wrong way round are all
// refusals rather than repairs. board cannot know where a half-deleted block ended, and
// the wrong guess deletes the user's Lua — the same line install-hooks holds on a
// settings.json it cannot parse (§8).
func findMakiBlock(src string) (int, int, error) {
	start, endMark := strings.Index(src, makiBegin), strings.Index(src, makiEnd)
	switch {
	case start < 0 && endMark < 0:
		return -1, -1, nil
	case start < 0:
		return 0, 0, fmt.Errorf("refusing to rewrite %s: it has board's closing marker and no opening one",
			maki.InitPath())
	case endMark < 0:
		return 0, 0, fmt.Errorf("refusing to rewrite %s: board's block is not closed by %q",
			maki.InitPath(), makiEnd)
	case endMark < start:
		return 0, 0, fmt.Errorf("refusing to rewrite %s: board's markers are out of order",
			maki.InitPath())
	}
	end := endMark + len(makiEnd)
	if i := strings.IndexByte(src[end:], '\n'); i >= 0 {
		end += i + 1
	} else {
		end = len(src)
	}
	return start, end, nil
}

// readInit is readSettings for a file board cannot parse: the text, the bytes as they
// stood, and the mode, so a writer can put both back. A missing file is the fresh answer.
func readInit(path string) (string, []byte, fs.FileMode, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, defaultInitMode, nil
		}
		return "", nil, 0, err
	}
	mode := defaultInitMode
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return string(b), b, mode, nil
}

// writeInit renames a finished file over the target, for the reason writeSettings does:
// the rename is the only step a reader can see and it is atomic. maki reads this file at
// startup, so a truncate-then-write window is a maki that boots without its config.
func writeInit(path, body string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".init.lua-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
