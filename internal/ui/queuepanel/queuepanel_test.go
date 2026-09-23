package queuepanel

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/action"
)

// Every gesture that changes the queue must leave it untouched and ask for
// the change as one action instead.
func TestEditGestures_EmitIntents(t *testing.T) {
	runes := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	tests := []struct {
		name     string
		current  int
		cursor   int
		selected []int
		key      tea.KeyMsg
		want     action.Action
	}{
		{"delete cursor track", 0, 1, nil, runes("d"), RemoveTracks{Indices: []int{1}}},
		{"delete selection", 0, 3, []int{2, 0}, runes("d"), RemoveTracks{Indices: []int{0, 2}}},
		{"keep only selected", 0, 0, []int{1, 3}, runes("D"), RemoveTracks{Indices: []int{0, 2}}},
		{"clear except playing", 2, 0, nil, runes("c"), RemoveTracks{Indices: []int{0, 1, 3}}},
		{"clear with nothing playing", -1, 0, nil, runes("c"), RemoveTracks{Indices: []int{0, 1, 2, 3}}},
		// Bubble Tea reports shift+j as the rune J, never as "shift+j".
		{"move cursor track down (J)", 0, 1, nil, runes("J"), MoveTracks{Indices: []int{1}, Delta: 1}},
		{"move cursor track down (shift+down)", 0, 1, nil, tea.KeyMsg{Type: tea.KeyShiftDown}, MoveTracks{Indices: []int{1}, Delta: 1}},
		{"move selection up (K)", 0, 0, []int{3, 1}, runes("K"), MoveTracks{Indices: []int{1, 3}, Delta: -1}},
		{"move selection up (shift+up)", 0, 0, []int{3, 1}, tea.KeyMsg{Type: tea.KeyShiftUp}, MoveTracks{Indices: []int{1, 3}, Delta: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"), testTrack("C", "x"), testTrack("D", "x"))
			q.JumpTo(tt.current)
			m := New(testService(t, q))
			m.SetSize(60, 10)
			m.SetFocused(true)
			m.list.Cursor().Jump(tt.cursor, q.Len(), m.listHeight())
			for _, idx := range tt.selected {
				m.selected[idx] = true
			}

			_, cmd := m.Update(tt.key)

			if cmd == nil {
				t.Fatal("no action emitted")
			}
			msg, ok := cmd().(action.Msg)
			if !ok || !reflect.DeepEqual(msg.Action, tt.want) {
				t.Errorf("action = %#v, want %#v", msg.Action, tt.want)
			}
			if q.Len() != 4 {
				t.Errorf("the panel changed the queue itself: %d tracks left", q.Len())
			}
		})
	}
}

func TestMoveOutOfQueue_EmitsNothing(t *testing.T) {
	q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"))
	m := New(testService(t, q))
	m.SetSize(60, 10)
	m.SetFocused(true)

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftUp}); cmd != nil {
		t.Errorf("moving the first track up emitted %#v", cmd())
	}
}
