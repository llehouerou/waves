package player

import (
	"math"

	"github.com/gopxl/beep/v2/speaker"
)

// SetVolume sets the volume level (0.0 to 1.0).
// If muted, only stores the level without applying it.
func (p *Player) SetVolume(level float64) {
	// Clamp to valid range
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}

	p.volumeLevel = level

	// Apply if not muted and volume effect exists
	if !p.muted && p.volume != nil {
		speaker.Lock()
		p.volume.Volume = p.levelToVolume(level)
		speaker.Unlock()
	}
}

// Volume returns the current volume level (0.0 to 1.0).
func (p *Player) Volume() float64 {
	return p.volumeLevel
}

// SetMuted sets the muted state.
// When unmuted, restores the previous volume level.
func (p *Player) SetMuted(muted bool) {
	p.muted = muted

	if p.volume != nil {
		speaker.Lock()
		p.volume.Silent = muted
		speaker.Unlock()
	}
}

// Muted returns true if audio is muted.
func (p *Player) Muted() bool {
	return p.muted
}

// levelToVolume converts a 0.0-1.0 level to beep's Volume value.
// beep uses a logarithmic scale where Volume is in "decibels" with base 2.
// Volume = 0 means no change, -1 = half volume, -2 = quarter, etc.
// We map: 1.0 -> 0, 0.5 -> -1, 0.25 -> -2, 0 -> -10 (essentially silent)
func (p *Player) levelToVolume(level float64) float64 {
	if level <= 0 {
		return -10
	}
	if level >= 1 {
		return 0
	}
	return math.Log2(level)
}
