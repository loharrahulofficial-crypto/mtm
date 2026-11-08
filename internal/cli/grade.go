package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func newGradeCmd() *cobra.Command {
	var max, weight string
	cmd := &cobra.Command{
		Use:   "grade <id> <score>",
		Short: "Record a score on an assignment or exam",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			score, err := parseFloatArg("score", args[1])
			if err != nil {
				return err
			}
			maxV, err := parseFloatArg("--max", max)
			if err != nil {
				return err
			}
			var weightPtr *float64
			if strings.TrimSpace(weight) != "" {
				w, err := parseFloatArg("--weight", weight)
				if err != nil {
					return err
				}
				weightPtr = &w
			}
			t, err := actions.RecordGrade(s, args[0], score, maxV, weightPtr)
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "grade: "+t.Title)
			pct := score / maxV * 100
			fmt.Printf("recorded %g/%g (%.1f%%) on %s  %s", score, maxV, pct, shortID(t.ID), t.Title)
			if weightPtr != nil {
				fmt.Printf(" · weight %g", *weightPtr)
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&max, "max", "100", "maximum score")
	cmd.Flags().StringVar(&weight, "weight", "", "grade weight % (sets/overrides the task's weight)")
	return cmd
}

func newGradesCmd() *cobra.Command {
	var (
		course, target string
		asJSON         bool
	)
	cmd := &cobra.Command{
		Use:   "grades [--course X]",
		Short: "Show per-course weighted totals and what you need to hit the target grade",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			var names []string
			if strings.TrimSpace(course) != "" {
				name, err := actions.ResolveCourse(s, course)
				if err != nil {
					return err
				}
				names = []string{name}
			} else {
				courses, err := actions.ListCourses(s)
				if err != nil {
					return err
				}
				if len(courses) == 0 {
					fmt.Println("no courses yet — mark one with `mtm course add`")
					return nil
				}
				for _, c := range courses {
					names = append(names, c.Name)
				}
			}
			if asJSON {
				type courseOut struct {
					Course string            `json:"course"`
					Code   string            `json:"code,omitempty"`
					Target *float64          `json:"target_pct,omitempty"`
					G      core.CourseGrades `json:"grades"`
				}
				out := make([]courseOut, 0, len(names))
				for _, name := range names {
					g, p, err := actions.CollectCourseGrades(s, name, target)
					if err != nil {
						return err
					}
					co := courseOut{Course: name, G: *g}
					if p.Course != nil {
						co.Code = p.Course.Code
					}
					if g.HasTarget {
						t := g.TargetPct
						co.Target = &t
					}
					out = append(out, co)
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			for i, name := range names {
				if i > 0 {
					fmt.Println()
				}
				g, p, err := actions.CollectCourseGrades(s, name, target)
				if err != nil {
					return err
				}
				printCourseGrades(name, p, g)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&course, "course", "", "limit to one course (name or leaf)")
	cmd.Flags().StringVar(&target, "target", "", "target grade override (e.g. A, 85)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func parseFloatArg(name, raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s %q must be a number", name, raw)
	}
	return f, nil
}

func printCourseGrades(name string, p *core.Project, g *core.CourseGrades) {
	targetLabel := "no target grade"
	if g.HasTarget {
		targetLabel = fmt.Sprintf("target %s (%.0f%%)", targetRaw(p, *g), g.TargetPct)
	}
	fmt.Printf("%s — %s\n", name, targetLabel)
	if len(g.Items) == 0 {
		fmt.Println("  no assignments or exams yet — add one with `mtm add --kind assignment` or `mtm exam add`")
		return
	}
	for _, it := range g.Items {
		weight := "-"
		if it.HasWeight {
			weight = fmt.Sprintf("w %g", it.Weight)
		}
		score := "not graded"
		if it.Scored {
			max := it.ScoreMax
			if max == 0 {
				max = 100
			}
			score = fmt.Sprintf("%g/%g (%.1f%%)", it.Score, max, it.ScorePct())
		}
		fmt.Printf("  %-8s %-32s %-10s %-8s %s\n", shortID(it.ID), truncateRunes(it.Title, 32), it.Kind, weight, score)
	}
	if g.Unweighted {
		fmt.Printf("  simple average %.1f%% over graded work — no weights set\n", g.UnweightedPct)
		fmt.Printf("  ! no item carries a weight; set one with `mtm grade <id> <score> --weight N`\n")
		return
	}
	fmt.Printf("  assigned %g · graded %g · remaining %g\n", g.TotalWeight, g.EarnedWeight, g.RemainingWeight)
	fmt.Printf("  current %.1f%% on graded work · standing %.1f%% of assigned weight\n", g.CurrentPct, g.StandingPct)
	switch {
	case g.HasTarget && g.RemainingWeight == 0:
		if g.StandingPct >= g.TargetPct {
			fmt.Printf("  all weight graded — target met (%.1f%% ≥ %.0f%%)\n", g.StandingPct, g.TargetPct)
		} else {
			fmt.Printf("  all weight graded — final %.1f%%, target %.0f%% not met\n", g.StandingPct, g.TargetPct)
		}
	case g.HasTarget:
		switch {
		case g.NeededSecured:
			fmt.Printf("  to reach target: already secured — remaining work can't pull you below %.0f%%\n", g.TargetPct)
		case g.NeededFeasible:
			fmt.Printf("  to reach target (%.0f%%): need %.1f%% average on the remaining %g\n", g.TargetPct, g.NeededOnRemaining, g.RemainingWeight)
		default:
			best := 0.0
			if g.TotalWeight > 0 {
				best = (g.StandingPct*g.TotalWeight + 100*g.RemainingWeight) / g.TotalWeight
			}
			fmt.Printf("  to reach target (%.0f%%): out of reach — even 100%% on the remaining yields %.1f%%\n", g.TargetPct, best)
		}
	default:
		fmt.Printf("  to hit a target grade: none set — add target_grade to the course or pass --target A\n")
	}
	if g.TotalWeightOff {
		fmt.Printf("  ! assigned weights total %g, not 100 — the needed average assumes only assigned work counts\n", g.TotalWeight)
	}
	if len(g.MissingWeights) > 0 {
		fmt.Printf("  ! no weight on: %s (excluded from weighted math)\n", strings.Join(g.MissingWeights, ", "))
	}
}

func targetRaw(p *core.Project, g core.CourseGrades) string {
	if p != nil && p.Course != nil && strings.TrimSpace(p.Course.TargetGrade) != "" {
		return p.Course.TargetGrade
	}
	return fmt.Sprintf("%.0f%%", g.TargetPct)
}
