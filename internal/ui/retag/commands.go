package retag

import (
	tea "github.com/charmbracelet/bubbletea"
)

// readAlbumTagsCmd asks the core for the tags of the album's files, off the
// UI goroutine: it reads them from the disk.
func readAlbumTagsCmd(lib Library, albumArtist, album string) tea.Cmd {
	return func() tea.Msg {
		fileTags, err := lib.AlbumTags(albumArtist, album)
		return TagsReadMsg{AlbumArtist: albumArtist, Album: album, Tags: fileTags, Err: err}
	}
}
