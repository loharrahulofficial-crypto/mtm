package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Course is the optional `course:` block of a project.yaml — it marks the
// project as an academic course and carries its metadata. A project without
// the block is a plain project.
type Course struct {
	Code        string   `yaml:"code"`
	Name        string   `yaml:"name"`
	Semester    string   `yaml:"semester,omitempty"`
	Credits     int      `yaml:"credits,omitempty"`
	TargetGrade string   `yaml:"target_grade,omitempty"`
	ExamDates   []string `yaml:"exam_dates,omitempty"`
}

// IsCourse reports whether the project carries a course block.
func (p *Project) IsCourse() bool { return p.Course != nil }

// ValidateCourse checks a course block is usable. Called from
// Project.Validate.
func (c *Course) Validate() error {
	if strings.TrimSpace(c.Code) == "" {
		return projectError("course needs a code (e.g. DBMS)")
	}
	if strings.TrimSpace(c.Name) == "" {
		return projectError("course needs a name (e.g. \"Database Systems\")")
	}
	if c.Credits < 0 || c.Credits > 60 {
		return projectError(fmt.Sprintf("course credits must be 0..60, got %d", c.Credits))
	}
	for _, d := range c.ExamDates {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return projectError(fmt.Sprintf("course exam_dates entry %q must be 2006-01-02", d))
		}
	}
	if strings.TrimSpace(c.TargetGrade) != "" {
		if _, err := GradeToPercent(c.TargetGrade); err != nil {
			return projectError(fmt.Sprintf("course target_grade: %v", err))
		}
	}
	return nil
}

var letterBase = map[byte]float64{'A': 90, 'B': 80, 'C': 70, 'D': 60, 'F': 0}

// GradeToPercent maps a letter grade ("A", "B+", "a-", case-insensitive) or a
// numeric percentage ("85", "85.5") to a 0–100 percentage. Plus is +3, minus
// is −3; the result is clamped to [0, 100].
func GradeToPercent(grade string) (float64, error) {
	g := strings.ToUpper(strings.TrimSpace(grade))
	if g == "" {
		return 0, fmt.Errorf("empty grade")
	}
	if f, err := strconv.ParseFloat(g, 64); err == nil {
		if f < 0 || f > 100 {
			return 0, fmt.Errorf("numeric grade %q outside 0..100", grade)
		}
		return f, nil
	}
	base, ok := letterBase[g[0]]
	if !ok {
		return 0, fmt.Errorf("unknown grade %q (want A..F with optional +/-, or a percentage)", grade)
	}
	switch rest := g[1:]; rest {
	case "":
	case "+":
		base += 3
	case "-":
		base -= 3
	default:
		return 0, fmt.Errorf("unknown grade %q (want A..F with optional +/-)", grade)
	}
	if base < 0 {
		base = 0
	}
	if base > 100 {
		base = 100
	}
	return base, nil
}

// Progress is a rolled-up completion fraction over a set of tasks,
// subtasks included.
type Progress struct {
	Done     int
	Total    int
	Fraction float64
	// Weighted reports whether weights were in effect (any task carried a
	// weight, or the caller asked for weighted math).
	Weighted bool
}

// ProgressFraction rolls tasks into one fraction. Every task passed in
// counts once (subtasks and parents alike). When weighted is true, each
// task is weighted by its Weight (default 1); the fraction is
// Σ(w·done)/Σ(w). With weighted=false the fraction is done/total.
func ProgressFraction(tasks []*Task, isDone func(*Task) bool, weighted bool) Progress {
	var p Progress
	if isDone == nil {
		isDone = func(t *Task) bool { return IsDoneStatusFallback(t.Status) }
	}
	if weighted {
		var sum, got float64
		p.Weighted = true
		for _, t := range tasks {
			w := 1.0
			if t.Weight != nil && *t.Weight > 0 {
				w = *t.Weight
			}
			sum += w
			if isDone(t) {
				got += w
				p.Done++
			}
			p.Total++
		}
		if sum > 0 {
			p.Fraction = got / sum
		}
		return p
	}
	p.Total = len(tasks)
	for _, t := range tasks {
		if isDone(t) {
			p.Done++
		}
	}
	if p.Total > 0 {
		p.Fraction = float64(p.Done) / float64(p.Total)
	}
	return p
}
