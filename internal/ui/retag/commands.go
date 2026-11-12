package retag

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/mbworkflow"
)

// readAlbumTagsCmd asks the core for the tags of the album's files, off the
// UI goroutine: it reads them from the disk.
func readAlbumTagsCmd(lib Library, albumArtist, album string) tea.Cmd {
	return func() tea.Msg {
		fileTags, err := lib.AlbumTags(albumArtist, album)
		return TagsReadMsg{Tags: fileTags, Err: err}
	}
}

// FetchReleaseByIDCmd fetches full release details directly by MusicBrainz ID.
// This is retag-specific as it returns ReleaseDetailsFetchedMsg for direct ID lookup.
func FetchReleaseByIDCmd(client mbworkflow.Client, releaseID string) tea.Cmd {
	return func() tea.Msg {
		release, err := client.GetRelease(releaseID)
		return mbworkflow.ReleaseDetailsResultMsg{Details: release, Err: err}
	}
}
