package tui

import (
	"strings"
	"testing"
)

// reference dropsBGTest mirrors the original strings.Split-based
// implementation; kept in the test to prove byte-scan equivalence.
func referenceDropsBG(seq string) bool {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		switch p := parts[i]; p {
		case "", "0", "00", "000":
			return true
		case "38", "48", "58":
			switch {
			case i+2 < len(parts) && parts[i+1] == "2":
				i += 4
			case i+1 < len(parts) && parts[i+1] == "5":
				i += 2
			case i+1 < len(parts):
				i++
			}
		case "49":
			return true
		}
	}
	return false
}

func TestSgrDropsBGMatchesReference(t *testing.T) {
	cases := []string{
		"\x1b[m",
		"\x1b[0m",
		"\x1b[00m",
		"\x1b[000m",
		"\x1b[38;2;1;2;3m",
		"\x1b[48;2;10;20;30m",
		"\x1b[48;2;10;20;30;0m",
		"\x1b[38;2;255;255;255m",
		"\x1b[38;5;123m",
		"\x1b[48;5;200m",
		"\x1b[38;5;200m",
		"\x1b[38;5m",
		"\x1b[38;2;1;2m",
		"\x1b[38;2;1;2;3;49m",
		"\x1b[38;2;1;2;3;0m",
		"\x1b[38;2;1;2;3;\x1b[0m", // not a real sequence, but the raw bytes matter
		"\x1b[49m",
		"\x1b[1;49;2m",
		"\x1b[38;2;1;2;3;49;2m",
		"\x1b[38;;2m",
		"\x1b[;;m",
		"\x1b[0;0m",
		"\x1b[4;2;3m",
		"\x1b[38;9;9m",
		"\x1b[48;5;200;38;2;1;2;3m",
		"\x1b[38;2;1;2;3;48;5;9m",
		"\x1b[48;2;20;40;60;38;2;70;90;110m",
		"\x1b[58;2;100;100;100m",
		"\x1b[58;5;7m",
		"\x1b[38;5;200;49m",
		"\x1b[39m", // foreground default — NOT a bg drop
		"\x1b[48;5m",
		"\x1b[48;2;0;0;0m",
		"\x1b[1m",
		"\x1b[22m",
		"\x1b[38;2;0;0;0;48;2;1;1;1m",
	}
	for _, c := range cases {
		if got, want := sgrDropsBG(c), referenceDropsBG(c); got != want {
			t.Errorf("sgrDropsBG(%q) = %v, reference = %v", c, got, want)
		}
	}
}
