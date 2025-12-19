// internal/app/update_playback.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/lastfm"
)

// handlePlaybackMsg routes playback-related messages.
func (m Model) handlePlaybackMsg(msg PlaybackMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TrackFinishedMsg:
		return m.handleTrackFinished()
	case TrackSkipTimeoutMsg:
		return m.handleTrackSkipTimeout(msg)
	case TickMsg:
		if m.Playback.IsPlaying() {
			cmds := []tea.Cmd{TickCmd()}
			if cmd := m.checkScrobbleThreshold(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if cmd := m.checkRadioFillNearEnd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
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
			// PlayTrack already handles radio fill when starting last track
			return m, tea.Batch(cmd, m.WatchTrackFinished())
		}
		return m, m.WatchTrackFinished()
	}

	// No next track - stop playback
	// Note: Radio fill is triggered when the last track STARTS (in PlayTrack),
	// so by now the queue should already have new tracks if radio mode is active.
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

// checkScrobbleThreshold checks if the current track has been played long enough to scrobble.
// Last.fm rules: scrobble after 50% of duration OR 4 minutes, whichever comes first.
// Track must be at least 30 seconds long.
func (m *Model) checkScrobbleThreshold() tea.Cmd {
	if m.ScrobbleState == nil || m.ScrobbleState.Scrobbled || !m.isLastfmLinked() {
		return nil
	}

	position := m.Playback.Position()
	duration := m.Playback.Duration()

	// Track must be at least 30 seconds
	if duration < 30*time.Second {
		return nil
	}

	// Scrobble threshold: min(50% of duration, 4 minutes)
	threshold := duration / 2
	fourMinutes := 4 * time.Minute
	if fourMinutes < threshold {
		threshold = fourMinutes
	}

	if position >= threshold {
		m.ScrobbleState.Scrobbled = true
		track := m.buildScrobbleTrack()
		if track != nil {
			return lastfm.ScrobbleCmd(m.Lastfm, *track, m.ScrobbleState.TrackPath)
		}
	}

	return nil
}
