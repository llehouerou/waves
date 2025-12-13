package playlists

import (
	"strings"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/navigator/sourceutil"
)

// DisplayPath returns a human-readable path for display.
// Uses icons to distinguish folders from playlists.
// At root level returns empty string (header bar shows view name).
func (s *Source) DisplayPath(node Node) string {
	switch node.level {
	case LevelRoot:
		return ""
	case LevelFolder:
		if node.folderID == nil {
			return ""
		}
		return s.buildFolderPathWithIcons(*node.folderID)
	case LevelPlaylist:
		if node.playlistID == nil {
			return ""
		}
		pl, err := s.playlists.Get(*node.playlistID)
		if err != nil {
			return ""
		}
		playlistName := icons.FormatPlaylist(pl.Name)
		if pl.FolderID != nil {
			path := s.buildFolderPathWithIcons(*pl.FolderID)
			return sourceutil.BuildPath(path, playlistName)
		}
		return playlistName
	case LevelTrack:
		if node.playlistID == nil {
			return ""
		}
		pl, err := s.playlists.Get(*node.playlistID)
		if err != nil {
			return ""
		}
		playlistName := icons.FormatPlaylist(pl.Name)
		if pl.FolderID != nil {
			path := s.buildFolderPathWithIcons(*pl.FolderID)
			return sourceutil.BuildPath(path, playlistName)
		}
		return playlistName
	}
	return ""
}

// buildFolderPathWithIcons builds the path string for a folder with folder icons.
func (s *Source) buildFolderPathWithIcons(folderID int64) string {
	var parts []string
	currentID := folderID

	for {
		folder, err := s.playlists.FolderByID(currentID)
		if err != nil {
			break
		}
		parts = append([]string{icons.FormatDir(folder.Name)}, parts...)
		if folder.ParentID == nil {
			break
		}
		currentID = *folder.ParentID
	}

	return strings.Join(parts, " > ")
}
