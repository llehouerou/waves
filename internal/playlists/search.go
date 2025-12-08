package playlists

import (
	"fmt"
	"strings"

	"github.com/llehouerou/waves/internal/icons"
)

// SearchItem represents a playlist in search results for add-to-playlist.
type SearchItem struct {
	ID         int64
	Name       string
	FolderPath string // e.g., "Rock/Favorites" for display
	LastUsedAt int64
}

// DeepSearchItem represents a playlist or track in deep search results.
type DeepSearchItem struct {
	PlaylistID    int64
	PlaylistName  string
	FolderPath    string // Folder containing the playlist
	TrackPosition int    // -1 for playlist items, >= 0 for tracks
	TrackTitle    string
	TrackArtist   string
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

// IsPlaylist returns true if this is a playlist item (not a track).
func (d DeepSearchItem) IsPlaylist() bool {
	return d.TrackPosition < 0
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

// AllForAddToPlaylist returns all playlists for the add-to-playlist picker, sorted by most recently used.
func (p *Playlists) AllForAddToPlaylist() ([]SearchItem, error) {
	rows, err := p.db.Query(`
		SELECT id, name, folder_id, last_used_at
		FROM playlists
		ORDER BY last_used_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build folder path cache
	folderPaths, err := p.buildFolderPaths()
	if err != nil {
		return nil, err
	}

	var items []SearchItem
	for rows.Next() {
		var item SearchItem
		var folderID *int64
		if err := rows.Scan(&item.ID, &item.Name, &folderID, &item.LastUsedAt); err != nil {
			return nil, err
		}
		if folderID != nil {
			item.FolderPath = folderPaths[*folderID]
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// AllDeepSearchItems returns all playlists and their tracks for deep search.
func (p *Playlists) AllDeepSearchItems() ([]DeepSearchItem, error) {
	// Build folder path cache
	folderPaths, err := p.buildFolderPaths()
	if err != nil {
		return nil, err
	}

	// Get all playlists
	playlistRows, err := p.db.Query(`
		SELECT id, name, folder_id
		FROM playlists
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer playlistRows.Close()

	type playlistInfo struct {
		id         int64
		name       string
		folderPath string
	}
	var playlists []playlistInfo

	for playlistRows.Next() {
		var pl playlistInfo
		var folderID *int64
		if err := playlistRows.Scan(&pl.id, &pl.name, &folderID); err != nil {
			return nil, err
		}
		if folderID != nil {
			pl.folderPath = folderPaths[*folderID]
		}
		playlists = append(playlists, pl)
	}
	if err := playlistRows.Err(); err != nil {
		return nil, err
	}

	// Pre-allocate with estimate (playlists + average tracks per playlist)
	items := make([]DeepSearchItem, 0, len(playlists)*10)

	// Add playlists and their tracks
	for _, pl := range playlists {
		// Add playlist item
		items = append(items, DeepSearchItem{
			PlaylistID:    pl.id,
			PlaylistName:  pl.name,
			FolderPath:    pl.folderPath,
			TrackPosition: -1, // Indicates this is a playlist, not a track
		})

		// Get tracks for this playlist
		tracks, err := p.Tracks(pl.id)
		if err != nil {
			continue // Skip tracks if error, but continue with other playlists
		}

		for i, track := range tracks {
			items = append(items, DeepSearchItem{
				PlaylistID:    pl.id,
				PlaylistName:  pl.name,
				FolderPath:    pl.folderPath,
				TrackPosition: i,
				TrackTitle:    track.Title,
				TrackArtist:   track.Artist,
			})
		}
	}

	return items, nil
}

type folderInfo struct {
	parentID *int64
	name     string
}

// buildFolderPaths builds a map of folder ID to full path string.
func (p *Playlists) buildFolderPaths() (map[int64]string, error) {
	rows, err := p.db.Query(`
		SELECT id, parent_id, name FROM playlist_folders
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make(map[int64]folderInfo)

	for rows.Next() {
		var id int64
		var parentID *int64
		var name string
		if err := rows.Scan(&id, &parentID, &name); err != nil {
			return nil, err
		}
		folders[id] = folderInfo{parentID: parentID, name: name}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build paths
	paths := make(map[int64]string)
	for id := range folders {
		paths[id] = buildPath(id, folders)
	}
	return paths, nil
}

func buildPath(id int64, folders map[int64]folderInfo) string {
	var parts []string
	current := id
	for {
		info, ok := folders[current]
		if !ok {
			break
		}
		parts = append([]string{info.name}, parts...)
		if info.parentID == nil {
			break
		}
		current = *info.parentID
	}
	return strings.Join(parts, "/")
}
