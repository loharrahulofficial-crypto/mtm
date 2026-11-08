// Package demo seeds the example dataset `mtm demo` uses. The corpus is
// the reference dataset with every date expressed as a day offset from
// the 2026-11-08 reference day; Seed applies the offsets against the
// injected clock, so `make demo` always shows a populated Today/Week/
// Overdue/calendar/heatmap regardless of the real current date.
//
// Tests seed with a fixed clock to stay deterministic.
package demo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var tokenRe = regexp.MustCompile(`@([+-]?\d+)d(T\d{2}:\d{2})?@`)

// Seed writes the demo dataset under dir (the projects root), shifting
// every date by the difference between `now` and the reference day.
// The file set and task identities never change — only dates move.
func Seed(dir string, now time.Time) error {
	y, m, d := now.Date()
	base := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	for rel, tmpl := range corpus {
		content := tokenRe.ReplaceAllStringFunc(tmpl, func(tok string) string {
			parts := tokenRe.FindStringSubmatch(tok)
			n, err := strconv.Atoi(parts[1])
			if err != nil {
				return tok
			}
			day := base.AddDate(0, 0, n)
			out := day.Format("2006-01-02")
			if tm := parts[2]; tm != "" {
				out += tm
			}
			return out
		})
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("demo seed: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("demo seed: %w", err)
		}
	}
	return nil
}
