package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommandPrintsEmbeddedInfo(t *testing.T) {
	cmd := newVersionCmd()
	cmd.SetArgs(nil)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"mtm ", "commit:", "built:", "go:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing %q:\n%s", want, out)
		}
	}
}

func TestRootStillCarriesVersion(t *testing.T) {
	root := NewRootCmd()
	if root.Version == "" {
		t.Fatal("root command carries an empty version")
	}
}
