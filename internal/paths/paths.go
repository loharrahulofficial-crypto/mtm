// Package paths is the single resolver for every directory mtm touches:
// the data repo, the config dir, the cache dir and the state dir.
//
// Safety rule: when this process is a test binary (testing.Testing) or
// MTM_SANDBOX=1 is set, any resolved path that equals or sits under the
// user's real mtm roots (derived from the passwd home, not $HOME, so a
// redirected HOME cannot hide the real location) is a bug — the resolver
// panics with a message naming the offending path. Tests and sandboxes
// must redirect HOME and the XDG roots (or MTM_DATA_DIR/MTM_CONFIG_DIR)
// to a temp dir; contrib/smoke.sh and the test helpers all do this.
//
// Resolution order (highest wins):
//
//	data:   MTM_DATA_DIR   > $XDG_DATA_HOME/mtm   > <passwd-or-HOME>/.local/share/mtm
//	config: MTM_CONFIG_DIR > $XDG_CONFIG_HOME/mtm  > <passwd-or-HOME>/.config/mtm
//	cache:                 $XDG_CACHE_HOME/mtm     > <passwd-or-HOME>/.cache/mtm
//	state:                 $XDG_STATE_HOME/mtm     > <passwd-or-HOME>/.local/state/mtm
package paths

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// Dirs holds the resolved mtm directories.
type Dirs struct {
	Data   string
	Config string
	Cache  string
	State  string
}

// Sandboxed reports whether this process must stay away from the real
// home-directory roots: a test binary, or MTM_SANDBOX=1.
func Sandboxed() bool {
	return testing.Testing() || os.Getenv("MTM_SANDBOX") == "1"
}

// realHomeDir returns the passwd-database home when available (so a
// redirected $HOME does not fool the guard). Falls back to $HOME.
func realHomeDir() (string, error) {
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return u.HomeDir, nil
	}
	if h := os.Getenv("HOME"); h != "" {
		return h, nil
	}
	return "", fmt.Errorf("cannot determine the passwd home directory")
}

// RealRoots lists the real mtm directories the sandbox guard refuses.
func RealRoots() []string {
	home, err := realHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "share", "mtm"),
		filepath.Join(home, ".config", "mtm"),
		filepath.Join(home, ".cache", "mtm"),
		filepath.Join(home, ".local", "state", "mtm"),
	}
}

func clean(p string) string {
	cleaned := filepath.Clean(p)
	if abs, err := filepath.Abs(cleaned); err == nil {
		return abs
	}
	return cleaned
}

// underRoot reports whether path equals root or lives beneath it.
func underRoot(path, root string) bool {
	p, r := clean(path), clean(root)
	if p == r {
		return true
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// Guard panics when running sandboxed/tested and dir equals or sits under
// one of the real mtm roots. Callers must route every resolved path
// through Guard; Resolve does this already.
func Guard(dir string) {
	if !Sandboxed() {
		return
	}
	for _, root := range RealRoots() {
		if underRoot(dir, root) {
			panic(fmt.Sprintf("mtm: refusing to touch the real data directory %s — this process is running under tests/MTM_SANDBOX but HOME/XDG (or MTM_DATA_DIR/MTM_CONFIG_DIR) still resolve there; redirect them to a temp dir (see contrib/smoke.sh)", clean(dir)))
		}
	}
}

// homeDir is the base for the fallback roots: the passwd home when it is
// readable, otherwise $HOME. In a correctly redirected test/sandbox the two
// agree on nothing real; when HOME lies outside /tmp the passwd home wins
// for the *guard* comparison but $HOME stays the resolution base.
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h, err := realHomeDir(); err == nil {
		return h
	}
	// Last resort: no usable home anywhere. Resolve relative to cwd; the
	// guard still rejects anything under the real roots above.
	return "."
}

func xdg(varName, fallback string) string {
	if v := os.Getenv(varName); v != "" {
		return filepath.Join(v, "mtm")
	}
	return filepath.Join(homeDir(), fallback)
}

// Resolve computes the four directories, applying env overrides and the
// sandbox guard to every one of them.
func Resolve() Dirs {
	d := Dirs{
		Data:   os.Getenv("MTM_DATA_DIR"),
		Config: os.Getenv("MTM_CONFIG_DIR"),
	}
	if d.Data == "" {
		d.Data = xdg("XDG_DATA_HOME", filepath.Join(".local", "share", "mtm"))
	}
	if d.Config == "" {
		d.Config = xdg("XDG_CONFIG_HOME", filepath.Join(".config", "mtm"))
	}
	d.Cache = xdg("XDG_CACHE_HOME", filepath.Join(".cache", "mtm"))
	d.State = xdg("XDG_STATE_HOME", filepath.Join(".local", "state", "mtm"))
	Guard(d.Data)
	Guard(d.Config)
	Guard(d.Cache)
	Guard(d.State)
	return d
}

// Data is the data-repo directory (MTM_DATA_DIR override honored).
func Data() string { return Resolve().Data }

// ConfigDir is the config directory (MTM_CONFIG_DIR override honored).
func ConfigDir() string { return Resolve().Config }

// ConfigPath is the config file path.
func ConfigPath() string { return filepath.Join(ConfigDir(), "config.yaml") }

// CacheDir is the index-cache directory.
func CacheDir() string { return Resolve().Cache }

// CachePath is the index cache file path (D-006: outside the data repo).
func CachePath() string { return filepath.Join(CacheDir(), "index.json") }

// StateDir is the runtime state directory (outside the data repo).
// Timer/pomodoro runtime state lives here; nothing in here is synced.
func StateDir() string { return Resolve().State }

// StateFile returns a file inside the state dir; the dir is not created.
func StateFile(name string) string { return filepath.Join(StateDir(), name) }
