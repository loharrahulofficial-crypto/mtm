package paths

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
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
	for _, v := range []string{"MTM_SANDBOX", "MTM_DATA_DIR", "MTM_CONFIG_DIR", "MTM_STATE_DIR", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		_ = os.Unsetenv(v)
	}
	t.Setenv("HOME", home)
}

// applySandboxEnv mirrors contrib/sandbox-env.sh: a throwaway HOME with
// every XDG root inside it and MTM_SANDBOX=1 — the same guard contract the
// shell launchers get. Keep in sync with contrib/sandbox-env.sh.
func applySandboxEnv(t *testing.T) string {
	t.Helper()
	sandbox := t.TempDir()
	home := filepath.Join(sandbox, "home")
	for _, v := range []string{"MTM_DATA_DIR", "MTM_CONFIG_DIR", "MTM_STATE_DIR"} {
		_ = os.Unsetenv(v)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("MTM_SANDBOX", "1")
	return sandbox
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
	stateDir := filepath.Join(tmp, "elsewhere", "state")
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "xdg-data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "xdg-state"))
	t.Setenv("MTM_DATA_DIR", dataDir)
	t.Setenv("MTM_CONFIG_DIR", configDir)
	t.Setenv("MTM_STATE_DIR", stateDir)
	d := Resolve()
	if d.Data != dataDir {
		t.Errorf("MTM_DATA_DIR ignored: got %q", d.Data)
	}
	if d.Config != configDir {
		t.Errorf("MTM_CONFIG_DIR ignored: got %q", d.Config)
	}
	if d.State != stateDir {
		t.Errorf("MTM_STATE_DIR ignored: got %q", d.State)
	}
	if got := ConfigPath(); got != filepath.Join(configDir, "config.yaml") {
		t.Errorf("ConfigPath = %q", got)
	}
	if got := StateFile("timer.json"); got != filepath.Join(stateDir, "timer.json") {
		t.Errorf("StateFile = %q", got)
	}
}

func TestEnvOverrideIntoRealRootPanics(t *testing.T) {
	roots := RealRoots()
	cases := map[string]string{"MTM_DATA_DIR": roots[0], "MTM_CONFIG_DIR": roots[1], "MTM_STATE_DIR": roots[3]}
	for env, root := range cases {
		t.Run(env, func(t *testing.T) {
			clearEnv(t, t.TempDir())
			t.Setenv("MTM_SANDBOX", "1")
			t.Setenv(env, root)
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("%s pointing at the real root did not panic", env)
				}
			}()
			_ = Resolve()
		})
	}
}

// TestEveryPathFunctionUnderSandbox applies the shared sandbox env (the
// mirror of contrib/sandbox-env.sh) and asserts every exported path
// function in this package resolves under it. The set of functions is
// enumerated from the source with go/ast, so a new exported func returning
// a path fails the test until it is covered here.
func TestEveryPathFunctionUnderSandbox(t *testing.T) {
	sandbox := applySandboxEnv(t)

	covered := map[string]func() string{
		"Data":       Data,
		"ConfigDir":  ConfigDir,
		"ConfigPath": ConfigPath,
		"CacheDir":   CacheDir,
		"CachePath":  CachePath,
		"StateDir":   StateDir,
		"StateFile":  func() string { return StateFile("probe-state-file") },
	}
	for name, call := range covered {
		if p := call(); !underRoot(p, sandbox) {
			t.Errorf("%s() = %q, want a path under the sandbox %q", name, p, sandbox)
		}
	}

	// Dirs carries one string field per root; every field must resolve
	// under the sandbox too.
	v := reflect.ValueOf(Resolve())
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.String {
			t.Fatalf("Dirs.%s is %s, want string — extend this test", f.Name, f.Type)
		}
		if p := v.Field(i).String(); !underRoot(p, sandbox) {
			t.Errorf("Dirs.%s = %q, want a path under the sandbox %q", f.Name, p, sandbox)
		}
	}

	// Enumerate every exported func in the package that returns a string
	// (i.e. hands back a path) and demand coverage above.
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing package source: %v", err)
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || !fn.Name.IsExported() {
					continue
				}
				if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
					continue
				}
				ident, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
				if !ok || ident.Name != "string" {
					continue
				}
				if _, ok := covered[fn.Name.Name]; !ok {
					t.Errorf("exported path function %s is not covered by this test — add it to `covered`", fn.Name.Name)
				}
			}
		}
	}
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
