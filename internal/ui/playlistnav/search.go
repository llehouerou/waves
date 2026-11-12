package playlistnav

import (
	"fmt"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/icons"
)

// SearchItem is a playlist in the add-to-playlist picker.
type SearchItem struct {
	core.PlaylistSearchItem
}

// DeepSearchItem is a playlist or one of its tracks in deep search results.
type DeepSearchItem struct {
	core.PlaylistDeepSearchItem
}

// FilterValue returns the searchable text for filtering.
func (s SearchItem) FilterValue() string {
	if s.FolderPath != "" {
		return s.Name + " " + s.FolderPath
	}
	return s.Name
}

// DisplayText returns the display string for search results.
func (s SearchItem) DisplayText() string {
	if s.FolderPath != "" {
		return s.Name + " (" + s.FolderPath + ")"
	}
	return s.Name
}

// LeftColumn returns the playlist name for two-column display.
func (s SearchItem) LeftColumn() string {
	return s.Name
}

// RightColumn returns the folder path for two-column display.
func (s SearchItem) RightColumn() string {
	return s.FolderPath
}

// FilterValue returns the searchable text for filtering.
func (d DeepSearchItem) FilterValue() string {
	if d.IsPlaylist() {
		if d.FolderPath != "" {
			return d.PlaylistName + " " + d.FolderPath
		}
		return d.PlaylistName
	}
	// Track: search by title, artist, and playlist name
	return d.TrackTitle + " " + d.TrackArtist + " " + d.PlaylistName
}

// DisplayText returns the display string for search results.
func (d DeepSearchItem) DisplayText() string {
	if d.IsPlaylist() {
		name := icons.FormatPlaylist(d.PlaylistName)
		if d.FolderPath != "" {
			return name + " (" + d.FolderPath + ")"
		}
		return name
	}
	// Track
	trackName := d.TrackTitle
	if d.TrackArtist != "" {
		trackName = d.TrackArtist + " - " + trackName
	}
	return icons.FormatAudio(trackName) + " [" + d.PlaylistName + "]"
}

// LeftColumn returns the main text for two-column display.
func (d DeepSearchItem) LeftColumn() string {
	if d.IsPlaylist() {
		return icons.FormatPlaylist(d.PlaylistName)
	}
	trackName := d.TrackTitle
	if d.TrackArtist != "" {
		trackName = d.TrackArtist + " - " + trackName
	}
	return icons.FormatAudio(trackName)
}

// RightColumn returns the context for two-column display.
func (d DeepSearchItem) RightColumn() string {
	if d.IsPlaylist() {
		return d.FolderPath
	}
	return d.PlaylistName
}

// NodeID returns the navigator node ID for this item.
func (d DeepSearchItem) NodeID() string {
	if d.IsPlaylist() {
		return fmt.Sprintf("playlists:playlist:%d", d.PlaylistID)
	}
	return fmt.Sprintf("playlists:track:%d:%d", d.PlaylistID, d.TrackPosition)
}
