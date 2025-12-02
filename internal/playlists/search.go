package playlists

import "strings"

// SearchItem represents a playlist in search results for add-to-playlist.
type SearchItem struct {
	ID         int64
	Name       string
	FolderPath string // e.g., "Rock/Favorites" for display
	LastUsedAt int64
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

// AllForSearch returns all playlists for fuzzy search, sorted by most recently used.
func (p *Playlists) AllForSearch() ([]SearchItem, error) {
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
