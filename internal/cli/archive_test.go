package cli

import (
	"testing"
	"time"
)

func TestParseOlderThan(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"48h", 48 * time.Hour, false},
		{"720h", 720 * time.Hour, false},
		{"30D", 30 * 24 * time.Hour, false},
		{"2w ", 14 * 24 * time.Hour, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-3d", 0, true},
		{"d", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseOlderThan(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseOlderThan(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("parseOlderThan(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
