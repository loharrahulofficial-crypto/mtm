package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var (
		status bool
		abort  bool
		cont   bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull --rebase and push the data repo (auto-sync is paused on conflicts)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			root := cfg.DataRepoPath()
			repo := sync.New(root)
			if !repo.IsRepo() {
				return fmt.Errorf("data repo at %s is not a git repository — run `mtm init`", root)
			}

			if status {
				st := repo.GetStatus(cfg.Sync.Remote)
				if asJSON {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(st)
				}
				fmt.Printf("repo:    %s\n", root)
				fmt.Printf("branch:  %s\n", st.Branch)
				if st.RemoteConfigured {
					fmt.Printf("remote:  %s (%s)\n", st.Remote, st.RemoteURL)
				} else {
					fmt.Printf("remote:  %s (not configured)\n", st.Remote)
					fmt.Printf("hint:    add one with `mtm init --remote <url>` (clones existing repos),\n")
					fmt.Printf("         or manually: git -C %s remote add %s <url>\n", root, st.Remote)
				}
				fmt.Printf("ahead:   %d commit(s)\n", st.Ahead)
				if st.State.Conflicted {
					fmt.Printf("state:   CONFLICTED since %s\n", st.State.Since.Format("2006-01-02 15:04"))
					fmt.Println("files:")
					for _, f := range st.State.Files {
						fmt.Printf("  - %s\n", f)
					}
					fmt.Println("resolve: fix the files in the data repo (or `mtm sync --abort` to undo the pull and re-enable sync), then `mtm sync --continue`")
					return nil
				}
				fmt.Println("state:   ok")
				return nil
			}

			if abort {
				if err := repo.Abort(); err != nil {
					return err
				}
				fmt.Println("rebase aborted; sync re-enabled (conflicted state cleared)")
				return nil
			}

			st, err := repo.LoadState()
			if err != nil {
				return err
			}
			if cont {
				if repo.RebaseInProgress() {
					if err := repo.RebaseContinue(); err != nil {
						return err
					}
				} else {
					files, ferr := repo.ConflictedFiles()
					if ferr != nil || len(files) > 0 {
						if len(files) == 0 {
							files = []string{"?"}
						}
						return fmt.Errorf("no rebase in progress and unmerged files remain: %s", strings.Join(files, ", "))
					}
				}
				if err := repo.ClearState(); err != nil {
					return err
				}
				res, err := repo.Sync(cfg.Sync.Remote)
				if err == sync.ErrConflicted {
					return err
				}
				if err != nil {
					return fmt.Errorf("sync failed: %w", err)
				}
				_ = res
				fmt.Println("rebase continued; sync re-enabled")
				return nil
			}
			if st.Conflicted {
				fmt.Println("sync is paused: the data repo has unresolved conflicts")
				for _, f := range st.Files {
					fmt.Printf("  - %s\n", f)
				}
				fmt.Println("fix the files in the data repo, then `mtm sync --continue`;")
				fmt.Println("or `mtm sync --abort` to undo the pull and re-enable sync")
				return sync.ErrConflicted
			}

			res, err := repo.Sync(cfg.Sync.Remote)
			if err == sync.ErrConflicted {
				fmt.Println("pull conflicted — sync paused (never auto-resolving)")
				st, _ := repo.LoadState()
				for _, f := range st.Files {
					fmt.Printf("  - %s\n", f)
				}
				fmt.Println("fix the files, then `mtm sync --continue`; inspect with `mtm sync --status`")
				return err
			}
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}
			switch {
			case res.Pulled && res.Pushed:
				fmt.Println("sync: pulled, rebased and pushed")
			case res.Pushed:
				fmt.Println("sync: pushed")
			default:
				fmt.Println("sync: no remote configured (local-only)")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&status, "status", false, "show sync state (branch, remote, ahead, conflicts)")
	cmd.Flags().BoolVar(&abort, "abort", false, "abort an in-progress rebase and clear the conflicted state")
	cmd.Flags().BoolVar(&cont, "continue", false, "continue an in-progress rebase after manual resolution and resume sync")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON (with --status)")
	return cmd
}
