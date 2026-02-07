//go:build linux

package mpris

import (
	"fmt"
	"hash/fnv"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/quarckster/go-mpris-server/pkg/server"
	"github.com/quarckster/go-mpris-server/pkg/types"

	"github.com/llehouerou/waves/internal/playback"
)

// Adapter connects PlaybackService to MPRIS over D-Bus.
type Adapter struct {
	service playback.Service
	server  *server.Server
	sub     *playback.Subscription
	done    chan struct{}
}

// New creates and starts a new MPRIS adapter.
func New(service playback.Service) (*Adapter, error) {
	a := &Adapter{
		service: service,
		done:    make(chan struct{}),
	}

	// Create adapters that delegate to the service
	rootAdapter := &rootAdapter{}
	playerAdapter := &playerAdapter{service: service}

	a.server = server.NewServer("waves", rootAdapter, playerAdapter)
	a.sub = service.Subscribe()

	// Start the server in background
	go func() {
		_ = a.server.Listen()
	}()

	return a, nil
}

// Resubscribe updates the adapter to use a new PlaybackService instance.
// Call this when PlaybackService is recreated (e.g., after queue restore).
func (a *Adapter) Resubscribe(service playback.Service) {
	a.service = service
	a.sub = service.Subscribe()
	// Update the player adapter's service reference
	if pa, ok := a.server.PlayerAdapter.(*playerAdapter); ok {
		pa.service = service
	}
}

// Close stops the adapter and releases D-Bus resources.
func (a *Adapter) Close() error {
	close(a.done)
	return a.server.Stop()
}

// rootAdapter implements OrgMprisMediaPlayer2Adapter.
type rootAdapter struct{}

func (r *rootAdapter) Raise() error {
	return nil // Not supported
}

func (r *rootAdapter) Quit() error {
	return nil // Not supported - app manages its own lifecycle
}

func (r *rootAdapter) CanQuit() (bool, error) {
	return false, nil
}

func (r *rootAdapter) CanRaise() (bool, error) {
	return false, nil
}

func (r *rootAdapter) HasTrackList() (bool, error) {
	return false, nil // Track list interface not implemented
}

func (r *rootAdapter) Identity() (string, error) {
	return "Waves", nil
}

//nolint:revive // Method name required by interface.
func (r *rootAdapter) SupportedUriSchemes() ([]string, error) {
	return []string{"file"}, nil
}

func (r *rootAdapter) SupportedMimeTypes() ([]string, error) {
	return []string{"audio/mpeg", "audio/flac", "audio/mp3"}, nil
}

// playerAdapter implements OrgMprisMediaPlayer2PlayerAdapter and optional interfaces.
type playerAdapter struct {
	service playback.Service
}

func (p *playerAdapter) Next() error {
	return p.service.Next()
}

func (p *playerAdapter) Previous() error {
	return p.service.Previous()
}

func (p *playerAdapter) Pause() error {
	return p.service.Pause()
}

func (p *playerAdapter) PlayPause() error {
	return p.service.Toggle()
}

func (p *playerAdapter) Stop() error {
	return p.service.Stop()
}

func (p *playerAdapter) Play() error {
	if p.service.IsStopped() {
		return p.service.Play()
	}
	return p.service.Toggle()
}

func (p *playerAdapter) Seek(offset types.Microseconds) error {
	return p.service.Seek(time.Duration(offset) * time.Microsecond)
}

func (p *playerAdapter) SetPosition(_ string, position types.Microseconds) error {
	return p.service.SeekTo(time.Duration(position) * time.Microsecond)
}

//nolint:revive // Method name required by interface.
func (p *playerAdapter) OpenUri(_ string) error {
	return nil // Not supported
}

func (p *playerAdapter) PlaybackStatus() (types.PlaybackStatus, error) {
	switch p.service.State() {
	case playback.StatePlaying:
		return types.PlaybackStatusPlaying, nil
	case playback.StatePaused:
		return types.PlaybackStatusPaused, nil
	case playback.StateStopped:
		return types.PlaybackStatusStopped, nil
	}
	return types.PlaybackStatusStopped, nil
}

func (p *playerAdapter) Rate() (float64, error) {
	return 1.0, nil
}

func (p *playerAdapter) SetRate(_ float64) error {
	return nil // Not supported
}

func (p *playerAdapter) Metadata() (types.Metadata, error) {
	track := p.service.CurrentTrack()
	if track == nil {
		return types.Metadata{}, nil
	}

	meta := types.Metadata{
		TrackId:     dbus.ObjectPath(formatTrackID(track.Path)),
		Length:      types.Microseconds(track.Duration.Microseconds()),
		Title:       track.Title,
		Artist:      []string{track.Artist},
		Album:       track.Album,
		TrackNumber: track.TrackNumber,
	}

	if artPath := FindAlbumArt(track.Path); artPath != "" {
		meta.ArtUrl = "file://" + artPath
	}

	return meta, nil
}

func (p *playerAdapter) Volume() (float64, error) {
	return 1.0, nil // Volume control not exposed via service
}

func (p *playerAdapter) SetVolume(_ float64) error {
	return nil // Not supported
}

func (p *playerAdapter) Position() (int64, error) {
	return p.service.Position().Microseconds(), nil
}

func (p *playerAdapter) MinimumRate() (float64, error) {
	return 1.0, nil
}

func (p *playerAdapter) MaximumRate() (float64, error) {
	return 1.0, nil
}

func (p *playerAdapter) CanGoNext() (bool, error) {
	return p.service.QueueHasNext(), nil
}

func (p *playerAdapter) CanGoPrevious() (bool, error) {
	return p.service.QueueCurrentIndex() > 0, nil
}

func (p *playerAdapter) CanPlay() (bool, error) {
	return !p.service.QueueIsEmpty(), nil
}

func (p *playerAdapter) CanPause() (bool, error) {
	return true, nil
}

func (p *playerAdapter) CanSeek() (bool, error) {
	return true, nil
}

func (p *playerAdapter) CanControl() (bool, error) {
	return true, nil
}

// LoopStatus implements OrgMprisMediaPlayer2PlayerAdapterLoopStatus.
func (p *playerAdapter) LoopStatus() (types.LoopStatus, error) {
	switch p.service.RepeatMode() {
	case playback.RepeatOne:
		return types.LoopStatusTrack, nil
	case playback.RepeatAll:
		return types.LoopStatusPlaylist, nil
	case playback.RepeatOff, playback.RepeatRadio:
		return types.LoopStatusNone, nil
	}
	return types.LoopStatusNone, nil
}

// SetLoopStatus implements OrgMprisMediaPlayer2PlayerAdapterLoopStatus.
func (p *playerAdapter) SetLoopStatus(status types.LoopStatus) error {
	switch status {
	case types.LoopStatusNone:
		p.service.SetRepeatMode(playback.RepeatOff)
	case types.LoopStatusTrack:
		p.service.SetRepeatMode(playback.RepeatOne)
	case types.LoopStatusPlaylist:
		p.service.SetRepeatMode(playback.RepeatAll)
	}
	return nil
}

// Shuffle implements OrgMprisMediaPlayer2PlayerAdapterShuffle.
func (p *playerAdapter) Shuffle() (bool, error) {
	return p.service.Shuffle(), nil
}

// SetShuffle implements OrgMprisMediaPlayer2PlayerAdapterShuffle.
func (p *playerAdapter) SetShuffle(shuffle bool) error {
	p.service.SetShuffle(shuffle)
	return nil
}

func formatTrackID(path string) string {
	h := fnv.New64a()
	h.Write([]byte(path))
	return fmt.Sprintf("/org/mpris/MediaPlayer2/Track/%x", h.Sum64())
}
