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

// ImportProgressed reports where an import is: every file of its plan, in the
// plan's order. Each one is the whole state, so that one left out by the core
// or missed by a client loses nothing.
type ImportProgressed struct {
	DownloadID int64
	Files      []FileProgress
}

// ImportFinished reports the end of an import, and how each file ended. The
// files that made it are in the library; the download is finished and gone
// when they all did (AllDone), kept otherwise, to be imported again.
//
// Err is what failed after the files: adding them to the library. It comes
// here rather than in a Failed: it is the answer to the import the user is
// looking at.
type ImportFinished struct {
	DownloadID    int64
	Artist, Album string // the album imported, as the library names it
	Files         []FileProgress
	Err           error
}

// RetagProgressed reports where the retag of an album is: every file of the
// album, in track order. Like ImportProgressed, each one is the whole state.
type RetagProgressed struct {
	AlbumArtist, Album string // the album as the retag was asked for it
	Files              []FileProgress
}

// RetagFinished reports the end of the retag of an album, and how each file
// ended. The library has read the files again: the album is under the names
// the retag wrote, which may not be those it was asked for.
//
// Err is what failed after the files: reading them again into the library.
type RetagFinished struct {
	AlbumArtist, Album string // the album as the retag was asked for it
	Files              []FileProgress
	Err                error
}

// ReleasesRefreshed reports the end of a refresh of the new releases: the list
// ListenBrainz gives is cached again, or Err is why it could not be. The
// failure comes here rather than in a Failed: it belongs with the list, which
// keeps showing what was cached before.
type ReleasesRefreshed struct {
	Err error
}

// WarmupFinished reports the end of a warm-up that asked Last.fm for the
// artists similar to some of the library's: the discoveries among the new
// releases may have changed.
type WarmupFinished struct{}

// VolumeChanged reports a volume set from outside waves: over MPRIS, by a
// desktop widget. A client that changes the volume itself gets none.
type VolumeChanged struct{}

// ExportProgressed reports where an export is: how many of its tracks are
// done, and what a client needs to name it. Each one is the whole state, so
// that one left out by the core or missed by a client loses nothing. The
// first comes before the first track.
type ExportProgressed struct {
	ID            int64  // the export, as Start returned it
	Target        string // the name of the target exported to
	Artist, Album string // those of the first track
	Done, Total   int
}

// ExportFinished reports the end of an export. The Done first tracks were
// reached, and are under the target but for those in Failed.
//
// Err is why the export ended before its last track: ErrTargetNotConnected
// when the device of its target was unplugged. It comes here rather than in a
// Failed, with the export it is about.
type ExportFinished struct {
	ID            int64
	Target        string
	Artist, Album string
	Done, Total   int
	Failed        []ExportFailure
	Err           error
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

func (ImportProgressed) event() {}

func (ImportFinished) event() {}

func (RetagProgressed) event() {}

func (RetagFinished) event() {}

func (ReleasesRefreshed) event() {}

func (WarmupFinished) event() {}

func (VolumeChanged) event() {}

func (ExportProgressed) event() {}

func (ExportFinished) event() {}

func (Failed) event() {}
