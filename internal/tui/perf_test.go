package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// buildBigRepo writes n task files directly (fast) and returns the root.
func buildBigRepo(b *testing.B, n int) string {
	b.Helper()
	root := b.TempDir()
	dir := filepath.Join(root, "projects", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	pdata := `description: inbox
statuses:
    - name: todo
    - name: doing
    - name: done
      done: true
`
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte(pdata), 0o644); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < n; i++ {
		content := fmt.Sprintf("---\nid: t%06x\ntitle: task number %d with some reasonable length title\nproject: inbox\nstatus: todo\npriority: medium\nkind: task\ncreated: 2026-11-07T10:00\nupdated: 2026-11-07T10:00\n---\nbody of task %d\n", i, i, i)
		name := fmt.Sprintf("t%06x-task-%d.md", i, i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func BenchmarkModelStart2000(b *testing.B) {
	root := buildBigRepo(b, 2000)
	cache := filepath.Join(b.TempDir(), "index.json")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, err := store.Open(root, cache)
		if err != nil {
			b.Fatal(err)
		}
		cfg := config.Default()
		cfg.DataRepo = root
		cfg.Sync.Auto = false
		m := New(cfg, s, sync.New(root))
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
		m = updated.(Model)
		_ = m.View()
		s.Close()
	}
}

func BenchmarkView2000(b *testing.B) {
	root := buildBigRepo(b, 2000)
	cache := filepath.Join(b.TempDir(), "index.json")
	s, err := store.Open(root, cache)
	if err != nil {
		b.Fatal(err)
	}
	cfg := config.Default()
	cfg.DataRepo = root
	cfg.Sync.Auto = false
	m := New(cfg, s, sync.New(root))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
	m = updated.(Model)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
