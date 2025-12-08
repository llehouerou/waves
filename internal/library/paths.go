package library

import (
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/navigator/sourceutil"
)

const libraryRootPath = "Library"

// DisplayPath returns a human-readable path for display.
func (s *Source) DisplayPath(node Node) string {
	root := icons.FormatDir(libraryRootPath)
	switch node.level {
	case LevelRoot:
		return root
	case LevelArtist:
		return sourceutil.BuildPath(root, icons.FormatArtist(node.artist))
	case LevelAlbum:
		return sourceutil.BuildPath(root, icons.FormatArtist(node.artist), icons.FormatAlbum(node.album))
	case LevelTrack:
		return sourceutil.BuildPath(root, icons.FormatArtist(node.artist), icons.FormatAlbum(node.album))
	}
	return root
}

// wrapPath wraps a path string into multiple lines with indent.
// Uses rune-based operations to handle Unicode characters correctly.
func wrapPath(path string, maxWidth int) []string {
	const indent = "  "
	contentWidth := maxWidth - len(indent)
	if contentWidth <= 0 {
		contentWidth = 20
	}

	runes := []rune(path)
	var lines []string

	for len(runes) > 0 {
		if len(runes) <= contentWidth {
			lines = append(lines, indent+string(runes))
			break
		}
		// Find a good break point (prefer after /)
		breakAt := contentWidth
		for i := contentWidth; i > contentWidth/2; i-- {
			if runes[i] == '/' {
				breakAt = i + 1
				break
			}
		}
		lines = append(lines, indent+string(runes[:breakAt]))
		runes = runes[breakAt:]
	}
	return lines
}
