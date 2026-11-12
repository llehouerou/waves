package core

import (
	"math"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlist"
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

// Playback is the playback domain of the core: transport, queue edits, modes,
// undo and redo, volume. Every command and query of the playback service is available
// on it; the core adds what the service alone cannot know.
//
// Subscribe is there for the MPRIS adapter only, until it moves into the core
// (issue #88, step 5): a client listens to Core.Events.
type Playback struct {
	playback.Service
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
}

// ToggleMute mutes or unmutes the sound, keeping the volume.
func (p *Playback) ToggleMute() {
	pl := p.core.player
	pl.SetMuted(!pl.Muted())
	mark(p.core.volumeDirty)
}
