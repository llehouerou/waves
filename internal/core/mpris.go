package core

import (
	"io"

	"github.com/llehouerou/waves/internal/mpris"
	"github.com/llehouerou/waves/internal/playback"
)

// mprisServer is the MPRIS server on the session bus.
type mprisServer interface {
	io.Closer
	// VolumeChanged tells the bus that the volume it reports changed.
	VolumeChanged()
}

// serveMPRIS registers waves on the session bus as a media player over the
// playback service and the volume. MPRIS listens to the service itself rather
// than to the core's loop: each track it announces makes it look for a cover
// on the disk.
func serveMPRIS(svc playback.Service, volume mpris.Volume) mprisServer {
	// It fails without a session bus: waves then plays without MPRIS.
	adapter, err := mpris.New(svc, volume)
	if err != nil {
		return nil
	}
	return adapter
}

// mprisVolume is the volume of waves as MPRIS sees it: MPRIS has no mute.
type mprisVolume struct {
	core *Core
}

// Volume is what is heard: 0 when muted.
func (v mprisVolume) Volume() float64 {
	if v.core.player.Muted() {
		return 0
	}
	return v.core.player.Volume()
}

// SetVolume is a change no client sent: the core tells them.
func (v mprisVolume) SetVolume(level float64) {
	c := v.core
	c.player.SetMuted(false)
	c.player.SetVolume(level)
	mark(c.volumeDirty)
	c.emit(VolumeChanged{})
}

// volumeChanged tells the session bus that the volume changed.
func (c *Core) volumeChanged() {
	if c.mpris != nil {
		c.mpris.VolumeChanged()
	}
}
