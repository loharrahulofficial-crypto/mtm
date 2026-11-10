package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func runExport(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newExportCmd()
	cmd.SetArgs(args)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	return buf.String(), err
}

func TestExportMarkdownToStdout(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Format(core.TimestampFormat)
	if err := s.Create(&core.Task{Title: "buy milk", Project: "inbox", Status: "todo", Kind: core.KindTask,
		Created: stamp, Updated: stamp}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	out, err := runExport(t)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# mtm export", "## inbox", "buy milk", "1 tasks"} {
		if !strings.Contains(out, want) {
			t.Errorf("export output missing %q:\n%s", want, out)
		}
	}
}

func TestExportJSONAndOutFile(t *testing.T) {
	gitEnv(t)
	dataDir := sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	file := dataDir + "/export.json"
	out, err := runExport(t, "--format", "json", "--out", file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wrote") {
		t.Fatalf("--out did not report writing: %q", out)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schema_version": 2`) {
		t.Fatalf("out file missing schema_version:\n%s", data)
	}
	// File mode is 0600 (owner-only — export carries your data).
	st, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("export file mode = %o, want 600", st.Mode().Perm())
	}
}

func TestExportBadFormatRefused(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	if _, err := runExport(t, "--format", "yaml"); err == nil {
		t.Fatal("--format yaml accepted")
	}
}

func TestExportFiltersCli(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Format(core.TimestampFormat)
	for _, task := range []*core.Task{
		{Title: "inbox item", Project: "inbox", Status: "todo", Kind: core.KindTask, Created: stamp, Updated: stamp},
	} {
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	_ = s.Close()
	out, err := runExport(t, "--format", "json", "--project", "inbox")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "inbox item") {
		t.Fatalf("--project filter lost the item:\n%s", out)
	}
	out, err = runExport(t, "--format", "json", "--since", "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "inbox item") {
		t.Fatalf("--since in the past lost the item:\n%s", out)
	}
}
