// internal/app/persistence.go
package app

import (
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
)

// SaveNavigationState persists the current navigation state.
func (m *Model) SaveNavigationState() {
	m.StateMgr.SaveNavigation(state.NavigationState{
		CurrentPath:         m.FileNavigator.CurrentPath(),
		SelectedName:        m.FileNavigator.SelectedName(),
		ViewMode:            string(m.ViewMode),
		LibrarySelectedID:   m.LibraryNavigator.SelectedID(),
		PlaylistsSelectedID: m.PlaylistNavigator.SelectedID(),
	})
}

// SaveQueueState persists the current queue state.
func (m *Model) SaveQueueState() {
	tracks := m.Playback.Queue().Tracks()
	queueTracks := make([]state.QueueTrack, len(tracks))
	for i, t := range tracks {
		queueTracks[i] = state.QueueTrack{
			TrackID:     t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
	}
	_ = m.StateMgr.SaveQueue(state.QueueState{
		CurrentIndex: m.Playback.Queue().CurrentIndex(),
		RepeatMode:   int(m.Playback.Queue().RepeatMode()),
		Shuffle:      m.Playback.Queue().Shuffle(),
		Tracks:       queueTracks,
	})
}

// TracksToQueueTracks converts playlist tracks to state queue tracks.
func TracksToQueueTracks(tracks []playlist.Track) []state.QueueTrack {
	result := make([]state.QueueTrack, len(tracks))
	for i, t := range tracks {
		result[i] = state.QueueTrack{
			TrackID:     t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
	}
	return result
}
