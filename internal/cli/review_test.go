package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// seedReviewFixture creates one done-this-week task, one overdue task and
// one upcoming task inside the sandbox repo.
func seedReviewFixture(t *testing.T) string {
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
	defer func() { _ = s.Close() }()
	now := time.Now()
	stamp := now.Format(core.TimestampFormat)
	mk := func(task *core.Task) {
		t.Helper()
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk(&core.Task{Title: "done this week", Project: "inbox", Status: "done", Kind: core.KindTask,
		Completed: stamp, Created: stamp, Updated: stamp})
	mk(&core.Task{Title: "overdue thing", Project: "inbox", Status: "todo", Kind: core.KindTask,
		Due: now.AddDate(0, 0, -3).Format("2006-01-02"), Created: stamp, Updated: stamp})
	mk(&core.Task{Title: "upcoming thing", Project: "inbox", Status: "todo", Kind: core.KindTask,
		Due: now.AddDate(0, 0, 2).Format("2006-01-02"), Created: stamp, Updated: stamp})
	return stamp
}

func runReview(t *testing.T, input string) (string, error) {
	t.Helper()
	cmd := newReviewCmd()
	cmd.SetArgs(nil)
	cmd.SetIn(strings.NewReader(input))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	return buf.String(), err
}

func TestReviewSectionsPrint(t *testing.T) {
	seedReviewFixture(t)
	out, err := runReview(t, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"weekly review",
		"finished this week (1)",
		"done this week",
		"slipped — overdue (1)",
		"overdue thing",
		"upcoming thing",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("review output missing %q:\n%s", want, out)
		}
	}
}

func TestReviewRescheduleAppliesWithKeypress(t *testing.T) {
	seedReviewFixture(t)
	// r for reschedule, a date, y not needed; then q to stop.
	out, err := runReview(t, "r\n2099-01-02\nq\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "due → 2099-01-02") {
		t.Fatalf("reschedule did not stick:\n%s", out)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	found := false
	for _, task := range s.Tasks() {
		if task.Title == "overdue thing" {
			found = true
			if task.Due != "2099-01-02" {
				t.Fatalf("Due = %q, want 2099-01-02", task.Due)
			}
		}
	}
	if !found {
		t.Fatal("task vanished")
	}
}

func TestReviewArchiveAppliesWithKeypress(t *testing.T) {
	seedReviewFixture(t)
	out, err := runReview(t, "a\ny\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "archived →") {
		t.Fatalf("archive did not happen:\n%s", out)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	for _, task := range s.Tasks() {
		if task.Title == "overdue thing" {
			t.Fatal("archived task still in index")
		}
	}
}

func TestReviewEmptyInputKeepsEverything(t *testing.T) {
	seedReviewFixture(t)
	out, err := runReview(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1 kept") {
		t.Fatalf("empty input should keep the overdue item:\n%s", out)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	for _, task := range s.Tasks() {
		if task.Title == "overdue thing" && task.Due == "2099-01-02" {
			t.Fatal("empty input mutated the due date")
		}
	}
}

func TestStatsOutput(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	cfg, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stamp := now.Format(core.TimestampFormat)
	for i := 0; i < 3; i++ {
		if err := s.Create(&core.Task{Title: "done", Project: "inbox", Status: "done", Kind: core.KindTask,
			Completed: stamp, Created: stamp, Updated: stamp}); err != nil {
			t.Fatal(err)
		}
	}
	_ = cfg
	_ = s.Close()

	cmd := newStatsCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"week of", "completed   3", "streak", "by course", "per day"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats output missing %q:\n%s", want, out)
		}
	}
}

func TestStatsMutuallyExclusiveFlags(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	cmd := newStatsCmd()
	cmd.SetArgs([]string{"--week", "--month"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("--week --month accepted, want an error")
	}
}

func TestStatsSinceFlag(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	cmd := newStatsCmd()
	cmd.SetArgs([]string{"--since", "2026-11-01"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "since 1 Nov 2026") {
		t.Fatalf("--since label missing:\n%s", buf.String())
	}
	cmd = newStatsCmd()
	cmd.SetArgs([]string{"--since", "nonsense"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("--since nonsense accepted")
	}
}
