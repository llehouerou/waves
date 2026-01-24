package player

import (
	"io"

	"github.com/gopxl/beep/v2"
)

const opusSampleRate = 48000

// decodeOpus decodes an Ogg/Opus stream. Kept for backward compatibility.
// Prefer decodeOgg which handles both Opus and Vorbis.
func decodeOpus(rc io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error) {
	return decodeOgg(rc)
}
