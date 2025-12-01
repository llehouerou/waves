// internal/app/queue.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playlist"
)

// HandleQueueAction performs the specified queue action on the selected item.
func (m *Model) HandleQueueAction(action QueueAction) tea.Cmd {
	var tracks []playlist.Track
	var err error

	switch m.ViewMode {
	case ViewFileBrowser:
		selected := m.FileNavigator.Selected()
		if selected == nil {
			return nil
		}
		tracks, err = playlist.CollectFromFileNode(*selected)
	case ViewLibrary:
		selected := m.LibraryNavigator.Selected()
		if selected == nil {
			return nil
		}
		tracks, err = playlist.CollectFromLibraryNode(m.Library, *selected)
	}

	if err != nil {
		m.ErrorMsg = err.Error()
		return nil
	}

	if len(tracks) == 0 {
		return nil
	}

	var trackToPlay *playlist.Track

	switch action {
	case QueueAddAndPlay:
		trackToPlay = m.Queue.AddAndPlay(tracks...)
	case QueueAdd:
		m.Queue.Add(tracks...)
	case QueueReplace:
		trackToPlay = m.Queue.Replace(tracks...)
	}

	m.SaveQueueState()
	m.QueuePanel.SyncCursor()

	if trackToPlay != nil {
		if err := m.Player.Play(trackToPlay.Path); err != nil {
			m.ErrorMsg = err.Error()
			return nil
		}

		m.ResizeComponents()
		return TickCmd()
	}

	return nil
}

// HandleAddAlbumAndPlay replaces queue with full album, plays from selected track.
func (m *Model) HandleAddAlbumAndPlay() tea.Cmd {
	selected := m.LibraryNavigator.Selected()
	if selected == nil {
		return nil
	}

	tracks, selectedIdx, err := playlist.CollectAlbumFromTrack(m.Library, *selected)
	if err != nil {
		m.ErrorMsg = err.Error()
		return nil
	}

	if len(tracks) == 0 {
		return nil
	}

	m.Queue.Replace(tracks...)
	trackToPlay := m.Queue.JumpTo(selectedIdx)

	m.SaveQueueState()
	m.QueuePanel.SyncCursor()

	if trackToPlay != nil {
		if err := m.Player.Play(trackToPlay.Path); err != nil {
			m.ErrorMsg = err.Error()
			return nil
		}
		m.ResizeComponents()
		return TickCmd()
	}

	return nil
}
