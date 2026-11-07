package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var (
		status bool
		abort  bool
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
				if st.RemoteURL != "" {
					fmt.Printf("remote:  %s (%s)\n", st.Remote, st.RemoteURL)
				} else {
					fmt.Printf("remote:  %s (not configured)\n", st.Remote)
				}
				fmt.Printf("ahead:   %d commit(s)\n", st.Ahead)
				if st.State.Conflicted {
					fmt.Printf("state:   CONFLICTED since %s\n", st.State.Since.Format("2006-01-02 15:04"))
					fmt.Println("files:")
					for _, f := range st.State.Files {
						fmt.Printf("  - %s\n", f)
					}
					fmt.Println("resolve: fix the files in the data repo, commit, then `mtm sync --abort` or run `mtm sync`")
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
			if st.Conflicted {
				fmt.Println("sync is paused: the data repo has unresolved conflicts")
				for _, f := range st.Files {
					fmt.Printf("  - %s\n", f)
				}
				fmt.Println("fix the files in the data repo, commit the resolution, then `mtm sync`;")
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
				fmt.Println("fix the files, commit, then `mtm sync`; inspect with `mtm sync --status`")
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
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON (with --status)")
	return cmd
}
