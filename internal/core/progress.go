package core

// SubtaskProgress derives percent completion from a task's subtasks.
// isDone is project-aware (may call store.IsDoneStatus). Tasks with no
// subtasks report total == 0.
func SubtaskProgress(all []*Task, parentID string, isDone func(project, status string) bool) (done, total int, percent float64) {
	for _, t := range all {
		if t.Parent != parentID {
			continue
		}
		total++
		doneNow := IsDoneStatusFallback(t.Status)
		if isDone != nil {
			doneNow = isDone(t.Project, t.Status)
		}
		if doneNow {
			done++
		}
	}
	if total == 0 {
		return 0, 0, 0
	}
	return done, total, float64(done) / float64(total) * 100
}
