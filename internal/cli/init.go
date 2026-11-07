package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var remote string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up the mtm data repo (git) and default config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(remote)
		},
	}
	cmd.Flags().StringVar(&remote, "remote", "", "optional git remote URL for the data repo")
	return cmd
}

// dirMissingOrEmpty reports whether path does not exist or is an empty
// directory — the only cases where cloning into it is safe.
func dirMissingOrEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s exists and is not a directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func runInit(remote string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	root := cfg.DataRepoPath()
	remoteName := cfg.Sync.Remote
	cloned := false

	// Default config, only when missing.
	if _, statErr := os.Stat(config.Path()); os.IsNotExist(statErr) {
		if err := config.WriteDefault(); err != nil {
			note("could not write default config: %v", err)
		} else {
			fmt.Printf("config: wrote default to %s\n", config.Path())
		}
	}

	// Fresh machine + remote with an existing repo: clone, don't init over it.
	if remote != "" {
		already := sync.New(root).IsRepo()
		if !already {
			empty, err := dirMissingOrEmpty(root)
			if err != nil {
				return err
			}
			if empty {
				if err := sync.Clone(remote, root); err != nil {
					note("clone of %s failed: %v — initializing a fresh local repo instead", remote, err)
				} else {
					cloned = true
					fmt.Printf("repo: cloned %s into %s\n", remote, root)
				}
			} else {
				note("%s is not empty and not a git repo — initializing a fresh repo instead of cloning", root)
			}
		}
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	repo := sync.New(root)
	already := repo.IsRepo()
	if !already {
		if err := repo.Init(); err != nil {
			return fmt.Errorf("git init: %w", err)
		}
	}

	// Layout: projects/, archive/, one project named inbox.
	archiveDir := filepath.Join(root, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return err
	}
	gitkeep := filepath.Join(archiveDir, ".gitkeep")
	if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
		_ = os.WriteFile(gitkeep, nil, 0o644)
	}
	s, err := store.Open(root, config.CachePath())
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	if !s.ProjectExists("inbox") {
		if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
			return fmt.Errorf("create inbox project: %w", err)
		}
	}

	if committed, err := repo.CommitAll("init: mtm data repo"); err != nil {
		note("initial commit failed: %v (set user.name/user.email in git config?)", err)
	} else if committed {
		fmt.Println("repo: initial commit created")
	}

	if remote != "" {
		if err := repo.AddRemote(remoteName, remote); err != nil {
			return fmt.Errorf("add remote %s: %w", remoteName, err)
		}
		fmt.Printf("remote: %s -> %s\n", remoteName, remote)
		if _, err := repo.Sync(remoteName); err != nil {
			switch {
			case err == sync.ErrConflicted:
				note("sync paused: conflicts need manual resolution — run `mtm sync --status`")
			case sync.IsUnrelatedHistories(err):
				note("local and remote histories are unrelated (both sides initialized?).")
				note("to keep both sides: git -C %s pull --rebase --allow-unrelated-histories origin", root)
				note("or start over from the remote: delete %s and re-run `mtm init --remote %s`", root, remote)
			default:
				note("push failed: %v — repo committed locally; push later with `mtm sync`", err)
			}
		} else {
			fmt.Println("sync: up to date with remote (pushed)")
		}
	}

	if cloned {
		fmt.Printf("data repo cloned at %s\n", root)
	} else if already {
		fmt.Printf("data repo already initialized at %s\n", root)
	} else {
		fmt.Printf("data repo initialized at %s\n", root)
	}
	fmt.Println("try: mtm add \"buy milk\" && mtm list")
	return nil
}
