package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestParseSyllabusMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    []SyllabusChapter
		wantErr bool
	}{
		{
			name: "headings and list items",
			data: "# Unit 1: ER model\n- entities\n- relationships\n\n## Unit 2: SQL\n1. joins\n2. normalization\n",
			want: []SyllabusChapter{
				{Title: "Unit 1: ER model", Subtopics: []string{"entities", "relationships"}},
				{Title: "Unit 2: SQL", Subtopics: []string{"joins", "normalization"}},
			},
		},
		{
			name: "checkbox markers are stripped",
			data: "# Normalization\n- [ ] 1NF\n- [x] 2NF\n* [ ] 3NF\n",
			want: []SyllabusChapter{
				{Title: "Normalization", Subtopics: []string{"1NF", "2NF", "3NF"}},
			},
		},
		{
			name: "prose is ignored",
			data: "A syllabus for the keen observer.\n\n# Week 1\nRead chapters 1-3 before class.\n- ER basics\n",
			want: []SyllabusChapter{
				{Title: "Week 1", Subtopics: []string{"ER basics"}},
			},
		},
		{name: "no chapters", data: "just some prose\nno headings here\n", wantErr: true},
		{name: "list item before any heading", data: "- orphan topic\n# Chapter\n", wantErr: true},
		{name: "empty heading", data: "#\n# Real chapter\n", want: []SyllabusChapter{{Title: "Real chapter"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chapters, err := ParseSyllabusMarkdown([]byte(tt.data))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(chapters) != len(tt.want) {
				t.Fatalf("got %d chapters, want %d", len(chapters), len(tt.want))
			}
			for i := range tt.want {
				if chapters[i].Title != tt.want[i].Title {
					t.Errorf("chapter %d title = %q, want %q", i, chapters[i].Title, tt.want[i].Title)
				}
				if strings.Join(chapters[i].Subtopics, ",") != strings.Join(tt.want[i].Subtopics, ",") {
					t.Errorf("chapter %d subtopics = %v, want %v", i, chapters[i].Subtopics, tt.want[i].Subtopics)
				}
			}
		})
	}
}

func TestParseSyllabusYAML(t *testing.T) {
	data := `
chapters:
  - title: "ER model"
    subtopics: ["entities", "relationships", "  "]
  - title: "SQL"
    topics: ["joins", "normalization"]
  - title: "No subtopics"
`
	chapters, err := ParseSyllabusYAML([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 3 {
		t.Fatalf("got %d chapters, want 3", len(chapters))
	}
	if strings.Join(chapters[0].Subtopics, ",") != "entities,relationships" {
		t.Errorf("blank subtopic not dropped: %v", chapters[0].Subtopics)
	}
	if strings.Join(chapters[1].Subtopics, ",") != "joins,normalization" {
		t.Errorf("topics alias not honored: %v", chapters[1].Subtopics)
	}
	if _, err := ParseSyllabusYAML([]byte("chapters: []\n")); err == nil {
		t.Error("empty chapters list accepted")
	}
	if _, err := ParseSyllabusYAML([]byte("- not a map\n")); err == nil {
		t.Error("garbage YAML accepted")
	}
}

func TestImportSyllabusCreatesChaptersAndSubtasks(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	chapters := []SyllabusChapter{
		{Title: "ER model", Subtopics: []string{"entities", "relationships"}},
		{Title: "SQL", Subtopics: []string{"joins"}},
	}
	res, err := ImportSyllabus(s, "college/dbms", chapters)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CreatedChapters) != 2 || len(res.CreatedSubtasks) != 3 {
		t.Fatalf("created %d chapters, %d subtasks; want 2/3", len(res.CreatedChapters), len(res.CreatedSubtasks))
	}
	for _, sub := range res.CreatedSubtasks {
		if sub.Parent == "" {
			t.Errorf("subtask %q has no parent", sub.Title)
		}
	}
	// Re-running is a no-op.
	res2, err := ImportSyllabus(s, "college/dbms", chapters)
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.CreatedChapters) != 0 || len(res2.CreatedSubtasks) != 0 {
		t.Fatalf("re-run created %d chapters, %d subtasks; want none", len(res2.CreatedChapters), len(res2.CreatedSubtasks))
	}
	if len(res2.Skipped) != 2 {
		t.Errorf("skipped = %v, want both chapters listed", res2.Skipped)
	}
}

func TestImportSyllabusReconcilesPartialImport(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	// Simulate a hand-created chapter task with no subtasks yet.
	p, err := s.GetProject("college/dbms")
	if err != nil {
		t.Fatal(err)
	}
	hand := &core.Task{Title: "ER model", Project: p.Name, Status: "todo", Kind: core.KindTask}
	if err := s.Create(hand); err != nil {
		t.Fatal(err)
	}
	res, err := ImportSyllabus(s, "dbms", []SyllabusChapter{{Title: "ER model", Subtopics: []string{"entities"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CreatedChapters) != 0 {
		t.Errorf("chapter recreated: %+v", res.CreatedChapters)
	}
	if len(res.CreatedSubtasks) != 1 || res.CreatedSubtasks[0].Parent != hand.ID {
		t.Errorf("subtask not linked to existing chapter: %+v", res.CreatedSubtasks)
	}
}

func TestLoadSyllabusFromFileAndTemplate(t *testing.T) {
	s := openActionStore(t, "college/dbms")
	// File path works.
	dir := t.TempDir()
	md := filepath.Join(dir, "dbms.md")
	if err := os.WriteFile(md, []byte("# Week 1\n- intro\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chapters, source, err := LoadSyllabus(s, md)
	if err != nil {
		t.Fatal(err)
	}
	if source != md || len(chapters) != 1 {
		t.Fatalf("got source %q, %d chapters", source, len(chapters))
	}
	// Template resolution from templates/ in the data repo.
	tplDir := filepath.Join(s.Root, "templates")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "dbms.yaml"), []byte("chapters:\n  - title: Unit 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chapters, source, err = LoadSyllabus(s, "dbms")
	if err != nil {
		t.Fatal(err)
	}
	if source != "templates/dbms.yaml" || chapters[0].Title != "Unit 1" {
		t.Fatalf("got source %q chapters %+v", source, chapters)
	}
	// Unknown template lists what is available.
	_, _, err = LoadSyllabus(s, "nope")
	if err == nil || !strings.Contains(err.Error(), "dbms") {
		t.Fatalf("error should list available templates, got %v", err)
	}
	// Path traversal in a template name is rejected.
	if _, _, err := LoadSyllabus(s, "../secret"); err == nil {
		t.Fatal("template name with .. accepted")
	}
	// Unsupported extension.
	bad := filepath.Join(dir, "syllabus.docx")
	if err := os.WriteFile(bad, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadSyllabus(s, bad); err == nil {
		t.Fatal("unsupported extension accepted")
	}
}
