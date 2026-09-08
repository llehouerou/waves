package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/player"
)

// A playback tick only moves the player bar, so Update must not rebuild the
// header/navigator/queue block for it; any other message must (issue #54).
func TestUpdate_PanelsReusedAcrossTicks(t *testing.T) {
	m := newIntegrationTestModel()
	m.loadingState = loadingDone
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	mock, _ := m.PlaybackService.Player().(*player.Mock)
	mock.SetState(player.Playing)

	if !strings.Contains(m.View(), "Queue") {
		t.Fatal("precondition: queue panel should be visible")
	}

	// Hide the queue behind Update's back: a tick must not notice.
	m.Layout.HideQueue()
	m, _ = updateModel(t, m, TickMsg{Gen: m.tickGen})
	if !strings.Contains(m.View(), "Queue") {
		t.Error("tick should reuse the previously rendered panels")
	}

	// Any other message re-renders.
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if strings.Contains(m.View(), "Queue") {
		t.Error("non-tick message should re-render the panels")
	}
}
