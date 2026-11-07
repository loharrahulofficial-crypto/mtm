package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// openStore loads config and opens the data repo. Caller must close.
func openStore() (config.Config, *store.Store, error) {
	return openStoreOpts(false)
}

// openStoreOpts is openStore with the archive/ tree included in the index.
func openStoreOpts(includeArchived bool) (config.Config, *store.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, nil, err
	}
	s, err := store.Open(cfg.DataRepoPath(), config.CachePath(), includeArchived)
	if err != nil {
		return cfg, nil, err
	}
	return cfg, s, nil
}

func note(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "note: "+format+"\n", a...)
}

func printDiagnostics(s *store.Store) {
	for _, d := range s.Diagnostics() {
		note("skipped %s", d)
	}
}

// commitAndSync auto-commits a data-repo change and best-effort syncs.
// Failures are reported on stderr but never fail the command: the change
// already happened and must not be lost or rolled back.
func commitAndSync(cfg config.Config, root string, msg string) {
	if !cfg.Sync.Auto {
		return
	}
	repo := sync.New(root)
	if !repo.IsRepo() {
		note("data repo at %s is not a git repository — change not committed", root)
		return
	}
	if _, err := repo.CommitAll(msg); err != nil {
		note("auto-commit failed: %v", err)
		return
	}
	if _, err := repo.Sync(cfg.Sync.Remote); err != nil {
		if err == sync.ErrConflicted {
			note("sync paused: conflicts need manual resolution — run `mtm sync --status`")
			return
		}
		note("committed locally but not pushed (%v) — run `mtm sync` when online", err)
		return
	}
}

// spawnNextOccurrence creates the follow-up for a recurring task that was
// just completed. The message describes what was spawned, or "".
func spawnNextOccurrence(s *store.Store, t *core.Task) string {
	if strings.TrimSpace(t.Recurrence) == "" {
		return ""
	}
	first := "todo"
	if p, err := s.GetProject(t.Project); err == nil && len(p.Statuses) > 0 {
		first = p.Statuses[0].Name
	}
	spawned, ok, err := core.SpawnRecurringOccurrence(t, first, time.Now())
	if err != nil {
		note("could not compute next occurrence: %v", err)
		return ""
	}
	if !ok {
		return ""
	}
	if err := s.Create(spawned); err != nil {
		note("could not create next occurrence: %v", err)
		return ""
	}
	return fmt.Sprintf("next occurrence %s  due %s\n", spawned.ID, spawned.Due)
}

// taskFilter builds a project-aware filter hiding done tasks unless all.
func taskFilter(s *store.Store, project, tag, status, priority string, all bool) core.Filter {
	return core.Filter{
		Project:          project,
		Tag:              tag,
		Status:           status,
		Priority:         priority,
		IncludeCompleted: all,
		DoneCheck:        func(p, st string) bool { return s.IsDoneStatus(p, st) },
	}
}
