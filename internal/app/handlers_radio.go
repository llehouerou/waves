// internal/app/handlers_radio.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/radio"
)

// RadioFillCmd returns a command that fills the queue with radio tracks.
// favorites is a map of track IDs that are in the user's Favorites playlist.
func RadioFillCmd(r *radio.Radio, seedArtist string, favorites map[int64]bool) tea.Cmd {
	return func() tea.Msg {
		result := r.Fill(seedArtist, favorites)

		// Convert to message format
		tracks := make([]struct {
			ID          int64
			Path        string
			Title       string
			Artist      string
			Album       string
			TrackNumber int
		}, 0, len(result.Tracks))

		for _, t := range result.Tracks {
			tracks = append(tracks, struct {
				ID          int64
				Path        string
				Title       string
				Artist      string
				Album       string
				TrackNumber int
			}{
				ID:          t.ID,
				Path:        t.Path,
				Title:       t.Title,
				Artist:      t.Artist,
				Album:       t.Album,
				TrackNumber: t.TrackNumber,
			})
		}

		return RadioFillResultMsg{
			Tracks:  tracks,
			Message: result.Message,
			Err:     result.Err,
		}
	}
}

// handleRadioFillResult handles the result of filling the queue from radio.
func (m *Model) handleRadioFillResult(msg RadioFillResultMsg) {
	if msg.Err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpRadioFill, msg.Err))
		return
	}

	// msg.Message contains transient info like "No related tracks found"
	// We don't show it as it's informational only

	if len(msg.Tracks) == 0 {
		return
	}

	// Convert to playback tracks and add to queue
	tracks := make([]playback.Track, len(msg.Tracks))
	for i, t := range msg.Tracks {
		tracks[i] = playback.Track{
			ID:          t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
	}

	m.PlaybackService.AddTracks(tracks...)
	m.Layout.QueuePanel().SyncCursor()
	m.SaveQueueState()
	// Clear preloaded track in case queue position changed
	m.PlaybackService.Player().ClearPreload()

	// Update seed to the last added track's artist for the "moving seed" behavior
	if len(tracks) > 0 {
		lastTrack := tracks[len(tracks)-1]
		m.Radio.SetSeed(lastTrack.Artist)

		// Add to recently played for decay scoring and artist variety
		for _, t := range tracks {
			m.Radio.AddToRecentlyPlayed(t.Path, t.Artist)
		}
	}
}

// shouldFillRadio checks if radio should fill the queue.
// Called when a track starts playing to pre-fetch more tracks.
func (m *Model) shouldFillRadio() bool {
	// Only active when in RepeatRadio mode
	if m.PlaybackService.RepeatMode() != playback.RepeatRadio {
		return false
	}

	if m.Radio == nil {
		return false
	}

	if m.PlaybackService.QueueIsEmpty() {
		return false
	}

	// Fill when starting the last track (pre-fetch before it ends)
	return m.PlaybackService.QueueCurrentIndex() >= m.PlaybackService.QueueLen()-1
}

// shouldFillRadioNearEnd checks if radio should fill because track is near end with no next.
// This handles the case where tracks were deleted/moved and current track became the last.
func (m *Model) shouldFillRadioNearEnd() bool {
	// Only active when in RepeatRadio mode
	if m.PlaybackService.RepeatMode() != playback.RepeatRadio {
		return false
	}

	if m.Radio == nil {
		return false
	}

	// Don't trigger if already triggered for this track
	if m.RadioFillTriggered {
		return false
	}

	// Check if there's no next track
	if m.PlaybackService.QueueHasNext() {
		return false
	}

	// Check if we're within 15 seconds of the end
	duration := m.PlaybackService.Duration()
	position := m.PlaybackService.Position()

	// Need valid duration and position
	if duration <= 0 || position <= 0 {
		return false
	}

	remaining := duration - position
	return remaining <= 15*time.Second
}

// triggerRadioFill triggers radio fill if conditions are met.
// Called at track start - does NOT set RadioFillTriggered so near-end check can still work
// if user modifies the queue during playback.
func (m *Model) triggerRadioFill() tea.Cmd {
	if !m.shouldFillRadio() {
		return nil
	}

	seed := m.Radio.CurrentSeed()
	if seed == "" {
		// Use current track's artist as seed
		if track := m.PlaybackService.CurrentTrack(); track != nil {
			seed = track.Artist
			m.Radio.SetSeed(seed)
		}
	}

	if seed == "" {
		return nil
	}

	// Get favorites for scoring boost
	favorites, _ := m.Playlists.FavoriteTrackIDs()

	return RadioFillCmd(m.Radio, seed, favorites)
}

// checkRadioFillNearEnd checks if we should fill the queue because track is near end.
// This handles the case where tracks were deleted/moved during playback.
func (m *Model) checkRadioFillNearEnd() tea.Cmd {
	if !m.shouldFillRadioNearEnd() {
		return nil
	}

	seed := m.Radio.CurrentSeed()
	if seed == "" {
		// Use current track's artist as seed
		if track := m.PlaybackService.CurrentTrack(); track != nil {
			seed = track.Artist
			m.Radio.SetSeed(seed)
		}
	}

	if seed == "" {
		return nil
	}

	// Get favorites for scoring boost
	favorites, _ := m.Playlists.FavoriteTrackIDs()

	m.RadioFillTriggered = true
	return RadioFillCmd(m.Radio, seed, favorites)
}
