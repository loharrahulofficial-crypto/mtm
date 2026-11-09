package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

func runWaybar(t *testing.T) (string, error) {
	t.Helper()
	cmd := newWaybarCmd()
	cmd.SetArgs(nil)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	return buf.String(), err
}

func TestWaybarPrintsValidJSON(t *testing.T) {
	seedRemindFixture(t) // seeds one todo with a reminder in the past
	out, err := runWaybar(t)
	if err != nil {
		t.Fatal(err)
	}
	var payload WaybarPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatalf("output is not JSON: %q (%v)", out, err)
	}
	if payload.Text == "" {
		t.Error("empty text")
	}
	if payload.Tooltip == "" {
		t.Error("empty tooltip")
	}
	switch payload.Class {
	case "normal", "overdue", "exam", "timer", "error":
	default:
		t.Errorf("class = %q, want one of normal|overdue|exam|timer|error", payload.Class)
	}
}

func TestWaybarIncludesTimerAndExamWhenPresent(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Format(core.TimestampFormat)
	now := time.Now()
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []*core.Task{
		{Title: "today thing", Project: "inbox", Status: "todo", Kind: core.KindTask,
			Due: now.Format("2006-01-02"), Created: stamp, Updated: stamp},
		{Title: "DBMS midterm", Project: "inbox", Status: "todo", Kind: core.KindExam,
			Due: now.AddDate(0, 0, 2).Format("2006-01-02") + "T09:00", Created: stamp, Updated: stamp},
	} {
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	// Start a timer on the exam task so the chip shows.
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	var examID string
	for _, task := range s.Tasks() {
		if task.Title == "DBMS midterm" {
			examID = task.ID
		}
	}
	if examID == "" {
		t.Fatal("exam task not found")
	}
	if _, err := actions.StartTimer(s, cfg, examID, core.TimerModeTimer); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	out, err := runWaybar(t)
	if err != nil {
		t.Fatal(err)
	}
	var payload WaybarPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.Text, "✦2d") {
		t.Errorf("text = %q, want the exam chip ✦2d", payload.Text)
	}
	if !strings.Contains(payload.Tooltip, "DBMS midterm") {
		t.Errorf("tooltip = %q, want the exam line", payload.Tooltip)
	}
	if payload.Class != "exam" {
		t.Errorf("class = %q, want exam (next exam within 3 days)", payload.Class)
	}
}

func TestWaybarOverdueClass(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(-72 * time.Hour).Format(core.TimestampFormat)
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(&core.Task{Title: "very late", Project: "inbox", Status: "todo", Kind: core.KindTask,
		Due: stamp[:len("2006-01-02")], Created: stamp, Updated: stamp}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	out, err := runWaybar(t)
	if err != nil {
		t.Fatal(err)
	}
	var payload WaybarPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Class != "overdue" {
		t.Errorf("class = %q, want overdue", payload.Class)
	}
}

func TestCompletionScriptsGenerate(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		// The generator names the script after the root it executes
		// under — exactly like production's `mtm completion <shell>`.
		root := &cobra.Command{Use: "mtm"}
		root.AddCommand(newCompletionCmd(root))
		root.SetArgs([]string{"completion", shell})
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		if err := root.Execute(); err != nil {
			t.Fatalf("completion %s: %v", shell, err)
		}
		if buf.Len() < 100 {
			t.Fatalf("completion %s: suspiciously short output (%d bytes)", shell, buf.Len())
		}
		if !strings.Contains(buf.String(), "mtm") {
			t.Fatalf("completion %s: output does not mention mtm", shell)
		}
	}
	root := &cobra.Command{Use: "mtm"}
	root.AddCommand(newCompletionCmd(root))
	root.SetArgs([]string{"completion", "tcsh"})
	if err := root.Execute(); err == nil {
		t.Fatal("completion tcsh accepted")
	}
}
