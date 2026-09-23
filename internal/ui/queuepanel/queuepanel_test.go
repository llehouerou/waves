package queuepanel

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Bubble Tea reports shift+j as the rune J, never as "shift+j".
func TestMoveKeys(t *testing.T) {
	tests := []struct {
		name      string
		cursor    int
		key       tea.KeyMsg
		wantFirst string
	}{
		{"J", 0, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")}, "B"},
		{"shift+down", 0, tea.KeyMsg{Type: tea.KeyShiftDown}, "B"},
		{"K", 1, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")}, "B"},
		{"shift+up", 1, tea.KeyMsg{Type: tea.KeyShiftUp}, "B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"))
			m := New(q)
			m.SetSize(60, 10)
			m.SetFocused(true)
			m.list.Cursor().Jump(tt.cursor, q.Len(), m.listHeight())

			m.Update(tt.key)

			if got := q.Tracks()[0].Title; got != tt.wantFirst {
				t.Errorf("first track = %s, want %s", got, tt.wantFirst)
			}
		})
	}
}
