package cli

import (
	"fmt"
	"os"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
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
	res := actions.CommitAndSync(cfg, root, msg)
	if res.Note != "" {
		note("%s", res.Note)
	}
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
