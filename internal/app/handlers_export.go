// internal/app/handlers_export.go
package app

import (
	"path/filepath"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/library"
)

// handleExportKey handles the 'e' key to open the export popup.
func (m *Model) handleExportKey(key string) handler.Result {
	action := m.Keys.Resolve(key)
	if action != keymap.ActionExport {
		return handler.NotHandled
	}

	// Check if we're in a context that supports export
	if !m.canExport() {
		return handler.NotHandled
	}

	// Collect tracks to export
	tracks, albumName := m.collectExportTracks()
	if len(tracks) == 0 {
		return handler.NotHandled
	}

	// Show export popup
	cmd := m.Popups.ShowExport(m.ExportRepo)
	if exp := m.Popups.Export(); exp != nil {
		exp.SetTracks(tracks, albumName)
	}
	return handler.Handled(cmd)
}

// canExport returns true if export is available in the current context.
func (m *Model) canExport() bool {
	// Can export from library navigator
	if m.Navigation.ViewMode() == ViewLibrary && m.Navigation.IsNavigatorFocused() {
		return true
	}
	// Can export from queue
	if m.Navigation.IsQueueFocused() {
		return true
	}
	return false
}

// collectExportTracks collects tracks for export based on current selection.
//
//nolint:nestif // Complex but clear structure for collecting tracks from different sources
func (m *Model) collectExportTracks() (tracks []export.Track, albumName string) {
	var trackIDs []int64

	if m.Navigation.IsQueueFocused() {
		// Export from queue
		queueTracks := m.PlaybackService.QueueTracks()
		for _, t := range queueTracks {
			trackIDs = append(trackIDs, t.ID)
		}
		albumName = "Queue"
	} else if m.Navigation.ViewMode() == ViewLibrary {
		// Export from library navigator
		if m.Navigation.LibrarySubMode() == LibraryModeAlbum {
			// Album view mode
			album := m.Navigation.AlbumView().SelectedAlbum()
			if album != nil {
				ids, err := m.Library.AlbumTrackIDs(album.AlbumArtist, album.Album)
				if err == nil {
					trackIDs = ids
				}
				albumName = album.Album
			}
		} else {
			// Miller columns mode
			selected := m.Navigation.LibraryNav().Selected()
			if selected != nil {
				trackIDs = m.collectTrackIDsFromNode(selected)
				albumName = selected.DisplayName()
			}
		}
	}

	if len(trackIDs) == 0 {
		return nil, ""
	}

	// Convert to export.Track
	tracks = make([]export.Track, 0, len(trackIDs))
	discTotals := m.countDiscs(trackIDs)

	for _, id := range trackIDs {
		libTrack, err := m.Library.TrackByID(id)
		if err != nil {
			continue
		}
		tracks = append(tracks, export.Track{
			ID:        id,
			SrcPath:   libTrack.Path,
			Artist:    libTrack.AlbumArtist,
			Album:     libTrack.Album,
			Title:     libTrack.Title,
			TrackNum:  libTrack.TrackNumber,
			DiscNum:   libTrack.DiscNumber,
			DiscTotal: discTotals[libTrack.Album],
			Extension: filepath.Ext(libTrack.Path),
		})
	}

	return tracks, albumName
}

// collectTrackIDsFromNode collects track IDs from a library node.
func (m *Model) collectTrackIDsFromNode(node *library.Node) []int64 {
	switch node.Level() {
	case library.LevelRoot:
		return nil
	case library.LevelArtist:
		tracks, err := m.Library.ArtistTracks(node.Artist())
		if err != nil {
			return nil
		}
		ids := make([]int64, len(tracks))
		for i := range tracks {
			ids[i] = tracks[i].ID
		}
		return ids
	case library.LevelAlbum:
		ids, _ := m.Library.AlbumTrackIDs(node.Artist(), node.Album())
		return ids
	case library.LevelTrack:
		if track := node.Track(); track != nil {
			return []int64{track.ID}
		}
	}
	return nil
}

// countDiscs counts the number of discs per album.
func (m *Model) countDiscs(trackIDs []int64) map[string]int {
	discCounts := make(map[string]int)
	for _, id := range trackIDs {
		track, err := m.Library.TrackByID(id)
		if err != nil {
			continue
		}
		if track.DiscNumber > discCounts[track.Album] {
			discCounts[track.Album] = track.DiscNumber
		}
	}
	return discCounts
}
