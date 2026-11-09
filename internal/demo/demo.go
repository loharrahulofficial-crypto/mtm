// Package demo seeds the example dataset `mtm demo` uses. The corpus is
// the reference dataset with every date expressed as a day offset from
// the 2026-11-08 reference day; Seed applies the offsets against the
// injected clock, so `make demo` always shows a populated Today/Week/
// Overdue/calendar/heatmap regardless of the real current date.
//
// Seeding is refused when the target repo's .mtm-data marker says
// kind: real (or any ancestor carries such a marker), or when the repo
// root resolves under the real passwd home — the demo seeder never
// touches a real data repo. Sandbox admission (MTM_SANDBOX=1 plus temp
// roots) is enforced by the caller (the demo-seed command).
//
// Tests seed with a fixed clock to stay deterministic.
package demo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/marker"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
)

var tokenRe = regexp.MustCompile(`@([+-]?\d+)d(T\d{2}:\d{2})?@`)

var idRe = regexp.MustCompile(`([a-z]{2}\d{4})-`)

var demoIDs = func() map[string]bool {
	ids := map[string]bool{}
	for rel := range corpus {
		if m := idRe.FindStringSubmatch(rel); m != nil {
			ids[m[1]] = true
		}
	}
	return ids
}()

// IDs returns the task IDs used by the demo corpus (dn0001, lt0001,
// od0001, ...). Used by `mtm doctor` to flag demo-looking data.
func IDs() map[string]bool { return demoIDs }

// IsDemoID reports whether id belongs to the demo corpus.
func IsDemoID(id string) bool { return demoIDs[strings.ToLower(id)] }

// Seed writes the demo dataset under dir (the projects root), shifting
// every date by the difference between `now` and the reference day.
// The file set and task identities never change — only dates move.
//
// The repo root for the target (see marker.RootForTarget) gets a
// kind: demo marker written if it has none; an ancestor marker saying
// kind: real refuses the seed outright.
func Seed(dir string, now time.Time) error {
	root, err := marker.RootForTarget(dir)
	if err != nil {
		return fmt.Errorf("demo seed: %w", err)
	}
	if m, ferr := marker.Read(root); ferr == nil && m.Kind == marker.KindReal {
		return fmt.Errorf("refusing to seed the demo dataset into %s — .mtm-data says kind: real; the demo seeder never touches a real data repo", root)
	}
	if paths.UnderRealHome(root) {
		return fmt.Errorf("refusing to seed the demo dataset into %s — it is under your real home directory", root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("demo seed: %w", err)
	}
	if _, _, err := marker.Ensure(root, marker.KindDemo); err != nil {
		return fmt.Errorf("demo seed: write %s: %w", filepath.Join(root, marker.Filename), err)
	}

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
