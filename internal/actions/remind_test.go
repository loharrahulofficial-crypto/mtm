package actions

import (
	"fmt"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

func remindFixture(t *testing.T) (*store.Store, time.Time) {
	t.Helper()
	// Full sandbox: redirect every root, or the paths guard panics.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("MTM_SANDBOX", "1")
	s := openActionStore(t, "college/dbms")
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	t.Cleanup(func() { _ = s.Close() })
	return s, now
}

func doneTask(t *testing.T, s *store.Store, title string, stamp string) {
	t.Helper()
	task := &core.Task{Title: title, Project: "college/dbms", Status: "done", Kind: core.KindTask,
		Completed: stamp, Created: stamp, Updated: stamp}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
}

func TestCollectReminderCandidates(t *testing.T) {
	s, now := remindFixture(t)
	past := now.Add(-2 * time.Hour).Format(core.TimestampFormat)
	future := now.Add(2 * time.Hour).Format(core.TimestampFormat)
	dueSoonTime := now.Add(30 * time.Minute).Format(core.TimestampFormat)
	mk := func(title, due, reminder string) {
		task := &core.Task{Title: title, Project: "college/dbms", Status: "todo", Kind: core.KindTask,
			Due: due, Reminder: reminder, Created: now.Format(core.TimestampFormat), Updated: now.Format(core.TimestampFormat)}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	mk("reminder passed", "", past)
	mk("reminder future", "", future)
	mk("due soon", dueSoonTime, "")
	mk("due far future", future, "")
	mk("no stamps", "", "")
	doneTask(t, s, "reminder passed but done", past)

	cfg := config.Config{Reminders: config.RemindersConfig{DueSoonMinutes: 60}}
	got := CollectReminderCandidates(s, cfg, now)
	kinds := map[string][]string{}
	for _, c := range got {
		kinds[c.Kind] = append(kinds[c.Kind], c.Task.Title)
	}
	if len(kinds["reminder"]) != 1 {
		t.Errorf("reminder candidates = %v, want [reminder passed]", kinds["reminder"])
	}
	if len(kinds["due_soon"]) != 1 {
		t.Errorf("due_soon candidates = %v, want [due soon]", kinds["due_soon"])
	}

	// Due-soon disabled: only explicit reminders fire.
	cfg.Reminders.DueSoonMinutes = 0
	got = CollectReminderCandidates(s, cfg, now)
	kinds = map[string][]string{}
	for _, c := range got {
		kinds[c.Kind] = append(kinds[c.Kind], c.Task.Title)
	}
	if len(kinds["due_soon"]) != 0 {
		t.Errorf("due_soon fired while disabled: %v", kinds["due_soon"])
	}
}

func TestRunRemindersSendsOnceThenDedupes(t *testing.T) {
	s, now := remindFixture(t)
	past := now.Add(-2 * time.Hour).Format(core.TimestampFormat)
	task := &core.Task{Title: "call the bank", Project: "college/dbms", Status: "todo", Kind: core.KindTask,
		Reminder: past, Created: now.Format(core.TimestampFormat), Updated: now.Format(core.TimestampFormat)}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}

	var sent []string
	send := func(title, body string) error {
		sent = append(sent, title+"|"+body)
		return nil
	}
	cfg := config.Config{}
	rep, err := RunReminders(s, cfg, now, send)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Sent) != 1 || len(sent) != 1 {
		t.Fatalf("first run sent %d (captured %d), want 1", len(rep.Sent), len(sent))
	}
	rep, err = RunReminders(s, cfg, now, send)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Sent) != 0 || rep.Skipped != 1 {
		t.Errorf("second run: sent %d skipped %d, want sent 0 skipped 1", len(rep.Sent), rep.Skipped)
	}

	// A different occurrence (new reminder stamp) fires again.
	task.Reminder = now.Add(-1 * time.Hour).Format(core.TimestampFormat)
	if err := s.Update(task); err != nil {
		t.Fatal(err)
	}
	rep, err = RunReminders(s, cfg, now, send)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Sent) != 1 {
		t.Errorf("edited reminder sent %d, want 1", len(rep.Sent))
	}
}

func TestRunRemindersQuietHours(t *testing.T) {
	s, now := remindFixture(t)
	past := now.Add(-2 * time.Hour).Format(core.TimestampFormat)
	task := &core.Task{Title: "whisper", Project: "college/dbms", Status: "todo", Kind: core.KindTask,
		Reminder: past, Created: now.Format(core.TimestampFormat), Updated: now.Format(core.TimestampFormat)}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	var sent int
	send := func(title, body string) error { sent++; return nil }
	cfg := config.Config{Reminders: config.RemindersConfig{QuietHours: "00:00-23:59"}} // always quiet
	rep, err := RunReminders(s, cfg, now, send)
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 || !rep.Quiet {
		t.Fatalf("quiet hours: sent %d quiet=%v, want 0/true", sent, rep.Quiet)
	}
	// Nothing was recorded as sent — the reminder fires when quiet ends.
	cfg.Reminders.QuietHours = ""
	rep, err = RunReminders(s, cfg, now, send)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Sent) != 1 {
		t.Errorf("after quiet ended: sent %d, want 1", len(rep.Sent))
	}
}

func TestRunRemindersSendFailureNotRecorded(t *testing.T) {
	s, now := remindFixture(t)
	past := now.Add(-2 * time.Hour).Format(core.TimestampFormat)
	task := &core.Task{Title: "fragile", Project: "college/dbms", Status: "todo", Kind: core.KindTask,
		Reminder: past, Created: now.Format(core.TimestampFormat), Updated: now.Format(core.TimestampFormat)}
	if err := s.Create(task); err != nil {
		t.Fatal(err)
	}
	calls := 0
	send := func(title, body string) error { calls++; return fmt.Errorf("no notification daemon") }
	cfg := config.Config{}
	if _, err := RunReminders(s, cfg, now, send); err == nil {
		t.Fatal("send failure did not surface")
	}
	cfg.Reminders.QuietHours = ""
	rep, err := RunReminders(s, cfg, now, func(title, body string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Sent) != 1 {
		t.Errorf("retry after failure: sent %d, want 1 (failure must not mark as sent)", len(rep.Sent))
	}
	_ = calls
}

func TestReminderStatePruned(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("MTM_SANDBOX", "1")
	old := time.Now().AddDate(0, 0, -45).Format(core.TimestampFormat)
	recent := time.Now().AddDate(0, 0, -2).Format(core.TimestampFormat)
	st := reminderState{Sent: map[string]string{
		"old-key":    old,
		"recent-key": recent,
	}}
	if err := saveReminderState(st); err != nil {
		t.Fatal(err)
	}
	loaded := loadReminderState()
	if _, ok := loaded.Sent["old-key"]; ok {
		t.Error("old-key was not pruned (>30 days)")
	}
	if _, ok := loaded.Sent["recent-key"]; !ok {
		t.Error("recent-key was wrongly pruned")
	}
}
