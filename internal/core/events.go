package core

import (
	"time"

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

// DownloadsChanged reports that the downloads are no longer what the core
// last reported: a Sync moved them, or a command changed them since. It
// carries them all, newest first, as Downloads.List returns them.
type DownloadsChanged struct {
	Downloads []Download
}

// SearchProgressed reports where a slskd search is.
type SearchProgressed struct {
	Query     string // the search it is about: the one Search was given
	Phase     string // SearchSearching, SearchCollecting or SearchFetching
	State     string // slskd's own state: "InProgress", "Completed, TimedOut"...
	Responses int    // users who have answered so far
	Elapsed   time.Duration
}

// SearchFinished reports the end of a slskd search: its responses, for
// FilterAndScoreResults, or why it failed. The failure comes here rather than
// in a Failed: it is the answer to the search the user is looking at.
type SearchFinished struct {
	Query     string
	Responses []SlskdResponse
	Err       error
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

func (DownloadsChanged) event() {}

func (SearchProgressed) event() {}

func (SearchFinished) event() {}

func (Failed) event() {}
