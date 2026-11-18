package retag

import (
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/request"
)

// TagsReadMsg carries the tags of the files of the album, in track order, or
// why the core could not read them.
type TagsReadMsg struct {
	Request request.ID
	Tags    []core.FileTags
	Err     error
}
