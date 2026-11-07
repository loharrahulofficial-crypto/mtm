package core

import "testing"

func TestGradeToPercent(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"A", 90, false}, {"A+", 93, false}, {"A-", 87, false},
		{"B", 80, false}, {"B+", 83, false}, {"B-", 77, false},
		{"C", 70, false}, {"C+", 73, false}, {"C-", 67, false},
		{"D", 60, false}, {"D+", 63, false}, {"D-", 57, false},
		{"F", 0, false},
		{"a", 90, false}, {"b-", 77, false}, {"f", 0, false},
		{"85", 85, false}, {"85.5", 85.5, false}, {"0", 0, false}, {"100", 100, false},
		{"", 0, true}, {"E", 0, true}, {"A++", 0, true}, {"101", 0, true}, {"-5", 0, true}, {"ABC", 0, true},
	}
	for _, c := range cases {
		got, err := GradeToPercent(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("GradeToPercent(%q) = %v, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("GradeToPercent(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("GradeToPercent(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestCourseValidate(t *testing.T) {
	cases := []struct {
		name    string
		course  Course
		wantErr bool
	}{
		{"minimal", Course{Code: "DBMS", Name: "Database Systems"}, false},
		{"full", Course{Code: "DBMS", Name: "Database Systems", Semester: "2026-Fall", Credits: 4, TargetGrade: "A", ExamDates: []string{"2026-12-01"}}, false},
		{"numeric target", Course{Code: "OS", Name: "Operating Systems", TargetGrade: "85"}, false},
		{"missing code", Course{Name: "x"}, true},
		{"missing name", Course{Code: "DBMS"}, true},
		{"credits too big", Course{Code: "DBMS", Name: "x", Credits: 61}, true},
		{"credits negative", Course{Code: "DBMS", Name: "x", Credits: -1}, true},
		{"bad exam date", Course{Code: "DBMS", Name: "x", ExamDates: []string{"01/12/2026"}}, true},
		{"bad target grade", Course{Code: "DBMS", Name: "x", TargetGrade: "H"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.course.Validate()
			if c.wantErr != (err != nil) {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

func TestProgressFraction(t *testing.T) {
	w := func(f float64) *float64 { return &f }
	tasks := []*Task{
		{Title: "a", Status: "done"},
		{Title: "b", Status: "doing"},
		{Title: "c", Status: "done", Weight: w(30)},
		{Title: "d", Status: "todo", Weight: w(70)},
	}
	isDone := func(t *Task) bool { return IsDoneStatusFallback(t.Status) }

	plain := ProgressFraction(tasks, isDone, false)
	if plain.Done != 2 || plain.Total != 4 || plain.Fraction != 0.5 {
		t.Fatalf("plain = %+v", plain)
	}
	weighted := ProgressFraction(tasks, isDone, true)
	// weights: a=1 (done), b=1, c=30 (done), d=70 → 31/102
	want := 31.0 / 102.0
	if !weighted.Weighted || weighted.Fraction < want-1e-9 || weighted.Fraction > want+1e-9 {
		t.Fatalf("weighted = %+v, want fraction %v", weighted, want)
	}

	if got := ProgressFraction(nil, isDone, false); got.Fraction != 0 || got.Total != 0 {
		t.Fatalf("empty = %+v", got)
	}
	// All weights zero → fraction 0, never NaN.
	zero := []*Task{{Title: "x", Weight: w(0)}}
	if got := ProgressFraction(zero, isDone, true); got.Fraction != 0 {
		t.Fatalf("zero weight = %+v", got)
	}
}
