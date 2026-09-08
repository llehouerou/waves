//go:build !unix

package albumart

import (
	"io"
	"strings"
	"time"
)

// newDeadlineReader has no portable implementation outside unix, so the probe
// reads nothing and the block renderer handles the art.
func newDeadlineReader(_ uintptr, _ time.Duration) io.Reader {
	return strings.NewReader("")
}
