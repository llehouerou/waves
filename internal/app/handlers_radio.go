// internal/app/handlers_radio.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/radio"
)

// RadioFillCmd returns a command that fills the queue with radio tracks.
func RadioFillCmd(r *radio.Radio, seedArtist string) tea.Cmd {
	return func() tea.Msg {
		result := r.Fill(seedArtist)

		// Convert to message format
		var tracks []struct {
			ID          int64
			Path        string
			Title       string
			Artist      string
			Album       string
			TrackNumber int
		}

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

	// Convert to playlist tracks and add to queue
	tracks := make([]playlist.Track, len(msg.Tracks))
	for i, t := range msg.Tracks {
		tracks[i] = playlist.Track{
			ID:          t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
	}

	m.Playback.Queue().Add(tracks...)
	m.Layout.QueuePanel().SyncCursor()
	m.SaveQueueState()

	// Update seed to the last added track's artist for the "moving seed" behavior
	if len(tracks) > 0 {
		lastTrack := tracks[len(tracks)-1]
		m.Radio.SetSeed(lastTrack.Artist)

		// Add to recently played for decay scoring
		for _, t := range tracks {
			m.Radio.AddToRecentlyPlayed(t.Path)
		}
	}
}

// shouldFillRadio checks if radio should fill the queue.
// Called when a track starts playing to pre-fetch more tracks.
func (m *Model) shouldFillRadio() bool {
	queue := m.Playback.Queue()

	// Only active when in RepeatRadio mode
	if queue.RepeatMode() != playlist.RepeatRadio {
		return false
	}

	if m.Radio == nil {
		return false
	}

	if queue.IsEmpty() {
		return false
	}

	// Fill when starting the last track (pre-fetch before it ends)
	return queue.CurrentIndex() >= queue.Len()-1
}

// triggerRadioFill triggers radio fill if conditions are met.
func (m *Model) triggerRadioFill() tea.Cmd {
	if !m.shouldFillRadio() {
		return nil
	}

	seed := m.Radio.CurrentSeed()
	if seed == "" {
		// Use current track's artist as seed
		if track := m.Playback.CurrentTrack(); track != nil {
			seed = track.Artist
			m.Radio.SetSeed(seed)
		}
	}

	if seed == "" {
		return nil
	}

	return RadioFillCmd(m.Radio, seed)
}
