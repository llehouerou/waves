package retag

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/request"
)

// readAlbumTags asks the core for the tags of the album's files, off the UI
// goroutine: it reads them from the disk.
func readAlbumTags(lib Library, albumArtist, album string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			fileTags, err := lib.AlbumTags(albumArtist, album)
			return TagsReadMsg{Request: id, Tags: fileTags, Err: err}
		}
	}
}
