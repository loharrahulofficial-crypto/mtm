package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSaveUIKeysMergesAndPreserves(t *testing.T) {
	fixedXDG(t)
	p := Path()
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := "# mtm config — do not lose this line\ndata_repo: /data/repo\ndefault_project: college/dbms\nsync:\n  auto: false\n  debounce_seconds: 9\nui:\n  ambient_render: sextant\n  icons: unicode\n  future_key: keepme\n"
	if err := os.WriteFile(p, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	err := SaveUIKeys("", map[string]interface{}{
		"scene":           "night",
		"ambient_render":  "braille",
		"ambient_sidebar": false,
		"ambient_fps":     30,
		"splash_seconds":  2.5,
		"rain_lightning":  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := readFile(t, p)
	for _, want := range []string{
		"# mtm config — do not lose this line", // comment preserved
		"data_repo: /data/repo",                // other sections preserved
		"default_project: college/dbms",
		"debounce_seconds: 9",
		"ambient_render: braille", // managed key updated
		"icons: unicode",          // unrelated ui key preserved
		"future_key: keepme",      // unknown ui key preserved
		"scene: night",            // new managed keys added
		"ambient_fps: 30",
		"rain_lightning: true",
		"splash_seconds: 2.5",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("saved file missing %q\n---\n%s", want, got)
		}
	}
	// backup keeps the pre-save content
	if bak, err := os.ReadFile(p + ".bak"); err == nil {
		if string(bak) != original {
			t.Error(".bak does not hold the pre-save file")
		}
	} else {
		t.Error("no .bak written")
	}
	// reload through Load(): the saved values stick
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Scene != "night" || cfg.UI.AmbientRender != "braille" || cfg.UI.AmbientSidebar || cfg.UI.AmbientFPS != 30 || cfg.UI.SplashSeconds != 2.5 || !cfg.UI.RainLightning {
		t.Errorf("saved ui keys did not round-trip: %+v", cfg.UI)
	}
	if cfg.UI.Icons != "unicode" {
		t.Error("unrelated ui key lost through the merge")
	}
	if cfg.DefaultProject != "college/dbms" || cfg.Sync.DebounceSeconds != 9 {
		t.Error("non-ui sections clobbered")
	}
}

func TestSaveUIKeysCreatesMissingFile(t *testing.T) {
	fixedXDG(t)
	p := Path()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Skip("config file exists unexpectedly")
	}
	if err := SaveUIKeys("", map[string]interface{}{"scene": "koi"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "scene: koi") {
		t.Errorf("created file missing ui key:\n%s", got)
	}
	if cfg, err := Load(); err != nil {
		t.Fatal(err)
	} else if cfg.UI.Scene != "koi" {
		t.Errorf("scene = %q, want koi", cfg.UI.Scene)
	}
}

func TestSaveUIKeysRejectsBrokenYAML(t *testing.T) {
	fixedXDG(t)
	p := Path()
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := "ui: [unclosed\n"
	if err := os.WriteFile(p, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	err := SaveUIKeys("", map[string]interface{}{"scene": "night"})
	if err == nil {
		t.Fatal("expected error for broken YAML")
	}
	if got := readFile(t, p); got != broken {
		t.Error("config file was modified despite the error")
	}
}

func TestSaveUIKeysOverwritesManagedKeys(t *testing.T) {
	fixedXDG(t)
	p := Path()
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := "ui:\n  scene: rain\n  rain_lightning: false\n"
	if err := os.WriteFile(p, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveUIKeys("", map[string]interface{}{"scene": "snow", "rain_lightning": true}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, p)
	if !strings.Contains(got, "scene: snow") || !strings.Contains(got, "rain_lightning: true") {
		t.Errorf("managed keys not overwritten:\n%s", got)
	}
}
