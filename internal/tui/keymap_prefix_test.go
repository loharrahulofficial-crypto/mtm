package tui

import "testing"

// Single-char bindings may never become unreachable because a named key
// starts with them (q → quit, t → tab, a → alt+s). Only true two-char
// sequences keep the pending state.
func TestIsPrefixAtomicNamedKeys(t *testing.T) {
	k := DefaultKeyMap()
	for _, atomic := range []string{"q", "t", "a", "c", "e", "p", "enter", "esc", "tab", "alt+s", "ctrl+d"} {
		if k.IsPrefix(atomic) {
			t.Errorf("IsPrefix(%q) = true — single-char binding would be unreachable", atomic)
		}
	}
	for _, seq := range []string{"g", "d"} {
		if !k.IsPrefix(seq) {
			t.Errorf("IsPrefix(%q) = false — multi-key sequence broken", seq)
		}
	}
}
