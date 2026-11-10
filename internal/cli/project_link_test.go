package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// linkedTestRepo creates a temp git repo with one commit and returns it.
func linkedTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=mtm", "GIT_AUTHOR_EMAIL=mtm@example.invalid",
		"GIT_COMMITTER_NAME=mtm", "GIT_COMMITTER_EMAIL=mtm@example.invalid")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# linked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "initial commit")
	return repo
}

// fakeGH writes a stub gh whose gh auth status exits 0 and gh issue/pr list
// print fixed JSON payloads. Nothing touches the network.
func fakeGH(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	body := `#!/bin/sh
case "$*" in
  "auth status"*) exit 0;;
  "issue list"*) printf '%s' '[{"number":12,"title":"Sync bug","url":"https://github.com/x/mtm/issues/12"}]'; exit 0;;
  "pr list"*) printf '%s' '[{"number":18,"title":"board columns","url":"https://github.com/x/mtm/pull/18"}]'; exit 0;;
esac
exit 1
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTM_GH", script)
}

func runProjectLink(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newProjectLinkCmd()
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestProjectLinkStoresFieldsAndSummarizes(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	fakeGH(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	repo := linkedTestRepo(t)
	out, err := captureStdout(t, func() error {
		return runProjectLink(t, "inbox", "--repo", "you/inbox", "--path", repo)
	})
	if err != nil {
		t.Fatalf("project link: %v", err)
	}
	for _, want := range []string{"inbox", "github:    you/inbox", "repo_path:", "commits", "1 open issues"} {
		if !strings.Contains(out, want) {
			t.Errorf("link output missing %q:\n%s", want, out)
		}
	}
	// project.yaml carries both fields.
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_DATA_HOME"), "mtm", "projects", "inbox", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"github: you/inbox", "repo_path: " + repo} {
		if !strings.Contains(string(data), want) {
			t.Errorf("project.yaml missing %q:\n%s", want, data)
		}
	}
}

func TestProjectLinkValidation(t *testing.T) {
	gitEnv(t)
	sandbox(t)
	if err := runInit(""); err != nil {
		t.Fatal(err)
	}
	repo := linkedTestRepo(t)
	// No flags at all.
	if err := runProjectLink(t, "inbox"); err == nil {
		t.Fatal("link with no flags accepted")
	}
	// Bogus slug.
	if err := runProjectLink(t, "inbox", "--repo", "bogus"); err == nil {
		t.Fatal("link with bogus slug accepted")
	}
	// Path that is not a git repo.
	if err := runProjectLink(t, "inbox", "--path", t.TempDir()); err == nil {
		t.Fatal("link to a non-git path accepted")
	}
	// Unknown project.
	if err := runProjectLink(t, "nosuchproject", "--path", repo); err == nil {
		t.Fatal("link to an unknown project accepted")
	}
}

func TestRepoSlugParsing(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"loharrahulofficial-crypto/mtm", true},
		{"owner/repo.name_ok-1", true},
		{"A/B", true},
		{"owner", false},
		{"owner/repo/extra", false},
		{"/repo", false},
		{"owner/", false},
		{"ow ner/repo", false},
		{"owner/repo!", false},
		{"../repo", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := repoSlug.MatchString(tc.in); got != tc.want {
			t.Errorf("repoSlug(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
