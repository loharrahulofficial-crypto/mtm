package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureCombined runs fn with stdout+stderr redirected and returns everything printed.
func captureCombined(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	fnErr := fn()
	_ = w.Close()
	os.Stdout, os.Stderr = origOut, origErr
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), fnErr
}

func TestSummaryIncludesIdeasAndMilestone(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	// Seed a milestone through the CLI.
	if err := runMilestoneCmd(t, "add", "inbox", "v0.1 cut", "--target", "2030-05-01"); err != nil {
		t.Fatal(err)
	}
	// Seed two ideas (one seed, one dropped) through the CLI.
	if err := runIdeaCmd(t, "first idea"); err != nil {
		t.Fatal(err)
	}
	if err := runIdeaCmd(t, "second idea"); err != nil {
		t.Fatal(err)
	}
	// Find the dropped one and drop it (so the count shows only open ones).
	out, err := captureStdout(t, func() error {
		cmd := newIdeasCmd()
		cmd.SetArgs([]string{"--status", "seed"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Fields(strings.Split(strings.TrimSpace(out), "\n")[1])[0]
	out2, err := captureCombined(t, func() error { return runIdeaCmd(t, "status", id, "dropped") })
	if err != nil {
		t.Fatal(err)
	}
	_ = out2
	out, err = captureCombined(t, func() error {
		cmd := newSummaryCmd()
		return cmd.Execute()
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1 ideas", "▸ v0.1 cut · in "} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	// The data path is local — the summary never shells out to a network.
}

func TestSummaryHidesEmptySections(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	// No ideas, no milestones, no tasks: only the bare counters.
	out, err := captureCombined(t, func() error {
		cmd := newSummaryCmd()
		return cmd.Execute()
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "ideas") {
		t.Errorf("empty store shows an ideas count:\n%s", out)
	}
	if strings.Contains(out, "▸") {
		t.Errorf("empty store shows a milestone line:\n%s", out)
	}
}
