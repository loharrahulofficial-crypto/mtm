package actions

// Reminders (Phase 5, item 4): what to notify and when. The timer-adjacent
// state (what was already sent) lives in the state dir, never the data
// repo.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

const (
	remindersStateName = "reminders.json"
	remindersMaxAge    = 30 * 24 * time.Hour
)

// ReminderCandidate is one notification to (maybe) send.
type ReminderCandidate struct {
	Task  *core.Task
	Title string
	Body  string
	Key   string // dedup key: task id + occurrence stamp
	Kind  string // "reminder" | "due_soon"
}

// CollectReminderCandidates returns everything due for notification right
// now: tasks whose reminder stamp has passed, and (when configured) tasks
// due within DueSoonMinutes. Done tasks are skipped.
func CollectReminderCandidates(s *store.Store, cfg config.Config, now time.Time) []ReminderCandidate {
	loc := now.Location()
	isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
	var out []ReminderCandidate
	for _, t := range s.Tasks() {
		if isDone(t.Project, t.Status) {
			continue
		}
		if stamp := strings.TrimSpace(t.Reminder); stamp != "" {
			if ts, ok := dates.Parse(stamp, loc); ok && !ts.After(now) {
				out = append(out, ReminderCandidate{
					Task:  t,
					Title: t.Title,
					Body:  "reminder",
					Key:   t.ID + "|reminder|" + stamp,
					Kind:  "reminder",
				})
			}
		}
		if cfg.Reminders.DueSoonMinutes > 0 {
			if due, ok := parseDueLoose(t.Due, loc); ok {
				until := time.Duration(cfg.Reminders.DueSoonMinutes) * time.Minute
				if !due.Before(now) && !due.After(now.Add(until)) {
					out = append(out, ReminderCandidate{
						Task:  t,
						Title: t.Title,
						Body:  fmt.Sprintf("due in %s", formatDurationShort(due.Sub(now))),
						Key:   t.ID + "|duesoon|" + t.Due,
						Kind:  "due_soon",
					})
				}
			}
		}
	}
	return out
}

func parseDueLoose(s string, loc *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.ParseInLocation(core.TimestampFormat, s, loc); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, loc); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func formatDurationShort(d time.Duration) string {
	d = d.Round(time.Minute)
	switch {
	case d < time.Minute:
		return "under a minute"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	default:
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
}

// ReminderResult summarizes a reminder run.
type ReminderResult struct {
	Sent    []ReminderCandidate
	Skipped int // already sent before (deduped)
	Quiet   bool
}

// reminderState is the on-disk dedup table (state dir, JSON).
type reminderState struct {
	Sent map[string]string `json:"sent"` // dedup key -> sent-at (local stamp)
}

func loadReminderState() reminderState {
	state := reminderState{Sent: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(paths.StateDir(), remindersStateName))
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		// Corrupt state: start fresh rather than notify-spam; rename aside.
		_ = os.Rename(filepath.Join(paths.StateDir(), remindersStateName), filepath.Join(paths.StateDir(), remindersStateName+".corrupt"))
		return reminderState{Sent: map[string]string{}}
	}
	if state.Sent == nil {
		state.Sent = map[string]string{}
	}
	return state
}

func saveReminderState(state reminderState) error {
	// Prune entries older than 30 days so the file never grows without bound.
	cutoff := time.Now().Add(-remindersMaxAge).Format(core.TimestampFormat)
	for key, at := range state.Sent {
		if at < cutoff {
			delete(state.Sent, key)
		}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	dir := paths.StateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return store.WriteFileAtomic(filepath.Join(dir, remindersStateName), data, 0o600)
}

// InQuietHours reports whether the quiet window covers now.
func InQuietHours(now time.Time, quiet string) bool {
	w, err := config.ParseQuietHours(quiet)
	if err != nil {
		return false
	}
	return w.InQuietHours(now)
}

// RunReminders evaluates candidates, sends what hasn't been sent yet, and
// records the sends. It never writes to the data repo. send may be nil in
// tests — pass Notify for the real path.
func RunReminders(s *store.Store, cfg config.Config, now time.Time, send func(title, body string) error) (*ReminderResult, error) {
	return runReminders(s, cfg, now, send, true)
}

// RunRemindersDry evaluates without sending or recording anything.
func RunRemindersDry(s *store.Store, cfg config.Config, now time.Time, send func(title, body string) error) (*ReminderResult, error) {
	return runReminders(s, cfg, now, send, false)
}

func runReminders(s *store.Store, cfg config.Config, now time.Time, send func(title, body string) error, commit bool) (*ReminderResult, error) {
	res := &ReminderResult{}
	if InQuietHours(now, cfg.Reminders.QuietHours) {
		res.Quiet = true
		return res, nil
	}
	candidates := CollectReminderCandidates(s, cfg, now)
	state := loadReminderState()
	for _, c := range candidates {
		if _, sent := state.Sent[c.Key]; sent {
			res.Skipped++
			continue
		}
		if send != nil {
			if err := send(c.Title, c.Body); err != nil {
				return res, fmt.Errorf("notify: %w", err)
			}
		}
		res.Sent = append(res.Sent, c)
		if commit {
			state.Sent[c.Key] = now.Format(core.TimestampFormat)
		}
	}
	if commit && len(res.Sent) > 0 {
		if err := saveReminderState(state); err != nil {
			return res, fmt.Errorf("record sent reminders: %w", err)
		}
	}
	return res, nil
}
