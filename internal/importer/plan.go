package importer

import (
	"errors"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/tags"
)

// ErrNoRelease is returned for a download that carries no MusicBrainz
// release: there is nothing to import it as.
var ErrNoRelease = errors.New("the download has no MusicBrainz release")

// Catalogue is what an import asks of MusicBrainz.
type Catalogue interface {
	GetRelease(mbid string) (*musicbrainz.ReleaseDetails, error)
	GetCoverArt(releaseMBID string) ([]byte, error)
}

// Plan is what importing a download will do, settled before anything is
// written: a client shows it, and Run carries it out as it is.
type Plan struct {
	DownloadID   int64
	Release      *musicbrainz.ReleaseDetails // the files' own release when their tags name another
	ReleaseGroup *musicbrainz.ReleaseGroup
	Cover        []byte        // embedded in each file and written to the album folder; nil when there is none
	Files        []PlannedFile // in track order: the order Run walks them in
}

// PlannedFile is one file of a download in a Plan.
type PlannedFile struct {
	Filename string   // its slskd path
	Source   string   // where it is on disk
	Tags     tags.Tag // its tags today; only Path is set when they can't be read
	Track    int      // the index of its track in Release.Tracks, -1 for none: it is not imported
	Dest     string   // where it goes, under the library source; "" without a track
}

// Prepare plans the import of a download from the files slskd wrote under
// completedPath: it reads their tags, settles the release they are imported
// as, fetches its cover, matches each file to a track of the release (see
// matchTracks) and names its destination. It waits for the disk and for
// MusicBrainz.
func Prepare(d *downloads.Download, completedPath string, mb Catalogue, cfg rename.Config) (Plan, error) {
	if d.MBReleaseDetails == nil {
		return Plan{}, ErrNoRelease
	}
	plan := Plan{DownloadID: d.ID, Release: d.MBReleaseDetails, ReleaseGroup: d.MBReleaseGroup}

	sorted := downloads.SortFilesByTrackNumber(d.Files)
	fileTags := make([]tags.Tag, len(sorted))
	plan.Files = make([]PlannedFile, len(sorted))
	for i, f := range sorted {
		source := downloads.ExpectedDiskPath(completedPath, f.Filename)
		fileTags[i] = tags.Tag{Path: source}
		if tag, err := tags.Read(source); err == nil {
			fileTags[i] = *tag
		}
		plan.Files[i] = PlannedFile{Filename: f.Filename, Source: source, Tags: fileTags[i]}
	}

	plan.Release = settleRelease(plan.Release, fileTags, mb)
	if plan.Release.ID != "" {
		// A cover that can't be fetched is no reason not to import.
		plan.Cover, _ = mb.GetCoverArt(plan.Release.ID) //nolint:errcheck // as above
	}

	for i, track := range matchTracks(sorted, fileTags, plan.Release) {
		plan.Files[i].Track = track
		if track >= 0 {
			plan.Files[i].Dest = DestPath(plan.params(i, "", cfg))
		}
	}
	return plan, nil
}

// settleRelease returns the release the files are imported as: the one their
// tags name when it is not the one the download was queued as, which is
// itself read again when it lacks fields the import writes (a download queued
// before waves kept them). A release MusicBrainz doesn't return leaves the
// queued one in place.
func settleRelease(queued *musicbrainz.ReleaseDetails, fileTags []tags.Tag, mb Catalogue) *musicbrainz.ReleaseDetails {
	named := ""
	for i := range fileTags {
		if named = fileTags[i].MBReleaseID; named != "" {
			break // the first file that names a release speaks for them all
		}
	}

	id := queued.ID
	switch {
	case named != "" && named != queued.ID:
		id = named
	case queued.ArtistID == "" || queued.Label == "" && queued.Status == "":
	default:
		return queued
	}
	if id == "" {
		return queued
	}
	if release, err := mb.GetRelease(id); err == nil && release != nil {
		return release
	}
	return queued
}

// params are the import of the plan's file i, which must have a track, into
// the library source destRoot. The plan and Run share them, so that what a
// client showed is what happens.
func (p Plan) params(i int, destRoot string, cfg rename.Config) ImportParams {
	f := p.Files[i]
	return ImportParams{
		SourcePath:   f.Source,
		DestRoot:     destRoot,
		ReleaseGroup: p.ReleaseGroup,
		Release:      p.Release,
		TrackIndex:   f.Track,
		DiscNumber:   max(p.Release.Tracks[f.Track].DiscNumber, 1),
		TotalDiscs:   max(p.Release.DiscCount, 1),
		CoverArt:     p.Cover,
		RenameConfig: cfg,
	}
}
