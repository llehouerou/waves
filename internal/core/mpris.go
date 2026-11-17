package core

import (
	"errors"
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
// playback it is given and the volume. MPRIS listens to the service itself
// rather than to the core's loop: each track it announces makes it look for a
// cover on the disk.
func serveMPRIS(svc playback.Service, volume mpris.Volume) mprisServer {
	// It fails without a session bus: waves then plays without MPRIS.
	adapter, err := mpris.New(svc, volume)
	if err != nil {
		return nil
	}
	return adapter
}

// mprisPlayback is the playback MPRIS drives: the service, but a start it
// sends that fails to open its track is reported to the clients.
type mprisPlayback struct {
	playback.Service
	core *Core
}

// Play is a start no client sent: a track that fails to open, the core tells
// them. The bus still gets the error.
func (p mprisPlayback) Play() error {
	// Read before the start: the service lets go of the queue while the
	// player opens the file, and a client may move it meanwhile.
	track := p.toStart()
	return p.reported(track, p.Service.Play())
}

// Toggle, from stopped, is a start no client sent, as Play is.
func (p mprisPlayback) Toggle() error {
	track := p.toStart() // before the start, as in Play
	return p.reported(track, p.Service.Toggle())
}

// toStart is the track a start from stopped plays: the current track, or the
// first of a queue that has none. Nil on an empty queue.
func (p mprisPlayback) toStart() *playback.Track {
	if track := p.CurrentTrack(); track != nil {
		return track
	}
	if tracks := p.QueueTracks(); len(tracks) > 0 {
		return &tracks[0]
	}
	return nil
}

// reported emits PlaybackFailed for a start of track that failed to open it,
// and returns err. On an empty queue, nothing failed to play.
func (p mprisPlayback) reported(track *playback.Track, err error) error {
	if err == nil || errors.Is(err, playback.ErrEmptyQueue) {
		return err
	}
	failed := playback.ErrorEvent{Operation: "play", Err: err}
	if track != nil {
		failed.Path = track.Path
	}
	p.core.emit(PlaybackFailed(failed))
	return err
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
