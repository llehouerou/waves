package retag

import (
	"github.com/llehouerou/waves/internal/core"
)

// TagsReadMsg carries the tags of the files of the album it names, in track
// order, or why the core could not read them.
type TagsReadMsg struct {
	AlbumArtist string
	Album       string
	Tags        []core.FileTags
	Err         error
}
