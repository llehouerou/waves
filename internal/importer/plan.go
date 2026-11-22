package importer

import (
	"errors"
	"io/fs"
	"os"

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
	DownloadID int64
	// Release is the files' own release when their tags name another; it is
	// settled together with ReleaseGroup.
	Release *musicbrainz.ReleaseDetails
	// ReleaseGroup is Release's group: the one MusicBrainz returned with it
	// when it was fetched, the one stored on the download when the queued
	// release is kept.
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
// as together with its release group, fetches its cover, matches each file to
// a track of the release (see matchTracks) and names its destination. It
// waits for the disk and for MusicBrainz.
func Prepare(d *downloads.Download, completedPath string, mb Catalogue, cfg rename.Config) (Plan, error) {
	if d.MBReleaseDetails == nil {
		return Plan{}, ErrNoRelease
	}
	plan := Plan{DownloadID: d.ID}

	sorted := downloads.SortFilesByTrackNumber(d.Files)
	sources := make([]string, len(sorted))
	gone := make(map[int]bool)
	for i, f := range sorted {
		sources[i] = downloads.ExpectedDiskPath(completedPath, f.Filename)
		// Only a file that is not there is gone; one that can't be read is left to ReadTags and Run.
		if _, err := os.Stat(sources[i]); errors.Is(err, fs.ErrNotExist) {
			gone[i] = true
		}
	}
	fileTags := ReadTags(sources)
	plan.Files = make([]PlannedFile, len(sorted))
	for i, f := range sorted {
		plan.Files[i] = PlannedFile{Filename: f.Filename, Source: sources[i], Tags: fileTags[i]}
	}

	plan.Release, plan.ReleaseGroup = settleRelease(d.MBReleaseDetails, d.MBReleaseGroup, fileTags, mb)
	if plan.Release.ID != "" {
		// A cover that can't be fetched is no reason not to import.
		plan.Cover, _ = mb.GetCoverArt(plan.Release.ID) //nolint:errcheck // as above
	}

	for i, track := range matchTracks(sorted, fileTags, gone, plan.Release) {
		plan.Files[i].Track = track
		if track >= 0 {
			plan.Files[i].Dest = DestPath(plan.params(i, "", cfg))
		}
	}
	return plan, nil
}

// ReadTags reads the tags of the files at paths, in their order. A file whose
// tags can't be read has none but its Path.
func ReadTags(paths []string) []tags.Tag {
	fileTags := make([]tags.Tag, len(paths))
	for i, path := range paths {
		fileTags[i] = tags.Tag{Path: path}
		if tag, err := tags.Read(path); err == nil {
			fileTags[i] = *tag
		}
	}
	return fileTags
}

// settleRelease returns the release the files are imported as and its release
// group: the one their tags name when it is not the one the download was
// queued as, which is itself read again when it lacks fields the import writes
// (a download queued before waves kept them). A release fetched comes with its
// own group. A release MusicBrainz doesn't return, or returns with no group,
// leaves the queued one in place with the group stored with it.
func settleRelease(
	queued *musicbrainz.ReleaseDetails, queuedGroup *musicbrainz.ReleaseGroup, fileTags []tags.Tag, mb Catalogue,
) (*musicbrainz.ReleaseDetails, *musicbrainz.ReleaseGroup) {
	named := ""
	for i := range fileTags {
		// The first file that names a release speaks for them all, and a list
		// names its first release.
		if named = tags.FirstValue(fileTags[i].MBReleaseID); named != "" {
			break
		}
	}

	id := queued.ID
	switch {
	case named != "" && named != queued.ID:
		id = named
	case queued.ArtistID == "" || queued.Label == "" && queued.Status == "" || lacksTrackArtistIDs(queued) || lacksTrackFormats(queued):
	default:
		return queued, queuedGroup
	}
	if id == "" {
		return queued, queuedGroup
	}
	if release, err := mb.GetRelease(id); err == nil && release != nil && release.ReleaseGroup != nil {
		return release, release.ReleaseGroup
	}
	return queued, queuedGroup
}

// lacksTrackArtistIDs reports whether a track of release credits no artist
// ID: MusicBrainz gives one for every track, so the release was queued before
// waves kept them as a list (Track.ArtistIDs).
func lacksTrackArtistIDs(release *musicbrainz.ReleaseDetails) bool {
	for i := range release.Tracks {
		if len(release.Tracks[i].ArtistIDs) == 0 {
			return true
		}
	}
	return false
}

// lacksTrackFormats reports whether release has formats while none of its
// tracks carries the format of its medium: the release was queued before
// waves kept it (Track.MediumFormat).
func lacksTrackFormats(release *musicbrainz.ReleaseDetails) bool {
	if release.Formats == "" {
		return false
	}
	for i := range release.Tracks {
		if release.Tracks[i].MediumFormat != "" {
			return false
		}
	}
	return true
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
