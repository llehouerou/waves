package core

import (
	"math"
	"time"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/tags"
)

// The playback data a client handles, so that it imports the core only.
type (
	Track      = playback.Track
	State      = playback.State
	RepeatMode = playback.RepeatMode
)

const (
	StateStopped = playback.StateStopped
	StatePlaying = playback.StatePlaying
	StatePaused  = playback.StatePaused

	RepeatOff   = playback.RepeatOff
	RepeatAll   = playback.RepeatAll
	RepeatOne   = playback.RepeatOne
	RepeatRadio = playback.RepeatRadio
)

// TracksFromLibrary converts library tracks to the tracks the queue takes.
func TracksFromLibrary(tracks []LibraryTrack) []Track {
	return playback.TracksFromPlaylist(playlist.FromLibraryTracks(tracks))
}

// TracksUnder reads the tracks at a path of the file system, in or out of the
// library: the file itself when it is a music file, every music file under
// it, sorted by path, when it is a directory.
func TracksUnder(path string) ([]Track, error) {
	tracks, err := playlist.CollectFromPath(path)
	return playback.TracksFromPlaylist(tracks), err
}

// FolderTracks reads the music files of the folder holding a file, sorted by
// path, and returns the index of that file among them. For a directory it
// returns what TracksUnder does, and 0.
func FolderTracks(path string) (tracks []Track, index int, err error) {
	found, index, err := playlist.CollectFolderFromPath(path)
	return playback.TracksFromPlaylist(found), index, err
}

// playbackAPI is the part of playback.Service a client calls as it is. What
// is left out is the core's: it listens to the service (a client listens to
// Core.Events), restores the saved queue, resumes a queue that ran out when a
// radio fill lands, and closes playback.
type playbackAPI interface {
	Play() error
	Pause() error
	Stop() error
	Toggle() error
	Next() error
	Previous() error
	Seek(delta time.Duration) error
	SeekTo(position time.Duration) error
	JumpTo(index int) error

	QueueAdvance() *Track
	QueueMoveTo(index int) *Track
	AddTracks(tracks ...Track)
	ReplaceTracks(tracks ...Track) *Track
	RemoveTracks(indices []int)
	MoveTracks(indices []int, delta int)
	ClearQueue()
	Undo() bool
	Redo() bool

	State() State
	IsPlaying() bool
	IsStopped() bool
	IsPaused() bool
	Position() time.Duration
	Duration() time.Duration
	CurrentTrack() *Track
	TrackInfo() *tags.FileInfo

	QueueTracks() []Track
	QueueCurrentIndex() int
	QueueLen() int
	QueueIsEmpty() bool
	QueueHasNext() bool
	QueuePeekNext() *Track

	RepeatMode() RepeatMode
	SetRepeatMode(mode RepeatMode)
	Shuffle() bool
	SetShuffle(enabled bool)
	ToggleShuffle() bool
}

// Playback is the playback domain of the core: transport, queue edits, modes,
// undo and redo, volume. The core adds what the playback service alone cannot
// know: the volume, and the radio among the repeat modes.
type Playback struct {
	playbackAPI
	svc  playback.Service
	core *Core
}

// Volume returns the volume, from 0 to 1.
func (p *Playback) Volume() float64 {
	return p.core.player.Volume()
}

// Muted reports whether the sound is muted.
func (p *Playback) Muted() bool {
	return p.core.player.Muted()
}

// AdjustVolume changes the volume by delta, and unmutes.
func (p *Playback) AdjustVolume(delta float64) {
	pl := p.core.player
	if pl.Muted() {
		pl.SetMuted(false)
	}
	// Rounded to the percent: steps of 0.1 would otherwise drift to 0.7000001.
	pl.SetVolume(math.Round((pl.Volume()+delta)*100) / 100)
	mark(p.core.volumeDirty)
	p.core.volumeChanged()
}

// ToggleMute mutes or unmutes the sound, keeping the volume.
func (p *Playback) ToggleMute() {
	pl := p.core.player
	pl.SetMuted(!pl.Muted())
	mark(p.core.volumeDirty)
	p.core.volumeChanged()
}
