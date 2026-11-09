// Package contrib holds repo-level checks for the shell launchers.
package contrib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// launchRefs are the spellings of the mtm binary that count as launching
// it. Only the command position of a line is checked, so build lines like
// `go build -o bin/mtm` and `install ... bin/$(BINARY) ...` do not match.
var launchRefs = []string{
	"$binary", "${binary}", "bin/mtm", "./bin/mtm", "bin/$(BINARY)", "bin/${BINARY}", "$(BINARY)", "${BINARY}",
}

// firstCommandWord returns the executable word of a shell/make line, or ""
// for comments and blank lines. It strips the decorations make recipes
// allow: leading @/-, a leading subshell paren, and a leading `exec`.
func firstCommandWord(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	line = strings.TrimLeft(line, "@- ")
	line = strings.TrimPrefix(line, "exec ")
	line = strings.TrimPrefix(line, "(")
	line = strings.TrimSpace(line)
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `"'`)
}

func launchesMtm(line string) bool {
	fw := firstCommandWord(line)
	for _, ref := range launchRefs {
		if fw != ref {
			continue
		}
		args := strings.Fields(line)
		if len(args) >= 2 && args[1] == "completion" {
			// `mtm completion <shell>` prints a static script; it opens no
			// repo, config or cache, so running it during `make install`
			// is safe on the real machine.
			return false
		}
		return true
	}
	return false
}

// TestLaunchersSourceSandboxEnv fails if the Makefile or any script under
// contrib/ launches the mtm binary without referencing the shared sandbox
// env (contrib/sandbox-env.sh). Shared sourcing is what keeps the demo,
// the smoke run and snap.py from drifting apart again.
func TestLaunchersSourceSandboxEnv(t *testing.T) {
	root := filepath.Join("..")
	files := []string{filepath.Join(root, "Makefile")}
	skip := map[string]bool{"sandbox-env.sh": true, "sandbox_test.go": true} // the env itself; this test
	entries, err := os.ReadDir(filepath.Join(root, "contrib"))
	if err != nil {
		t.Fatalf("reading contrib/: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || skip[e.Name()] {
			continue
		}
		files = append(files, filepath.Join(root, "contrib", e.Name()))
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		content := string(data)
		launches := false
		for _, line := range strings.Split(content, "\n") {
			if launchesMtm(line) {
				launches = true
				break
			}
		}
		if !launches {
			continue
		}
		if !strings.Contains(content, "sandbox-env.sh") {
			t.Errorf("%s launches mtm but never references contrib/sandbox-env.sh — source it instead of hand-rolling HOME/XDG exports", path)
		}
	}
}
