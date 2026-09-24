package popup

import (
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/tags"
)

// discTrack is a track's place in a release: its disc and its position on it.
type discTrack struct{ disc, position int }

// trackDisc is the disc a release track is on, 1 when MusicBrainz gave none.
func trackDisc(t *musicbrainz.Track) int {
	return max(t.DiscNumber, 1)
}

// matchTracks gives each file, in track order (downloads.SortFilesByTrackNumber),
// the index of its track in release.Tracks, or -1 when it has none: such a file
// is never imported, and no file ever lands on another's track. fileTags are
// the files' tags in the same order.
//
// Files whose tags all carry a track number, unique and in the release, match
// by (disc, track) from their tags. Otherwise a file matches by its position in
// its folder: in a disc folder ("CD2"), the tracks of that disc; in any other
// folder, the release's tracks from the first.
func matchTracks(files []downloads.DownloadFile, fileTags []tags.FileInfo, release *musicbrainz.ReleaseDetails) []int {
	if release == nil {
		return nil
	}
	if len(fileTags) == len(files) {
		if matches := matchByTags(fileTags, release); matches != nil {
			return matches
		}
	}
	return matchByFolder(files, release)
}

// matchByTags matches files by their disc and track tags, or returns nil when
// a file's tags don't name exactly one of the release's tracks. A missing disc
// tag means disc 1 only for a single-disc release.
func matchByTags(fileTags []tags.FileInfo, release *musicbrainz.ReleaseDetails) []int {
	index := make(map[discTrack]int, len(release.Tracks))
	for i := range release.Tracks {
		index[discTrack{trackDisc(&release.Tracks[i]), release.Tracks[i].Position}] = i
	}

	matches := make([]int, len(fileTags))
	taken := make(map[discTrack]bool, len(fileTags))
	for i := range fileTags {
		key := discTrack{fileTags[i].DiscNumber, fileTags[i].TrackNumber}
		if key.disc == 0 && release.DiscCount <= 1 {
			key.disc = 1
		}
		trackIndex, ok := index[key]
		if key.position <= 0 || !ok || taken[key] {
			return nil
		}
		taken[key] = true
		matches[i] = trackIndex
	}
	return matches
}

// matchByFolder matches each file to the track at its position in its folder.
func matchByFolder(files []downloads.DownloadFile, release *musicbrainz.ReleaseDetails) []int {
	all := make([]int, len(release.Tracks))
	byDisc := make(map[int][]int)
	for i := range release.Tracks {
		all[i] = i
		disc := trackDisc(&release.Tracks[i])
		byDisc[disc] = append(byDisc[disc], i)
	}

	matches := make([]int, len(files))
	positions := make(map[string]int) // files seen so far per folder
	for i := range files {
		dir := downloads.SlskdFolder(files[i].Filename)
		tracks := all
		if disc := downloads.DiscNumber(downloads.ExtractFolderName(dir)); disc > 0 {
			tracks = byDisc[disc]
		}
		matches[i] = -1
		if pos := positions[dir]; pos < len(tracks) {
			matches[i] = tracks[pos]
		}
		positions[dir]++
	}
	return matches
}
