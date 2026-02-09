package similarartists

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// Update handles messages and returns updated model and commands.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case FetchResultMsg:
		return m.handleFetchResult(msg), nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleFetchResult(msg FetchResultMsg) *Model {
	m.loading = false
	if msg.Err != nil {
		m.errorMsg = msg.Err.Error()
		return m
	}
	m.inLibrary = msg.InLibrary
	m.notInLibrary = msg.NotInLibrary
	m.cursor = 0
	return m
}

func (m *Model) handleKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	// Allow closing on error state
	if m.errorMsg != "" || m.loading {
		if msg.String() == "esc" || msg.String() == "q" {
			return m, func() tea.Msg { return ActionMsg(Close{}) }
		}
		return m, nil
	}

	switch msg.String() {
	case "esc", "q":
		return m, func() tea.Msg { return ActionMsg(Close{}) }

	case "j", "down":
		if m.totalItems() > 0 {
			m.cursor = (m.cursor + 1) % m.totalItems()
		}
		return m, nil

	case "k", "up":
		if m.totalItems() > 0 {
			m.cursor = (m.cursor - 1 + m.totalItems()) % m.totalItems()
		}
		return m, nil

	case "enter":
		item := m.selectedItem()
		if item == nil {
			return m, nil
		}
		if item.InLibrary {
			return m, func() tea.Msg { return ActionMsg(GoToArtist{Name: item.Name}) }
		}
		return m, func() tea.Msg { return ActionMsg(OpenDownload{Name: item.Name}) }

	case "d":
		item := m.selectedItem()
		if item == nil {
			return m, nil
		}
		return m, func() tea.Msg { return ActionMsg(OpenDownload{Name: item.Name}) }
	}

	return m, nil
}
