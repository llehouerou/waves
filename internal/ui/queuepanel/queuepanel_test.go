package queuepanel

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
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
			markRows(&m, tt.selected...)
			m.list.Cursor().Jump(tt.cursor, q.Len(), m.listHeight())

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

func TestEscape_ClearsSelection(t *testing.T) {
	q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"))
	m := New(testService(t, q))
	m.SetSize(60, 10)
	m.SetFocused(true)
	markRows(&m, 1)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.HasSelection() {
		t.Error("selection after esc, want none")
	}
	if cmd != nil {
		t.Errorf("esc with a selection emitted %#v, want nothing", cmd())
	}
}

func TestEscape_WithNothingSelected_Leaves(t *testing.T) {
	q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"))
	m := New(testService(t, q))
	m.SetSize(60, 10)
	m.SetFocused(true)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("esc with nothing selected emitted nothing, want Leave")
	}
	if got, want := cmd(), ActionMsg(Leave{}); got != want {
		t.Errorf("esc with nothing selected emitted %#v, want %#v", got, want)
	}
}

// newStaleSelectionModel builds a panel whose five marked tracks, the cursor
// on the last, lost the last two to an edit made outside the panel, before
// the panel was told: the queue is A, B, C.
func newStaleSelectionModel(t *testing.T) (Model, playback.Service) {
	t.Helper()
	q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"), testTrack("C", "x"), testTrack("D", "x"), testTrack("E", "x"))
	svc := testService(t, q)
	m := New(svc)
	m.SetSize(60, 10)
	m.SetFocused(true)
	markRows(&m, 0, 1, 2, 3, 4)
	m.list.Cursor().Jump(4, q.Len(), m.listHeight())
	svc.RemoveTracks([]int{3, 4})
	return m, svc
}

func TestStaleSelection_NamesOnlyTracksOfTheQueue(t *testing.T) {
	runes := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	m, _ := newStaleSelectionModel(t)

	if view := stripANSI(m.View()); !strings.Contains(view, "[3 selected]") {
		t.Errorf("header should show '[3 selected]', got: %s", view)
	}

	m, cmd := m.Update(runes("d"))
	if cmd == nil {
		t.Fatal("d emitted nothing")
	}
	if msg, ok := cmd().(action.Msg); !ok || !reflect.DeepEqual(msg.Action, RemoveTracks{Indices: []int{0, 1, 2}}) {
		t.Errorf("d emitted %#v, want RemoveTracks [0 1 2]", msg)
	}
	if pos := m.list.Cursor().Pos(); pos < 0 {
		t.Fatalf("cursor at %d after d", pos)
	}

	for _, key := range []tea.KeyMsg{runes("F"), {Type: tea.KeyCtrlA}, runes("L")} {
		m.Update(key) // must not panic
	}

	m, _ = m.Update(runes("x"))
	_, cmd = m.Update(runes("D"))
	if cmd == nil {
		t.Fatal("D emitted nothing")
	}
	// The cursor is back on the first track: D keeps it and removes the rest.
	if msg, ok := cmd().(action.Msg); !ok || !reflect.DeepEqual(msg.Action, RemoveTracks{Indices: []int{1, 2}}) {
		t.Errorf("x then D emitted %#v, want RemoveTracks [1 2]", msg)
	}
}

// A mark whose track left the queue is dropped for good: an undo that brings
// the track back does not bring the mark back.
func TestQueueChanged_DropsTheMarksOfTracksThatLeftTheQueue(t *testing.T) {
	m, svc := newStaleSelectionModel(t)

	m.QueueChanged()
	if !svc.Undo() {
		t.Fatal("nothing to undo")
	}

	view := stripANSI(m.View())
	if !strings.Contains(view, "[3 selected]") {
		t.Errorf("header should show '[3 selected]', got: %s", view)
	}
	for _, title := range []string{"D", "E"} {
		if line := testutil.FindLine(view, " "+title+" "); strings.Contains(line, "●") {
			t.Errorf("%s is marked again after the undo: %q", title, line)
		}
	}
}

// J and K leave the moved tracks marked, once the queue holds the move.
func TestMoveSelection_LeavesTheMovedTracksMarked(t *testing.T) {
	q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"), testTrack("C", "x"))
	svc := testService(t, q)
	m := New(svc)
	m.SetSize(60, 10)
	m.SetFocused(true)
	markRows(&m, 0)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	msg, ok := cmd().(action.Msg)
	move, isMove := msg.Action.(MoveTracks)
	if !ok || !isMove {
		t.Fatalf("J emitted %#v, want MoveTracks", msg)
	}
	svc.MoveTracks(move.Indices, move.Delta)
	m.QueueChanged()

	view := stripANSI(m.View())
	if line := testutil.FindLine(view, " A "); !strings.Contains(line, "●") {
		t.Errorf("A moved down is no longer marked: %q", line)
	}
	if line := testutil.FindLine(view, " B "); strings.Contains(line, "●") {
		t.Errorf("B is marked: %q", line)
	}
}

// One cursor key moves the queue cursor by exactly one line: the list moves
// it, and the action the key resolves to in the queue adds no second move.
func TestCursorKeys_MoveOneLine(t *testing.T) {
	runes := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	tests := []struct {
		name     string
		down, up tea.KeyMsg
	}{
		{"j and k", runes("j"), runes("k")},
		{"down and up", tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyUp}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := newTestQueue(testTrack("A", "x"), testTrack("B", "x"), testTrack("C", "x"), testTrack("D", "x"))
			m := New(testService(t, q))
			m.SetSize(60, 10)
			m.SetFocused(true)
			m.list.Cursor().Jump(1, q.Len(), m.listHeight())

			m, _ = m.Update(tt.down)
			if pos := m.list.Cursor().Pos(); pos != 2 {
				t.Errorf("cursor after %s = %d, want 2", tt.down, pos)
			}
			m, _ = m.Update(tt.up)
			if pos := m.list.Cursor().Pos(); pos != 1 {
				t.Errorf("cursor after %s = %d, want 1", tt.up, pos)
			}
		})
	}
}
