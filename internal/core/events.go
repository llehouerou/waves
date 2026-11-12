package core

import (
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/playback"
)

// Event is one thing that happened in the core. Events carry plain data.
type Event interface {
	event()
}

type (
	// StateChanged reports that playback started, paused or stopped.
	StateChanged playback.StateChange
	// TrackChanged reports that playback moved to another track.
	TrackChanged playback.TrackChange
	// QueueChanged reports a queue edit, or a move of the queue position
	// without a track starting.
	QueueChanged playback.QueueChange
	// ModeChanged reports a change of repeat or shuffle.
	ModeChanged playback.ModeChange
	// PositionChanged reports a seek.
	PositionChanged playback.PositionChange
	// PlaybackFailed reports a track that could not be played.
	PlaybackFailed playback.ErrorEvent
)

// ScanProgressed reports where a library scan is.
type ScanProgressed struct {
	Phase   string // "scanning", "processing" or "cleaning"
	Current int
	Total   int // 0 while unknown
}

// ScanFinished reports the end of a library scan: the library and its search
// index are up to date. Stats is nil when the scan failed; a Failed came
// before it.
type ScanFinished struct {
	Stats *ScanStats
}

// Failed reports background work that failed, for the client to show or not.
type Failed struct {
	Op  errmsg.Op
	Err error
}

func (StateChanged) event() {}

func (TrackChanged) event() {}

func (QueueChanged) event() {}

func (ModeChanged) event() {}

func (PositionChanged) event() {}

func (PlaybackFailed) event() {}

func (ScanProgressed) event() {}

func (ScanFinished) event() {}

func (Failed) event() {}
