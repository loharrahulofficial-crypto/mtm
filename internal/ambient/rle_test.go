package ambient

import (
	"strings"
	"testing"
	"time"
)

func TestRunLengthOutputMergesRuns(t *testing.T) {
	st := stateFor(0.5, PhaseWork)
	s := New("ocean", ModeHalfblock) // big waves: long same-color runs
	frame := s.Frame(12*time.Second, 100, 30, st)
	// count foreground flips per line — must be far fewer than cell count
	flips := 0
	for _, l := range frame {
		flips += strings.Count(l, "38;2;")
	}
	if flips > 100*30/3 {
		t.Fatalf("%d color flips in %d cells — run-length merging is not working", flips, 100*30)
	}
}

func TestBytesPerFrameReported(t *testing.T) {
	for _, scene := range Registered() {
		s := New(scene, ModeHalfblock)
		_ = s.Frame(3*time.Second, 40, 14, stateFor(0.5, PhaseWork))
		// BytesPerFrame is exercised through Benchmarks too; here we assert
		// render produces sane sizes through the buffer directly.
		st := stateFor(0.5, PhaseWork)
		st2 := st
		b := newFrameBuffer(mustPaint(t, s), ModeHalfblock, 3*time.Second, 40, 14, st2, false)
		b.render(b.palette, 40, 14)
		if b.bytesOut() <= 0 {
			t.Fatalf("%s: bytes = 0", scene)
		}
		if b.bytesOut() > 60000 {
			t.Fatalf("%s: bytes = %d, unreasonably large (RLE broken?)", scene, b.bytesOut())
		}
	}
}

func mustPaint(t *testing.T, s Scene) paintable {
	t.Helper()
	p, ok := s.(paintable)
	if !ok {
		t.Fatal("scene is not paintable")
	}
	return p
}
