package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func newMilestoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone",
		Short: "Manage project milestones (planned|active|done|missed)",
	}
	cmd.AddCommand(newMilestoneAddCmd(), newMilestoneListCmd(), newMilestoneShowCmd(), newMilestoneEditCmd(), newMilestoneDoneCmd())
	return cmd
}

func newMilestoneAddCmd() *cobra.Command {
	var (
		target      string
		description string
	)
	cmd := &cobra.Command{
		Use:   "add <project> <title...>",
		Short: "Add a milestone to a project",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			title := strings.Join(args[1:], " ")
			if day := strings.TrimSpace(target); day != "" {
				if _, err := time.ParseInLocation("2006-01-02", day, time.Local); err != nil {
					return fmt.Errorf("--target %q must be 2006-01-02", day)
				}
			}
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			m := &core.Milestone{Title: title, Target: strings.TrimSpace(target), Status: core.MilestonePlanned, Description: description, Project: project}
			if description != "" {
				m.Body = description
			}
			if err := s.CreateMilestone(m); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "milestone add: "+m.Title)
			fmt.Printf("milestone %s  %s  (%s", m.ID, m.Title, m.Project)
			if m.Target != "" {
				fmt.Printf(", target %s", m.Target)
			}
			fmt.Println(")")
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "target date 2006-01-02")
	cmd.Flags().StringVar(&description, "description", "", "short description (also stored as the body)")
	return cmd
}

func newMilestoneListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list [project]",
		Short: "List milestones with progress (tasks linked via milestone:)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			project := ""
			if len(args) == 1 {
				resolved, err := actions.ResolveProject(s, args[0])
				if err != nil {
					return err
				}
				project = resolved
			}
			if st := strings.TrimSpace(status); st != "" {
				if core.CanonicalMilestoneStatus(st) == "" {
					return fmt.Errorf("invalid --status %q (want %s)", status, strings.Join(core.MilestoneStatuses, "|"))
				}
			}
			canon := core.CanonicalMilestoneStatus(status)
			now := s.Now()
			isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
			all := s.Tasks()
			var ms []*core.Milestone
			for _, m := range s.Milestones(project) {
				if canon != "" && m.Status != canon {
					continue
				}
				ms = append(ms, m)
			}
			if len(ms) == 0 {
				fmt.Println("no milestones — create one with `mtm milestone add <project> \"title\"`")
				return nil
			}
			sortMilestones(ms)
			fmt.Printf("%-9s %-18s %-28s %-10s %-9s %s %s\n", "id", "project", "title", "target", "status", "progress (linked tasks)", "when")
			for _, m := range ms {
				done, total, frac := core.MilestoneProgress(all, m.ID, isDone)
				target := m.Target
				if target == "" {
					target = "-"
				}
				prog := "no linked tasks"
				if total > 0 {
					prog = bar(core.Progress{Done: done, Total: total, Fraction: frac}, 16)
				}
				fmt.Printf("%-9s %-18s %-28s %-10s %-9s %s %s\n",
					shortID(m.ID), truncateRunes(m.Project, 18), truncateRunes(m.Title, 28), target, m.Status, prog, daysChip(m, now))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (planned|active|done|missed)")
	return cmd
}

func sortMilestones(ms []*core.Milestone) {
	rank := func(s string) int {
		switch strings.ToLower(s) {
		case core.MilestoneActive:
			return 0
		case core.MilestonePlanned:
			return 1
		case core.MilestoneDone:
			return 2
		case core.MilestoneMissed:
			return 3
		default:
			return 4
		}
	}
	for i := 1; i < len(ms); i++ { // insertion sort; tiny n
		for j := i; j > 0; j-- {
			a, b := ms[j], ms[j-1]
			rA, rB := rank(a.Status), rank(b.Status)
			before := false
			if rA != rB {
				before = rA < rB
			} else if a.Target == "" || b.Target == "" {
				before = b.Target != "" // dated milestones first
			} else {
				before = a.Target < b.Target
			}
			if !before {
				break
			}
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
}

// daysChip renders the "today|in Nd|overdue" suffix for a milestone's target.
func daysChip(m *core.Milestone, now time.Time) string {
	if m.Target == "" {
		return ""
	}
	day, err := time.ParseInLocation("2006-01-02", m.Target, now.Location())
	if err != nil {
		return ""
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := int(day.Sub(today).Hours() / 24)
	if strings.EqualFold(m.Status, core.MilestoneDone) {
		return " done"
	}
	switch {
	case days == 0:
		return " today"
	case days > 0:
		return fmt.Sprintf(" in %dd", days)
	default:
		return fmt.Sprintf(" %dd overdue", -days)
	}
}

func newMilestoneShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one milestone and its linked tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			m, err := s.ResolveMilestone(args[0])
			if err != nil {
				return err
			}
			now := s.Now()
			isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
			done, total, frac := core.MilestoneProgress(s.Tasks(), m.ID, isDone)
			fmt.Printf("id:          %s\n", m.ID)
			fmt.Printf("project:     %s\n", m.Project)
			fmt.Printf("title:       %s\n", m.Title)
			fmt.Printf("status:      %s\n", m.Status)
			if m.Target != "" {
				fmt.Printf("target:      %s%s\n", m.Target, daysChip(m, now))
			}
			fmt.Printf("progress:    %s (%d/%d)\n", bar(core.Progress{Done: done, Total: total, Fraction: frac}, 24), done, total)
			if m.Description != "" {
				fmt.Printf("description: %s\n", m.Description)
			}
			scope := core.MilestoneScope(s.Tasks(), m.ID)
			if len(scope) > 0 {
				fmt.Printf("tasks:\n")
				for _, t := range scope {
					mark := "[ ]"
					if isDone(t.Project, t.Status) {
						mark = "[x]"
					}
					fmt.Printf("  %-4s %s  %s\n", shortID(t.ID), mark, t.Title)
				}
			}
			if body := strings.TrimSpace(m.Body); body != "" {
				fmt.Printf("\n%s\n", body)
			}
			return nil
		},
	}
}

func newMilestoneEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit the milestone file in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			m, err := s.ResolveMilestone(args[0])
			if err != nil {
				return err
			}
			path, err := s.MilestonePath(m.ID)
			if err != nil {
				return err
			}
			changed, err := editorEdited(cfg, path, "milestone", func() error { return s.ReparseMilestone(m) })
			if err != nil {
				return err
			}
			if !changed {
				fmt.Println("no changes")
				return nil
			}
			if err := s.UpdateMilestone(m); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "milestone edit: "+m.Title)
			fmt.Printf("updated %s  %s\n", m.ID, m.Title)
			return nil
		},
	}
}

func newMilestoneDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a milestone done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			m, err := s.ResolveMilestone(args[0])
			if err != nil {
				return err
			}
			m.Status = core.MilestoneDone
			if err := s.UpdateMilestone(m); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "milestone done: "+m.Title)
			fmt.Printf("milestone %s  %s → done\n", m.ID, m.Title)
			return nil
		},
	}
}
