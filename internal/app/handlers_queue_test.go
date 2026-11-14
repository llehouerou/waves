package app

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
)

func queuePaths(m Model) []string {
	tracks := m.Core.Playback().QueueTracks()
	paths := make([]string, 0, len(tracks))
	for _, track := range tracks {
		paths = append(paths, track.Path)
	}
	return paths
}

func assertQueue(t *testing.T, m Model, want ...string) {
	t.Helper()
	if got := queuePaths(m); !slices.Equal(got, want) {
		t.Errorf("queue = %v, want %v", got, want)
	}
}

func TestQueueHistoryKeys_CtrlZUndoesAndCtrlYRedoes(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/a"}, core.Track{Path: "/b"}, core.Track{Path: "/c"})
	m.Core.Playback().RemoveTracks([]int{0})
	assertQueue(t, m, "/b", "/c")

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlZ})
	assertQueue(t, m, "/a", "/b", "/c")

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlY})
	assertQueue(t, m, "/b", "/c")
}

func TestQueueHistoryKeys_CtrlYWithNothingToRedoChangesNothing(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/a"}, core.Track{Path: "/b"})

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlY})

	assertQueue(t, m, "/a", "/b")
}
