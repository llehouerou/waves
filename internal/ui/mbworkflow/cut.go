package mbworkflow

import (
	"fmt"

	"github.com/llehouerou/waves/internal/core"
)

// Cut is a list MusicBrainz gave in part: Read of the Total entities it has.
// The zero Cut is a list read in full.
type Cut struct {
	Read  int
	Total int
}

// cutOf is the cut of a browse: none when it read every entity.
func cutOf[T any](b core.MBBrowsed[T]) Cut {
	if len(b.Items) >= b.Total {
		return Cut{}
	}
	return Cut{Read: len(b.Items), Total: b.Total}
}

// Notice is the line a popup shows with a cut list of what (release groups,
// releases): how many were read and how many MusicBrainz has. A list read in
// full has none.
func (c Cut) Notice(what string) string {
	if c == (Cut{}) {
		return ""
	}
	return fmt.Sprintf("%d of %d %s read from MusicBrainz", c.Read, c.Total, what)
}
