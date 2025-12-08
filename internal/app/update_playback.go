// internal/app/update_playback.go
package app

import tea "github.com/charmbracelet/bubbletea"

// handlePlaybackMsg routes playback-related messages.
func (m Model) handlePlaybackMsg(msg PlaybackMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TrackFinishedMsg:
		return m.handleTrackFinished()
	case TrackSkipTimeoutMsg:
		return m.handleTrackSkipTimeout(msg)
	case TickMsg:
		if m.Playback.IsPlaying() {
			return m, TickCmd()
		}
	}
	return m, nil
}

// handleTrackFinished advances to the next track or stops playback.
func (m Model) handleTrackFinished() (tea.Model, tea.Cmd) {
	if m.Playback.Queue().HasNext() {
		next := m.Playback.Queue().Next()
		m.SaveQueueState()
		m.Layout.QueuePanel().SyncCursor()
		cmd := m.PlayTrack(next.Path)
		if cmd != nil {
			return m, tea.Batch(cmd, m.WatchTrackFinished())
		}
		return m, m.WatchTrackFinished()
	}
	m.Playback.Stop()
	m.ResizeComponents()
	return m, m.WatchTrackFinished()
}

// handleTrackSkipTimeout handles the debounced track skip after rapid key presses.
func (m Model) handleTrackSkipTimeout(msg TrackSkipTimeoutMsg) (tea.Model, tea.Cmd) {
	if msg.Version == m.TrackSkipVersion {
		cmd := m.PlayTrackAtIndex(m.PendingTrackIdx)
		return m, cmd
	}
	return m, nil
}
