package tui

import (
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestAmbientCommandFlow(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{testTask("revise ch.6", core.KindStudy)})
	_ = s
	m = press(t, m, ":")
	if m.mode != modePalette {
		t.Fatalf(": opened mode %d, want modePalette", m.mode)
	}
	m = typeInto(t, m, "ambient") // helper ends with enter
	if m.mode != modeAmbient {
		t.Fatalf(":ambient flow landed in mode %d, want modeAmbient (=:ambient)", m.mode)
	}
}
