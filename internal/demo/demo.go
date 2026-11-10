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
		content := substituteDates(tmpl, base)
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("demo seed: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("demo seed: %w", err)
		}
	}
	// Project milestones and dev-log entries (date-offset tokens like tasks).
	// Written here rather than in the corpus generator to keep the file list
	// readable.
	devlogRel := func(daysBack int, id string) string {
		return "college/dbms/devlog/" + base.AddDate(0, 0, daysBack).Format("2006-01-02") + "-" + id + ".md"
	}
	extras := map[string]string{
		"college/dbms/milestones/ms0001-v0-1-study-mvp.md": `---
id: ms0001
title: v0.1 — study MVP
target: @+12d@
status: active
description: Course companion features
created: @-18dT09:00@
updated: @-2dT10:00@
---
Scope: quick-add, grades, syllabus import.
`,
		"college/dbms/milestones/ms0002-v1-0-public-beta.md": `---
id: ms0002
title: v1.0 — public beta
target: @+90d@
status: planned
description: Open-source release
created: @-6dT09:00@
updated: @-6dT09:00@
---
`,
		devlogRel(-2, "dl00001"): `---
id: dl00001
title: Week 2 wrap
mood: flow
time_spent_minutes: 90
tags:
- study
created: @-2dT21:30@
---
ER diagrams, then 3NF. Lost an hour to the foreign-key rabbit hole but the schema clicked.
`,
		devlogRel(-6, "dl00002"): `---
id: dl00002
title: ER diagrams
time_spent_minutes: 45
tags:
- study
created: @-6dT20:15@
---
Crow's foot notation is opinionated and I respect it.
`,
	}
	for rel, tmpl := range extras {
		content := substituteDates(tmpl, base)
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("demo seed: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("demo seed: %w", err)
		}
	}

	// Ideas live at the repo root, outside projects/.
	ideasRoot := filepath.Join(root, "ideas")
	if err := os.MkdirAll(ideasRoot, 0o755); err != nil {
		return fmt.Errorf("demo seed: %w", err)
	}
	ideas := map[string]string{
		"ide0001-local-first-sync-engine.md": `---
id: ide0001
title: Local-first sync engine
status: exploring
rating: 5
tags:
- sync
- go
created: @-40dT10:00@
updated: @-2dT10:00@
---
CRDT or oplog?
Compare Automerge and ElectricSQL.
`,
		"ide0002-offline-flashcard-deck-for-exam-prep.md": `---
id: ide0002
title: Offline flashcard deck for exam prep
status: seed
tags:
- study
created: @-3dT10:00@
updated: @-1dT10:00@
---
`,
		"ide0003-pomodoro-ambient-scenes.md": `---
id: ide0003
title: Pomodoro ambient scenes
status: seed
rating: 3
tags:
- tui
created: @-20dT10:00@
updated: @-10dT10:00@
---
Rain, cafe, library.
`,
		"ide0004-paperless-todo-pipeline.md": `---
id: ide0004
title: Paperless todo pipeline
status: parked
rating: 2
tags:
- paper
created: @-90dT10:00@
updated: @-30dT10:00@
---
Waiting on a scanner.
`,
	}
	for rel, tmpl := range ideas {
		content := substituteDates(tmpl, base)
		path := filepath.Join(ideasRoot, rel)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("demo seed: %w", err)
		}
	}
	return nil
}

// substituteDates applies the @+Nd[T]HH:MM@ day-offset tokens the corpus
// uses, relative to base.
func substituteDates(tmpl string, base time.Time) string {
	return tokenRe.ReplaceAllStringFunc(tmpl, func(tok string) string {
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
}
