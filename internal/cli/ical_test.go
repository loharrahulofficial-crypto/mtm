package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func runIcal(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newIcalCmd()
	cmd.SetArgs(args)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	return buf.String(), err
}

func seedIcalFixture(t *testing.T) string {
	t.Helper()
	gitEnv(t)
	dataHome := sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	_, s, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Format(core.TimestampFormat)
	now := time.Now()
	for _, task := range []*core.Task{
		{Title: "buy milk", Project: "inbox", Status: "todo", Kind: core.KindTask,
			Due: now.AddDate(0, 0, 1).Format("2006-01-02"), Created: stamp, Updated: stamp},
		{Title: "DBMS midterm", Project: "inbox", Status: "todo", Kind: core.KindExam,
			Due: now.AddDate(0, 0, 6).Format("2006-01-02") + "T09:00", Created: stamp, Updated: stamp},
		{Title: "done item", Project: "inbox", Status: "done", Kind: core.KindTask,
			Due: now.AddDate(0, 0, 2).Format("2006-01-02"), Created: stamp, Updated: stamp, Completed: stamp},
	} {
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	_ = s.Close()
	return dataHome
}

func TestIcalToStdout(t *testing.T) {
	seedIcalFixture(t)
	out, err := runIcal(t)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"BEGIN:VCALENDAR", "END:VCALENDAR\r\n", "buy milk", "DBMS midterm", "UID:mtm-"} {
		if !strings.Contains(out, want) {
			t.Errorf("ical output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "done item") {
		t.Error("done item leaked into the feed (feed is for what's ahead)")
	}
}

func TestIcalOutFile(t *testing.T) {
	dataHome := seedIcalFixture(t)
	file := dataHome + "/mtm/feed.ics"
	out, err := runIcal(t, "--out", file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wrote") {
		t.Fatalf("--out did not report writing: %q", out)
	}
	st, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("feed file mode = %o, want 600", st.Mode().Perm())
	}
}

func TestIcalProjectFilter(t *testing.T) {
	seedIcalFixture(t)
	// All seeded tasks are in inbox: filtering to nosuchproject empties
	// the feed; filtering to inbox keeps it.
	out, err := runIcal(t, "--project", "nosuchproject")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "buy milk") {
		t.Fatalf("--project nosuchproject kept tasks:\n%s", out)
	}
	out, err = runIcal(t, "--project", "inbox")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "buy milk") {
		t.Fatalf("--project inbox lost the task:\n%s", out)
	}
}

func TestIcalIncludeSubset(t *testing.T) {
	seedIcalFixture(t)
	out, err := runIcal(t, "--include", "tasks")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "DBMS midterm") {
		t.Errorf("--include tasks leaked the exam:\n%s", out)
	}
	out, err = runIcal(t, "--include", "exams")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "buy milk") {
		t.Errorf("--include exams leaked the task:\n%s", out)
	}
	if !strings.Contains(out, "DBMS midterm") {
		t.Errorf("--include exams lost the exam:\n%s", out)
	}
}

func TestIcalBadIncludeAndBind(t *testing.T) {
	seedIcalFixture(t)
	if _, err := runIcal(t, "--include", "nonsense"); err == nil {
		t.Fatal("--include nonsense accepted")
	}
	if _, err := runIcal(t, "--serve", "--bind", "not-a-bind"); err == nil {
		t.Fatal("--bind not-a-bind accepted")
	}
	if _, err := runIcal(t, "--serve", "--token", "bad token!"); err == nil {
		t.Fatal("--token with spaces accepted")
	}
}
