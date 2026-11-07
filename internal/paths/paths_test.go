package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func realHome(t *testing.T) string {
	t.Helper()
	home, err := realHomeDir()
	if err != nil || home == "" {
		t.Skip("passwd home unavailable on this machine")
	}
	return home
}

func clearEnv(t *testing.T, home string) {
	t.Helper()
	for _, v := range []string{"MTM_SANDBOX", "MTM_DATA_DIR", "MTM_CONFIG_DIR", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		_ = os.Unsetenv(v)
	}
	t.Setenv("HOME", home)
}

func TestGuardFiresOnRealRoots(t *testing.T) {
	roots := RealRoots()
	if len(roots) != 4 {
		t.Fatalf("want 4 real roots, got %v", roots)
	}
	for _, root := range roots {
		t.Run(filepath.Base(filepath.Dir(root))+"_"+filepath.Base(root), func(t *testing.T) {
			clearEnv(t, realHome(t))
			t.Setenv("MTM_SANDBOX", "1")
			for _, target := range []string{root, filepath.Join(root, "projects", "college"), filepath.Join(root, "index.json")} {
				func() {
					defer func() {
						r := recover()
						if r == nil {
							t.Fatalf("Guard(%q) did not panic", target)
						}
						msg := fmt.Sprint(r)
						if !strings.Contains(msg, "refusing") || !strings.Contains(msg, filepath.Base(root)) {
							t.Fatalf("panic message unclear: %v", msg)
						}
					}()
					Guard(target)
				}()
			}
		})
	}
}

func TestResolvePanicsWhenNotRedirected(t *testing.T) {
	clearEnv(t, realHome(t))
	t.Setenv("MTM_SANDBOX", "1")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Resolve did not panic with HOME/XDG pointing at the real home")
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, "refusing") {
			t.Fatalf("panic message unclear: %v", msg)
		}
	}()
	_ = Resolve()
}

func TestTestingBinaryIsDetected(t *testing.T) {
	if !testing.Testing() {
		t.Fatal("testing.Testing() reported false inside a test binary")
	}
	if !Sandboxed() {
		t.Fatal("Sandboxed() false inside a test binary — the guard would not fire")
	}
}

func TestGuardQuietOutsideSandboxAndTempRoots(t *testing.T) {
	// Still sandboxed via testing.Testing; the temp dir is under /tmp, so
	// the guard stays quiet.
	clearEnv(t, realHome(t))
	Guard(filepath.Join(t.TempDir(), "under-tmp"))

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	for _, v := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		_ = os.Unsetenv(v)
	}
	d := Resolve() // must not panic: everything resolves under the temp HOME
	for name, got := range map[string]string{
		"data": d.Data, "config": d.Config, "cache": d.Cache, "state": d.State,
	} {
		if !strings.HasPrefix(got, tmp) {
			t.Errorf("%s dir %q not under redirected HOME %q", name, got, tmp)
		}
	}
}

func TestEnvOverridesWin(t *testing.T) {
	clearEnv(t, t.TempDir())
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "elsewhere", "data")
	configDir := filepath.Join(tmp, "elsewhere", "config")
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "xdg-data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))
	t.Setenv("MTM_DATA_DIR", dataDir)
	t.Setenv("MTM_CONFIG_DIR", configDir)
	d := Resolve()
	if d.Data != dataDir {
		t.Errorf("MTM_DATA_DIR ignored: got %q", d.Data)
	}
	if d.Config != configDir {
		t.Errorf("MTM_CONFIG_DIR ignored: got %q", d.Config)
	}
	if got := ConfigPath(); got != filepath.Join(configDir, "config.yaml") {
		t.Errorf("ConfigPath = %q", got)
	}
}

func TestEnvOverrideIntoRealRootPanics(t *testing.T) {
	clearEnv(t, t.TempDir())
	t.Setenv("MTM_SANDBOX", "1")
	t.Setenv("MTM_DATA_DIR", RealRoots()[0])
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MTM_DATA_DIR pointing at the real data root did not panic")
		}
	}()
	_ = Resolve()
}

func TestUnderRootEdgeCases(t *testing.T) {
	root := filepath.Join(realHome(t), ".local", "share", "mtm")
	cases := []struct {
		path string
		want bool
	}{
		{root, true},
		{root + string(os.PathSeparator), true},
		{filepath.Join(root, "projects", "x"), true},
		{root + "2", false}, // sibling with shared prefix
		{filepath.Join(root, "..", "escaped"), false},
		{filepath.Join(t.TempDir(), "mtm"), false},
	}
	for _, c := range cases {
		if got := underRoot(c.path, root); got != c.want {
			t.Errorf("underRoot(%q, %q) = %v, want %v", c.path, root, got, c.want)
		}
	}
}
