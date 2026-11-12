package retag

import (
	"github.com/llehouerou/waves/internal/core"
)

// TagsReadMsg carries the tags of the album's files, in track order, or why
// the core could not read them.
type TagsReadMsg struct {
	Tags []core.FileTags
	Err  error
}
