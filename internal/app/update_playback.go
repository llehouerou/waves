// internal/app/update_playback.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/notify"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/ui/albumart"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// resetScrobbleState resets scrobble tracking for a new track.
func (m *Model) resetScrobbleState() {
	track := m.PlaybackService.CurrentTrack()
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
	case ServiceStateChangedMsg:
		return m.handleServiceStateChanged(msg)
	case ServiceTrackChangedMsg:
		return m.handleServiceTrackChanged(msg)
	case ServiceErrorMsg:
		return m.handleServiceError(msg)
	case ServiceClosedMsg:
		return m, nil // Service closed, nothing to do
	case ServiceQueueChangedMsg:
		// Queue changed - schedule album art update for next tick to ensure
		// playback service has fully updated its state
		cmds := []tea.Cmd{m.WatchServiceEvents()}
		if m.PlaybackService.IsPlaying() {
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
	case NowPlayingNotifiedMsg:
		// Remember the id so the next notification replaces this one.
		m.lastNowPlayingID = msg.ID
		return m, nil
	case LyricsUpdateMsg:
		// Deferred lyrics update - track info should be ready now
		if lyr := m.Popups.Lyrics(); lyr != nil {
			track := m.PlaybackService.CurrentTrack()
			if track != nil {
				return m, lyr.SetTrack(
					track.Path, track.Artist, track.Title, track.Album,
					m.PlaybackService.Player().Duration(),
				)
			}
		}
		return m, nil
	case ServiceModeChangedMsg, ServicePositionChangedMsg:
		// These are drained from the subscription channel but handled synchronously in UI.
		// Just re-issue the watch command to continue listening.
		return m, m.WatchServiceEvents()
	case TrackSkipTimeoutMsg:
		return m.handleTrackSkipTimeout(msg)
	case TickMsg:
		// Drop ticks from a stale chain: only the current generation may
		// re-arm, so at most one chain stays alive (issue #28).
		if msg.Gen != m.tickGen {
			return m, nil
		}
		if !m.PlaybackService.IsPlaying() {
			// Playback no longer active (stopped/paused): end this chain.
			m.clearRunning()
			return m, nil
		}
		cmds := []tea.Cmd{TickCmd(m.tickGen)}
		if cmd := m.checkScrobbleThreshold(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.checkRadioFillNearEnd(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Update lyrics position if popup is visible
		if lyr := m.Popups.Lyrics(); lyr != nil {
			lyr.SetPosition(m.PlaybackService.Player().Position())
		}
		return m, tea.Batch(cmds...)
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
func (m Model) handleServiceStateChanged(msg ServiceStateChangedMsg) (tea.Model, tea.Cmd) {
	// Update UI to reflect new state
	m.ResizeComponents()

	if m.PlaybackService.IsPlaying() {
		return m.handlePlaybackStarted(msg.Previous == int(playback.StateStopped))
	}

	if m.PlaybackService.IsStopped() {
		m.handlePlaybackStopped()
	} else if m.PlaybackService.IsPaused() {
		// End the tick chain immediately so we do not emit another
		// Update+View while paused (issue #28). Resume reseeds via
		// ensureTickRunning from handlePlaybackStarted.
		m.stopTick()
	}

	return m, m.WatchServiceEvents()
}

// handlePlaybackStarted handles the transition to playing state.
// fromStopped indicates if we're starting from a stopped state (first play).
func (m Model) handlePlaybackStarted(fromStopped bool) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.ensureTickRunning(), m.WatchServiceEvents()}

	// When starting from stopped, handle first track setup
	// (TrackChange is not emitted for first play, only for track changes)
	if fromStopped {
		m.resetScrobbleState()

		// Send notification for first track
		if cmd := m.nowPlayingCmd(m.PlaybackService.CurrentTrack()); cmd != nil {
			cmds = append(cmds, cmd)
		}

		// Trigger radio fill if starting the last track
		if cmd := m.triggerRadioFill(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Schedule album art update for next tick to ensure service state is stable
	cmds = append(cmds, func() tea.Msg { return AlbumArtUpdateMsg{} })

	return m, tea.Batch(cmds...)
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
// This handles track-related side effects for track CHANGES (not first play):
// - Scrobble state reset
// - Desktop notifications
// - Album art updates
// - Lyrics updates
// - Radio fill checks
//
// Note: First play (stopped → playing) is handled by handleServiceStateChanged
// since TrackChange is not emitted for the initial play.
// See playback.TrackChange for when this event is emitted.
func (m Model) handleServiceTrackChanged(_ ServiceTrackChangedMsg) (tea.Model, tea.Cmd) {
	// Update UI to reflect new track
	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()
	m.ResizeComponents()

	// Reset scrobble state for new track
	m.resetScrobbleState()

	cmds := []tea.Cmd{m.WatchServiceEvents()}

	// Send desktop notification, off the UI goroutine
	if cmd := m.nowPlayingCmd(m.PlaybackService.CurrentTrack()); cmd != nil {
		cmds = append(cmds, cmd)
	}

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

	// Trigger radio fill if now on the last track (pre-fetch next tracks)
	if cmd := m.triggerRadioFill(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Keep ticking if playing; no-op if a chain is already alive (issue #28).
	if m.PlaybackService.IsPlaying() {
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
	track := m.PlaybackService.CurrentTrack()
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

// nowPlayingCmd returns a command that sends the "now playing" notification off
// the UI goroutine. Finding the artwork stats the track's directory and reads the
// audio file, which on a network library is exactly the freeze issue #49 is
// about, and the D-Bus call itself can block too.
func (m *Model) nowPlayingCmd(track *playback.Track) tea.Cmd {
	if m.notifier == nil || track == nil {
		return nil
	}
	cfg := m.notificationsConfig
	if cfg.Enabled == nil || !*cfg.Enabled {
		return nil
	}
	if cfg.NowPlaying == nil || !*cfg.NowPlaying {
		return nil
	}

	notifier := m.notifier
	n := notify.Notification{
		Title:      track.Title,
		Body:       track.Artist + " · " + track.Album,
		Timeout:    cfg.Timeout,
		ReplacesID: m.lastNowPlayingID,
		Urgency:    notify.UrgencyLow,
	}
	withArt := cfg.ShowAlbumArt != nil && *cfg.ShowAlbumArt
	path := track.Path

	return func() tea.Msg {
		if withArt {
			if artPath := notify.FindAlbumArtPath(path); artPath != "" {
				n.Icon = "file://" + artPath
			}
		}
		id, _ := notifier.Notify(n) //nolint:errcheck // a failed notification is not worth surfacing
		return NowPlayingNotifiedMsg{ID: id}
	}
}

// sendDownloadCompleteNotification sends a notification when a download finishes.
func (m *Model) sendDownloadCompleteNotification(artist, album string) {
	if m.notifier == nil {
		return
	}
	cfg := m.notificationsConfig
	if cfg.Enabled == nil || !*cfg.Enabled {
		return
	}
	if cfg.Downloads == nil || !*cfg.Downloads {
		return
	}

	n := notify.Notification{
		Title:   "Download Complete",
		Body:    artist + " - " + album,
		Timeout: cfg.Timeout,
		Urgency: notify.UrgencyNormal,
	}

	_, _ = m.notifier.Notify(n)
}

// checkScrobbleThreshold checks if the current track has been played long enough to scrobble.
// Last.fm rules: scrobble after 50% of duration OR 4 minutes, whichever comes first.
// Track must be at least 30 seconds long.
func (m *Model) checkScrobbleThreshold() tea.Cmd {
	if m.ScrobbleState == nil || m.ScrobbleState.Scrobbled || !m.isLastfmLinked() {
		return nil
	}

	position := m.PlaybackService.Position()
	duration := m.PlaybackService.Duration()

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
