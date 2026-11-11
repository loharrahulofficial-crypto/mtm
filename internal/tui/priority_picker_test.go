package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// Regression: the priority picker previously discarded its mutated model
// (value receiver, no return) — pressing `p` did nothing at all. It now
// opens the picker like every other action picker.
func TestPriorityPickerOpensOnP(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{testTask("revise ch.6", core.KindStudy)})
	defer func() { _ = s.Close() }()
	m.view = viewList
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	if m.mode != modePicker {
		t.Fatalf("pressing p did not open the picker (mode=%v)", m.mode)
	}
	if m.picker != pickerPriority {
		t.Fatalf("picker = %v, want pickerPriority", m.picker)
	}
	if len(m.pickerItems) == 0 {
		t.Fatal("priority picker opened with no items")
	}
}
