package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func seedRemindFixture(t *testing.T) {
	t.Helper()
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(-2 * time.Hour).Format(core.TimestampFormat)
	if err := s.Create(&core.Task{Title: "call the bank", Project: "inbox", Status: "todo", Kind: core.KindTask,
		Reminder: stamp, Created: stamp, Updated: stamp}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
}

func runRemind(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRemindCmd()
	cmd.SetArgs(args)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	return buf.String(), err
}

func TestRemindSendsOnceThenQuiets(t *testing.T) {
	seedRemindFixture(t)
	out, err := runRemind(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sent 1 reminder") {
		t.Fatalf("first run = %q, want 'sent 1 reminder'", out)
	}
	out, err = runRemind(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "nothing to remind") {
		t.Fatalf("second run = %q, want 'nothing to remind'", out)
	}
}

func TestRemindDryRunSendsNothing(t *testing.T) {
	seedRemindFixture(t)
	out, err := runRemind(t, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "would send 1 reminder") {
		t.Fatalf("dry-run = %q, want 'would send 1 reminder'", out)
	}
	// Dry run must not consume the one-shot: a real run still sends.
	out, err = runRemind(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sent 1 reminder") {
		t.Fatalf("after --dry-run, real run = %q, want 'sent 1 reminder'", out)
	}
}
