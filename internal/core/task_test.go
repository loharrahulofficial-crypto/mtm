package core

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Submit DBMS Assignment 2", "submit-dbms-assignment-2"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"C++ & C# !", "c-c"},
		{"", "task"},
		{"---", "task"},
		{"a very long title that keeps going and going and going and going and going and going and going beyond the limit", "a-very-long-title-that-keeps-going-and-going-and"},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewID()
		if len(id) != 6 {
			t.Fatalf("id %q: len %d, want 6", id, len(id))
		}
		for _, r := range id {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				t.Fatalf("id %q contains non-hex char %q", id, string(r))
			}
		}
		if seen[id] {
			t.Fatalf("id %q generated twice", id)
		}
		seen[id] = true
	}
}

func TestValidPriority(t *testing.T) {
	for _, p := range Priorities {
		if !ValidPriority(p) {
			t.Errorf("ValidPriority(%q) = false, want true", p)
		}
	}
	if ValidPriority("critical") {
		t.Error("ValidPriority(critical) = true, want false")
	}
}

func TestIsDoneStatusFallback(t *testing.T) {
	for _, s := range []string{"done", "Done", "completed", "cancelled"} {
		if !IsDoneStatusFallback(s) {
			t.Errorf("IsDoneStatusFallback(%q) = false, want true", s)
		}
	}
	if IsDoneStatusFallback("doing") {
		t.Error("IsDoneStatusFallback(doing) = true, want false")
	}
}
