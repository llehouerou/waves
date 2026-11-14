package core

import (
	"sync/atomic"
	"time"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/library"
)

// progressEvery is the least time between two progress events of one piece
// of background work: two ScanProgressed of one scan phase, two
// ImportProgressed of one import. A scan reports every hundred files, an
// import every file: unbounded, a large library or a box set would fill the
// event stream and push playback events, and their own end, out of it.
const progressEvery = 100 * time.Millisecond

// The library data a client handles, so that it imports the core only.
type (
	LibraryTrack = library.Track
	Album        = library.Album
	AlbumEntry   = library.AlbumEntry
	SearchResult = library.SearchResult
	ScanStats    = library.ScanStats
	SourceStats  = library.SourceStats

	// DatePrecision is how much of a date an album carries.
	DatePrecision = library.DatePrecision
)

const (
	// What a SearchResult is.
	ResultArtist = library.ResultArtist
	ResultAlbum  = library.ResultAlbum
	ResultTrack  = library.ResultTrack

	PrecisionNone  = library.PrecisionNone
	PrecisionYear  = library.PrecisionYear
	PrecisionMonth = library.PrecisionMonth
	PrecisionDay   = library.PrecisionDay

	// SourceRoot stands for the source itself in SourceStats.Unread.
	SourceRoot = library.SourceRoot
)

// ParseDate reads an album date of any precision: YYYY, YYYY-MM or YYYY-MM-DD.
func ParseDate(date string) (t time.Time, precision DatePrecision) {
	return library.ParseDate(date)
}

// ParseDatePrecision returns the precision of an album date.
func ParseDatePrecision(date string) DatePrecision {
	return library.ParseDatePrecision(date)
}

// NormalizeTitle folds an artist or album name into the form the library
// compares them in: the keys of AlbumsForArtistNormalized.
func NormalizeTitle(s string) string {
	return library.NormalizeTitle(s)
}

// libraryAPI is the part of library.Library a client calls as it is.
type libraryAPI interface {
	Sources() ([]string, error)
	SourceExists(path string) (bool, error)
	TrackCountBySource(path string) (int, error)
	RemoveSource(path string) error

	Artists() ([]string, error)
	Albums(albumArtist string) ([]Album, error)
	AllAlbums() ([]AlbumEntry, error)
	Tracks(albumArtist, album string) ([]LibraryTrack, error)
	ArtistTracks(albumArtist string) ([]LibraryTrack, error)
	TrackByID(id int64) (*LibraryTrack, error)
	AlbumTrackIDs(albumArtist, album string) ([]int64, error)
	AlbumHasMultipleDiscs(albumArtist, album string) (bool, error)
	AlbumsForArtistNormalized(albumArtist string) map[string]struct{}
	SearchFTS(query string) ([]SearchResult, error)
	SearchAlbumsFTS(query string) ([]SearchResult, error)

	DeleteTrack(id int64) error
}

// Library is the library domain of the core: sources, artists, albums,
// tracks, search. The core adds the scan and the retag of an album, which it
// runs and reports itself, and keeps the search index up to date.
type Library struct {
	libraryAPI
	lib       *library.Library
	core      *Core
	scanning  atomic.Bool
	retagging atomic.Bool
}

// Scanning reports whether a scan is running.
func (l *Library) Scanning() bool {
	return l.scanning.Load()
}

// Refresh scans every source for new, changed and removed files. It does
// nothing while a scan is running; ScanFinished reports the end of the scan.
func (l *Library) Refresh() error {
	return l.scanSources(l.lib.Refresh)
}

// FullRefresh scans every source again, reading every file even when it has
// not changed.
func (l *Library) FullRefresh() error {
	return l.scanSources(l.lib.FullRefresh)
}

// AddSource adds a source to the library and scans it. The source is added
// but not scanned while another scan is running.
func (l *Library) AddSource(path string) error {
	if err := l.lib.AddSource(path); err != nil {
		return err
	}
	l.scan(func(progress chan<- library.ScanProgress) error {
		return l.lib.RefreshSource(path, progress, l.core.stopping)
	})
	return nil
}

func (l *Library) scanSources(refresh func([]string, chan<- library.ScanProgress, func() bool) error) error {
	sources, err := l.lib.Sources()
	if err != nil || len(sources) == 0 {
		return err
	}
	l.scan(func(progress chan<- library.ScanProgress) error {
		return refresh(sources, progress, l.core.stopping)
	})
	return nil
}

// scan runs a scan off the caller's goroutine, one at a time. The library
// waits for each of its progress reports to be read: the core reads them all,
// so that the scan never waits for a client.
func (l *Library) scan(run func(progress chan<- library.ScanProgress) error) {
	if !l.scanning.CompareAndSwap(false, true) {
		return
	}
	// It writes to the database: Close waits for it.
	l.core.work.Go(func() {
		progress := make(chan library.ScanProgress)
		failed := make(chan error, 1)
		go func() { failed <- run(progress) }()

		stats := l.report(progress)
		// A scan cut short by Close did not fail, nor did it finish: it
		// reports neither.
		if err := <-failed; err != nil && !l.core.stopping() {
			l.core.emit(Failed{Op: errmsg.OpLibraryScan, Err: err})
		}
		// Even after a failure or Close: the scan may have changed tracks before it.
		if err := l.lib.RebuildFTSIndex(); err != nil && !l.core.stopping() {
			l.core.emit(Failed{Op: errmsg.OpLibraryRebuild, Err: err})
		}
		l.scanning.Store(false)
		if !l.core.stopping() {
			l.core.emit(ScanFinished{Stats: stats})
		}
	})
}

// report forwards the progress of a scan to the client until the scan closes
// it, and returns the stats of the scan: nil when it failed.
func (l *Library) report(progress <-chan library.ScanProgress) *ScanStats {
	var (
		stats *ScanStats
		phase string
		last  time.Time
	)
	for p := range progress {
		if p.Phase == "done" {
			stats = p.Stats
			continue
		}
		if p.Phase == phase && time.Since(last) < progressEvery {
			continue
		}
		phase, last = p.Phase, time.Now()
		l.core.emit(ScanProgressed{Phase: p.Phase, Current: p.Current, Total: p.Total})
	}
	return stats
}

// start takes the sources of the config file when the library has none, and
// builds the search index of a library that has none.
func (l *Library) start(configSources []string) {
	if err := l.lib.MigrateSources(configSources); err != nil {
		l.core.emit(Failed{Op: errmsg.OpSourceLoad, Err: err})
	}
	// It writes to the database: Close waits for it.
	l.core.work.Go(func() {
		if err := l.lib.EnsureFTSIndex(); err != nil {
			l.core.emit(Failed{Op: errmsg.OpLibraryRebuild, Err: err})
		}
	})
}
