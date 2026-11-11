package core

import (
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

// TracksFromPlaylist converts playlist tracks to the tracks the queue takes.
func TracksFromPlaylist(tracks []playlist.Track) []Track {
	return playback.TracksFromPlaylist(tracks)
}

// Playback is the playback domain of the core: transport, queue edits, modes,
// undo and redo. Every command and query of the playback service is available
// on it.
//
// Subscribe is there for the MPRIS adapter only, until it moves into the core
// (issue #88, step 5): a client listens to Core.Events.
type Playback struct {
	playback.Service
}
