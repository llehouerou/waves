// internal/app/update_playback.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/playback"
)

// resetScrobbleState resets scrobble tracking for a new track.
func (m *Model) resetScrobbleState() {
	track := m.Playback.CurrentTrack()
	if track == nil {
		m.ScrobbleState = nil
		return
	}
	m.ScrobbleState = &lastfm.ScrobbleState{
		TrackPath: track.Path,
		StartedAt: time.Now(),
	}
	m.RadioFillTriggered = false
}

// handlePlaybackMsg routes playback-related messages.
func (m Model) handlePlaybackMsg(msg PlaybackMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TrackFinishedMsg:
		// Deprecated: service now handles auto-advance internally
		return m.handleTrackFinished()
	case ServiceStateChangedMsg:
		return m.handleServiceStateChanged(msg)
	case ServiceTrackChangedMsg:
		return m.handleServiceTrackChanged(msg)
	case ServiceClosedMsg:
		return m, nil // Service closed, nothing to do
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

// handleServiceStateChanged handles playback state changes from the service.
func (m Model) handleServiceStateChanged(msg ServiceStateChangedMsg) (tea.Model, tea.Cmd) {
	// Update UI to reflect new state
	m.ResizeComponents()

	// When starting playback (transitioning to playing), reset scrobble and check radio
	if m.Playback.IsPlaying() {
		cmds := []tea.Cmd{TickCmd(), m.WatchServiceEvents()}

		// Reset scrobble state when starting from stopped
		if msg.Previous == int(playback.StateStopped) {
			m.resetScrobbleState()
			if cmd := m.checkRadioFillNearEnd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		return m, tea.Batch(cmds...)
	}

	return m, m.WatchServiceEvents()
}

// handleServiceTrackChanged handles track changes from the service.
func (m Model) handleServiceTrackChanged(_ ServiceTrackChangedMsg) (tea.Model, tea.Cmd) {
	// Update UI to reflect new track
	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()
	m.ResizeComponents()

	// Reset scrobble state for new track
	m.resetScrobbleState()

	// Check if we need to fill radio queue (when starting last track)
	var cmds []tea.Cmd
	cmds = append(cmds, m.WatchServiceEvents())

	if cmd := m.checkRadioFillNearEnd(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Start tick command if playing
	if m.Playback.IsPlaying() {
		cmds = append(cmds, TickCmd())
	}

	return m, tea.Batch(cmds...)
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
