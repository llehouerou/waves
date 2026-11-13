package mpris

// Volume is the volume of waves, which the playback service does not have.
type Volume interface {
	// Volume is what is heard, from 0 to 1: 0 when muted.
	Volume() float64
	// SetVolume makes level, from 0 to 1, the volume of waves, unmuted.
	SetVolume(level float64)
}
