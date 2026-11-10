package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

const ideaScaffold = "# \n\n"

func newIdeaCmd() *cobra.Command {
	var (
		tags   []string
		rating int
	)
	cmd := &cobra.Command{
		Use:   "idea [title...]",
		Short: "Capture or manage incubation ideas",
		Long: `Capture a half-formed idea, or manage the ones you have by id.

Without arguments, $EDITOR opens on a scaffold: write the title after
"# " on the first line; everything below becomes the notes. --tag and
--rating apply to either form.

  mtm idea "git-backed notes" --tag sync --rating 4
  mtm idea                      # capture in $EDITOR
  mtm idea show|edit|status|rate|note|rm|promote <id>`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				data, path, err := editNewFile(cfg, "mtm-idea-*.md", []byte(ideaScaffold))
				if err != nil {
					return err
				}
				parsedTitle, body, ok := parseHashTitle(data)
				if !ok {
					return fmt.Errorf("empty idea discarded (no `# Title` line; scaffold kept at %s)", path)
				}
				i, err := actions.AddIdea(s, parsedTitle, parseTags(tags), rating)
				if err != nil {
					return err
				}
				i.Body = body
				if err := s.UpdateIdea(i); err != nil {
					return err
				}
				commitAndSync(cfg, s.Root, "idea: "+i.Title)
				fmt.Printf("idea %s  %s\n", i.ID, i.Title)
				return nil
			}
			if rating < 0 || rating > 5 {
				return fmt.Errorf("--rating must be 1-5, got %d", rating)
			}
			i, err := actions.AddIdea(s, title, parseTags(tags), rating)
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "idea: "+i.Title)
			fmt.Printf("idea %s  %s\n", i.ID, i.Title)
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "tags (comma-separated or repeated)")
	cmd.Flags().IntVarP(&rating, "rating", "r", 0, "rating 1-5 (omit for unrated)")
	cmd.AddCommand(
		newIdeaShowCmd(),
		newIdeaEditCmd(),
		newIdeaStatusCmd(),
		newIdeaRateCmd(),
		newIdeaNoteCmd(),
		newIdeaRmCmd(),
		newIdeaPromoteCmd(),
	)
	return cmd
}

func newIdeasCmd() *cobra.Command {
	var (
		status string
		tag    string
	)
	cmd := &cobra.Command{
		Use:   "ideas",
		Short: "List ideas (seed, exploring, parked, promoted, dropped)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			ides := s.Ideas()
			if st := strings.TrimSpace(status); st != "" {
				canonical := core.CanonicalIdeaStatus(st)
				if canonical == "" {
					return fmt.Errorf("invalid --status %q (want %s)", status, strings.Join(core.IdeaStatuses, "|"))
				}
				filtered := []*core.Idea{}
				for _, i := range ides {
					if i.Status == canonical {
						filtered = append(filtered, i)
					}
				}
				ides = filtered
			}
			if tg := strings.TrimSpace(tag); tg != "" {
				tg = strings.ToLower(strings.TrimPrefix(tg, "#"))
				filtered := []*core.Idea{}
				for _, i := range ides {
					for _, t := range i.Tags {
						if strings.EqualFold(t, tg) {
							filtered = append(filtered, i)
							break
						}
					}
				}
				ides = filtered
			}
			if len(ides) == 0 {
				fmt.Println("no ideas — capture one with `mtm idea \"title\"`")
				return nil
			}
			sortIdeas(ides)
			fmt.Printf("%-8s %-10s %-7s %-18s %s\n", "id", "status", "rating", "tags", "title")
			for _, i := range ides {
				rating := "-"
				if i.Rating > 0 {
					rating = fmt.Sprintf("%d/5", i.Rating)
				}
				fmt.Printf("%-8s %-10s %-7s %-18s %s\n", shortID(i.ID), i.Status, rating, truncateRunes(strings.Join(i.Tags, ","), 18), i.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (seed|exploring|parked|promoted|dropped)")
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	return cmd
}

// sortIdeas orders active ideas first (seed, exploring, parked by updated
// desc), then promoted and dropped.
func sortIdeas(ides []*core.Idea) {
	rank := map[string]int{core.IdeaSeed: 0, core.IdeaExploring: 1, core.IdeaParked: 2, core.IdeaPromoted: 3, core.IdeaDropped: 4}
	sort.SliceStable(ides, func(a, b int) bool {
		if rank[ides[a].Status] != rank[ides[b].Status] {
			return rank[ides[a].Status] < rank[ides[b].Status]
		}
		return ides[a].Updated > ides[b].Updated
	})
}

func newIdeaShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one idea (frontmatter + notes)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			i, err := s.ResolveIdea(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("id:         %s\n", i.ID)
			fmt.Printf("title:      %s\n", i.Title)
			fmt.Printf("status:     %s\n", i.Status)
			if i.Rating > 0 {
				fmt.Printf("rating:     %d/5\n", i.Rating)
			} else {
				fmt.Printf("rating:     unrated\n")
			}
			if len(i.Tags) > 0 {
				fmt.Printf("tags:       %s\n", strings.Join(i.Tags, ", "))
			}
			if len(i.Links) > 0 {
				fmt.Printf("links:      %s\n", strings.Join(i.Links, ", "))
			}
			fmt.Printf("created:    %s\n", i.Created)
			fmt.Printf("updated:    %s\n", i.Updated)
			if i.PromotedTo != "" {
				fmt.Printf("promoted_to: %s\n", i.PromotedTo)
			}
			if strings.TrimSpace(i.Body) != "" {
				fmt.Printf("\n%s\n", strings.TrimRight(i.Body, "\n"))
			}
			return nil
		},
	}
}

func newIdeaEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit the idea file in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			i, err := s.ResolveIdea(args[0])
			if err != nil {
				return err
			}
			path, err := s.IdeaPath(i.ID)
			if err != nil {
				return err
			}
			changed, err := editorEdited(cfg, path, "idea", func() error { return s.ReparseIdea(i) })
			if err != nil {
				return err
			}
			if !changed {
				fmt.Println("no changes")
				return nil
			}
			if err := s.UpdateIdea(i); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "idea edit: "+i.Title)
			fmt.Printf("updated %s  %s\n", i.ID, i.Title)
			return nil
		},
	}
}

func newIdeaStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id> <status>",
		Short: "Set the idea status (seed|exploring|parked|promoted|dropped)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			i, err := s.ResolveIdea(args[0])
			if err != nil {
				return err
			}
			canonical, err := actions.SetIdeaStatus(s, i, args[1])
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "idea status: "+i.Title)
			fmt.Printf("idea %s → %s\n", i.ID, canonical)
			return nil
		},
	}
}

func newIdeaRateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rate <id> <1-5>",
		Short: "Rate an idea 1-5",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			i, err := s.ResolveIdea(args[0])
			if err != nil {
				return err
			}
			r, err := core.ParseRating(args[1])
			if err != nil {
				return err
			}
			i.Rating = r
			if err := s.UpdateIdea(i); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "idea rate: "+i.Title)
			fmt.Printf("idea %s rated %d/5\n", i.ID, r)
			return nil
		},
	}
}

func newIdeaNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "note <id> <text...>",
		Short: "Append a dated note to an idea",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			i, err := s.ResolveIdea(args[0])
			if err != nil {
				return err
			}
			now := s.Now()
			if err := actions.NoteIdea(s, i, strings.Join(args[1:], " "), now); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "idea note: "+i.Title)
			fmt.Printf("noted %s  %s\n", i.ID, i.Title)
			return nil
		},
	}
}

func newIdeaRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete an idea (asks for confirmation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			i, err := s.ResolveIdea(args[0])
			if err != nil {
				return err
			}
			if !force {
				if err := confirm(fmt.Sprintf("remove idea %q?", i.Title)); err != nil {
					return err
				}
			}
			if err := s.RemoveIdea(i); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "idea rm: "+i.Title)
			fmt.Printf("removed %s  %s\n", i.ID, i.Title)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "remove without confirmation")
	return cmd
}

func newIdeaPromoteCmd() *cobra.Command {
	var (
		template string
		name     string
	)
	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promote an idea to a project (keeps the idea file)",
		Long: `Promote an idea to a project: creates the project with the standard
todo/doing/done statuses, one initial milestone (planned, target today+30d)
and — with --template — starter tasks from templates/<name>.md, one
quick-add line each, linked to the initial milestone. The idea's status
becomes promoted and promoted_to records the project. The idea file is kept.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			now := s.Now()
			res, err := actions.PromoteIdea(s, args[0], name, template, now)
			if err != nil {
				if res != nil {
					note("partial promotion: project %q and milestone %s were created", res.Project, res.MilestoneID)
					fmt.Fprintf(os.Stderr, "the idea was not marked promoted — re-run `mtm idea promote %s` after fixing the problem\n", args[0])
				}
				return err
			}
			commitAndSync(cfg, s.Root, "idea promote: "+res.Project)
			fmt.Printf("promoted → project %q\n", res.Project)
			fmt.Printf("milestone %s  Initial milestone  (target today+30d)\n", res.MilestoneID)
			for _, id := range res.TaskIDs {
				fmt.Printf("task      %s\n", id)
			}
			fmt.Printf("idea      %s marked promoted (file kept)\n", res.IdeaID)
			return nil
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "templates/<name>.md with starter tasks (one quick-add line per entry)")
	cmd.Flags().StringVar(&name, "name", "", "project name (default: slugified idea title)")
	return cmd
}
