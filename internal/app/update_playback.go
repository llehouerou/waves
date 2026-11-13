// internal/app/update_playback.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/albumart"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// handlePlaybackMsg routes playback-related messages.
func (m Model) handlePlaybackMsg(msg PlaybackMessage) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceStateChangedMsg:
		return m.handleServiceStateChanged(msg)
	case ServiceTrackChangedMsg:
		return m.handleServiceTrackChanged(msg)
	case ServiceErrorMsg:
		return m.handleServiceError(msg)
	case ServiceFailedMsg:
		// An slskd that is down is no reason to interrupt the user: the
		// downloads view shows what was last known.
		if msg.Op != errmsg.OpDownloadRefresh {
			m.Popups.ShowOpError(msg.Op, msg.Err)
		}
		return m, m.WatchServiceEvents()
	case ServiceClosedMsg:
		return m, nil // Core closed, nothing to do
	case ServiceQueueChangedMsg:
		// Schedule album art update for next tick to ensure the playback
		// service has fully updated its state
		cmds := []tea.Cmd{m.WatchServiceEvents()}
		if m.Core.Playback().IsPlaying() {
			cmds = append(cmds, func() tea.Msg { return AlbumArtUpdateMsg{} })
		}
		return m, tea.Batch(cmds...)
	case AlbumArtUpdateMsg:
		// Deferred album art update - service state should be stable now. The
		// load itself runs as a command, off the UI goroutine.
		cmd := m.albumArtCmdIfNeeded()
		return m, cmd
	case AlbumArtLoadedMsg:
		if m.AlbumArt != nil {
			m.albumArtPendingTransmit = m.AlbumArt.Commit(msg.Path, msg.PNG)
		}
		return m, nil
	case LyricsUpdateMsg:
		// Deferred lyrics update - track info should be ready now
		if lyr := m.Popups.Lyrics(); lyr != nil {
			track := m.Core.Playback().CurrentTrack()
			if track != nil {
				return m, lyr.SetTrack(
					track.Path, track.Artist, track.Title, track.Album,
					m.Core.Playback().Duration(),
				)
			}
		}
		return m, nil
	case ServiceModeChangedMsg:
		return m, m.WatchServiceEvents()
	case ServicePositionChangedMsg:
		// Drained only: position updates come from TickMsg.
		return m, m.WatchServiceEvents()
	case TrackSkipTimeoutMsg:
		return m.handleTrackSkipTimeout(msg)
	case TickMsg:
		// Drop ticks from a stale chain: only the current generation may
		// re-arm, so at most one chain stays alive (issue #28).
		if msg.Gen != m.tickGen {
			return m, nil
		}
		if !m.Core.Playback().IsPlaying() {
			// Playback no longer active (stopped/paused): end this chain.
			m.clearRunning()
			return m, nil
		}
		// The tick only moves what the TUI shows: the core checks the scrobble
		// threshold and the end of the queue on its own clock.
		if lyr := m.Popups.Lyrics(); lyr != nil {
			lyr.SetPosition(m.Core.Playback().Position())
		}
		return m, TickCmd(m.tickGen)
	}
	return m, nil
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
func (m Model) handleServiceStateChanged(_ ServiceStateChangedMsg) (tea.Model, tea.Cmd) {
	// Update UI to reflect new state
	m.ResizeComponents()

	if m.Core.Playback().IsPlaying() {
		return m.handlePlaybackStarted()
	}

	if m.Core.Playback().IsStopped() {
		m.handlePlaybackStopped()
	} else if m.Core.Playback().IsPaused() {
		// End the tick chain immediately so we do not emit another
		// Update+View while paused (issue #28). Resume reseeds via
		// ensureTickRunning from handlePlaybackStarted.
		m.stopTick()
	}

	return m, m.WatchServiceEvents()
}

// handlePlaybackStarted handles the transition to playing state.
func (m Model) handlePlaybackStarted() (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		m.ensureTickRunning(),
		m.WatchServiceEvents(),
		// Schedule album art update for next tick to ensure service state is stable
		func() tea.Msg { return AlbumArtUpdateMsg{} },
	)
}

// handlePlaybackStopped cleans up when playback stops.
func (m *Model) handlePlaybackStopped() {
	m.stopTick()
	if m.AlbumArt != nil {
		m.albumArtPendingTransmit = m.AlbumArt.Clear()
	}
	if m.Popups.Lyrics() != nil {
		m.Popups.Hide(popupctl.Lyrics)
	}
}

// handleServiceTrackChanged handles track changes from the service.
// This handles what the TUI shows for track CHANGES (not first play):
// - Album art updates
// - Lyrics updates
//
// Note: First play (stopped → playing) is handled by handleServiceStateChanged
// since TrackChange is not emitted for the initial play.
// See core.TrackChanged for when this event is emitted.
func (m Model) handleServiceTrackChanged(_ ServiceTrackChangedMsg) (tea.Model, tea.Cmd) {
	// Update UI to reflect new track
	m.Layout.QueuePanel().SyncCursor()
	m.ResizeComponents()

	cmds := []tea.Cmd{m.WatchServiceEvents()}
	// Schedule lyrics update if popup is visible (deferred to ensure track info is ready)
	if m.Popups.Lyrics() != nil {
		cmds = append(cmds, func() tea.Msg { return LyricsUpdateMsg{} })
	}

	// Invalidate album art cache so next update will re-prepare
	if m.AlbumArt != nil {
		m.AlbumArt.InvalidateCache()
	}
	// Schedule album art update for next tick
	cmds = append(cmds, func() tea.Msg { return AlbumArtUpdateMsg{} })

	// Keep ticking if playing; no-op if a chain is already alive (issue #28).
	if m.Core.Playback().IsPlaying() {
		cmds = append(cmds, m.ensureTickRunning())
	}

	return m, tea.Batch(cmds...)
}

// albumArtLoadCmd returns a command that loads the cover off the UI goroutine.
// Reading the cover out of an audio file is slow enough to freeze the interface
// when the library is on a network mount, so it must not happen in Update
// (issue #49).
func albumArtLoadCmd(art *albumart.Renderer, path string) tea.Cmd {
	return func() tea.Msg {
		return AlbumArtLoadedMsg{Path: path, PNG: art.LoadTrack(path)}
	}
}

// albumArtCmdIfNeeded returns a command to load album art when it is missing for
// the current track, or nil when there is nothing to do.
func (m *Model) albumArtCmdIfNeeded() tea.Cmd {
	if m.AlbumArt == nil {
		return nil
	}
	track := m.Core.Playback().CurrentTrack()
	if track == nil {
		return nil
	}
	if track.Path != m.AlbumArt.CurrentPath() {
		m.AlbumArt.InvalidateCache()
	}
	m.AlbumArt.SetSize(playerbar.AlbumArtWidth, playerbar.AlbumArtHeight)
	if !m.AlbumArt.NeedsLoad(track.Path) {
		return nil
	}
	return albumArtLoadCmd(m.AlbumArt, track.Path)
}

// handleServiceError handles errors from the playback service.
func (m Model) handleServiceError(msg ServiceErrorMsg) (tea.Model, tea.Cmd) {
	// Display error to user
	errMsg := "Playback error"
	if msg.Path != "" {
		errMsg = "Failed to play: " + msg.Path
	}
	if msg.Err != nil {
		errMsg += ": " + msg.Err.Error()
	}
	m.Popups.ShowError(errMsg)

	return m, m.WatchServiceEvents()
}
