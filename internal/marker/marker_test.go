package marker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	root := t.TempDir()
	m, err := Write(root, KindReal)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != KindReal || m.ID == "" || m.Created == "" {
		t.Fatalf("bad marker: %+v", m)
	}
	got, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != m {
		t.Fatalf("round trip mismatch: %+v != %+v", got, m)
	}
}

func TestReadMissingReturnsErrNotExist(t *testing.T) {
	_, err := Read(t.TempDir())
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}

func TestEnsureNeverRewritesExistingMarker(t *testing.T) {
	root := t.TempDir()
	first, err := Write(root, KindReal)
	if err != nil {
		t.Fatal(err)
	}
	second, created, err := Ensure(root, KindDemo)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("Ensure rewrote an existing marker")
	}
	if second.Kind != KindReal || second.ID != first.ID {
		t.Fatalf("marker changed: %+v -> %+v", first, second)
	}
}

func TestFindUpWalksAncestors(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	want, err := Write(root, KindDemo)
	if err != nil {
		t.Fatal(err)
	}
	gotRoot, got, err := FindUp(deep)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != root || got != want {
		t.Fatalf("FindUp = (%s, %+v), want (%s, %+v)", gotRoot, got, root, want)
	}
}

func TestFindUpNoMarker(t *testing.T) {
	// Walk-up from a temp dir: must not find a marker above (temp set has none).
	_, _, err := FindUp(filepath.Join(t.TempDir(), "x", "y"))
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}

func TestRootForTarget(t *testing.T) {
	root := t.TempDir()
	// projects/ layout: root is the parent.
	got, err := RootForTarget(filepath.Join(root, "projects"))
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("RootForTarget = %s, want %s", got, root)
	}
	// Marker somewhere above wins.
	if _, err := Write(root, KindReal); err != nil {
		t.Fatal(err)
	}
	got, err = RootForTarget(filepath.Join(root, "projects", "inbox"))
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("RootForTarget with marker = %s, want %s", got, root)
	}
	// A non-projects dir with no marker is its own root.
	other := t.TempDir()
	got, err = RootForTarget(other)
	if err != nil {
		t.Fatal(err)
	}
	if got != other {
		t.Fatalf("RootForTarget = %s, want %s", got, other)
	}
}

func TestWriteOverwritesOnlyWhenAsked(t *testing.T) {
	root := t.TempDir()
	m1, err := Write(root, KindDemo)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := Write(root, KindReal)
	if err != nil {
		t.Fatal(err)
	}
	if m1.ID == m2.ID || m2.Kind != KindReal {
		t.Fatalf("Write did not replace marker: %+v vs %+v", m1, m2)
	}
}
