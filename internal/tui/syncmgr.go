package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

var errSyncConflict = errors.New("sync paused: conflicts need manual resolution")

type syncOutcome struct {
	note string
	err  error
}

// syncFinish commits each message with git, then syncs. It performs all git
// work and never resolves conflicts (D-003). Errors are folded into the
// outcome; the caller maps them to a status.
func syncFinish(cfg config.Config, root string, commitMsgs []string) syncOutcome {
	repo := sync.New(root)
	if !repo.IsRepo() {
		return syncOutcome{note: "data repo is not a git repository — change not committed"}
	}
	var notes []string
	for _, msg := range commitMsgs {
		if _, err := repo.CommitAll(msg); err != nil {
			notes = append(notes, fmt.Sprintf("auto-commit failed: %v", err))
		}
	}
	_, err := repo.Sync(cfg.Sync.Remote)
	if err != nil {
		if err == sync.ErrConflicted {
			notes = append(notes, "sync paused: conflicts need manual resolution")
			return syncOutcome{note: strings.Join(notes, "; "), err: errSyncConflict}
		}
		if len(notes) > 0 {
			notes = append(notes, fmt.Sprintf("push failed: %v", err))
			return syncOutcome{note: strings.Join(notes, "; "), err: errors.New("push failed")}
		}
		return syncOutcome{note: fmt.Sprintf("sync failed: %v", err), err: err}
	}
	if len(notes) > 0 {
		return syncOutcome{note: strings.Join(notes, "; ")}
	}
	return syncOutcome{}
}

// commitOnly is used on quit to flush pending commits synchronously without
// a network push (the pull/push debounce may never have fired).
func commitOnly(cfg config.Config, root string, commitMsgs []string) string {
	res := syncFinish(cfg, root, commitMsgs)
	return res.note
}
