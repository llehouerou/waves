package importer

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

// folderDisc is the disc a file's folder names ("CD2"), 0 when it names none.
func folderDisc(f *downloads.DownloadFile) int {
	return downloads.DiscNumber(downloads.ExtractFolderName(downloads.SlskdFolder(f.Filename)))
}

// matchTracks gives each file, in track order (downloads.SortFilesByTrackNumber),
// the index of its track in release.Tracks, or -1 when it has none: such a file
// is never imported, and no file ever lands on another's track. fileTags are
// the files' tags in the same order; gone holds the files missing from the
// download folder, which an earlier import may have moved.
//
// Files still there whose tags all carry a track number, unique and in the
// release, match by (disc, track) from their tags, and the gone ones by their
// filename (see matchByTags). Otherwise a file matches by its position in its
// folder: in a disc folder ("CD2"), the tracks of that disc; in any other
// folder, the release's tracks from the first.
func matchTracks(files []downloads.DownloadFile, fileTags []tags.Tag, gone map[int]bool, release *musicbrainz.ReleaseDetails) []int {
	if release == nil {
		return nil
	}
	if len(fileTags) == len(files) {
		if matches := matchByTags(files, fileTags, gone, release); matches != nil {
			return matches
		}
	}
	return matchByFolder(files, release)
}

// matchByTags matches the files still there by their disc and track tags, or
// returns nil when one's tags don't name exactly one of the release's tracks.
// A missing disc tag means disc 1 only for a single-disc release.
//
// A gone file has no tags left: it takes the track its filename numbers on its
// disc, else the next free track of its disc (see matchByFolder), else none.
func matchByTags(files []downloads.DownloadFile, fileTags []tags.Tag, gone map[int]bool, release *musicbrainz.ReleaseDetails) []int {
	index := make(map[discTrack]int, len(release.Tracks))
	for i := range release.Tracks {
		index[discTrack{trackDisc(&release.Tracks[i]), release.Tracks[i].Position}] = i
	}

	matches := make([]int, len(fileTags))
	taken := make(map[int]bool, len(fileTags)) // track indexes
	for i := range fileTags {
		matches[i] = -1
		if gone[i] {
			continue
		}
		key := discTrack{fileTags[i].DiscNumber, fileTags[i].TrackNumber}
		if key.disc == 0 && release.DiscCount <= 1 {
			key.disc = 1
		}
		trackIndex, ok := index[key]
		if key.position <= 0 || !ok || taken[trackIndex] {
			return nil
		}
		taken[trackIndex] = true
		matches[i] = trackIndex
	}
	for i := range files {
		if !gone[i] {
			continue
		}
		key := discTrack{folderDisc(&files[i]), downloads.ParseTrackNumber(files[i].Filename)}
		if key.disc == 0 && release.DiscCount <= 1 {
			key.disc = 1
		}
		if trackIndex, ok := index[key]; ok && key.position > 0 && !taken[trackIndex] {
			taken[trackIndex] = true
			matches[i] = trackIndex
		}
	}
	for i := range files {
		if !gone[i] || matches[i] >= 0 {
			continue
		}
		disc := folderDisc(&files[i]) // 0: the whole release
		for t := range release.Tracks {
			if !taken[t] && (disc == 0 || trackDisc(&release.Tracks[t]) == disc) {
				taken[t] = true
				matches[i] = t
				break
			}
		}
	}
	return matches
}

// matchByFolder matches each file to the track at its position among the files
// of its disc: the disc its folder names, or the whole release for a folder that
// names none. Two folders naming the same disc ("CD1", "Disc 1") share its
// tracks, so no two files ever get the same one.
func matchByFolder(files []downloads.DownloadFile, release *musicbrainz.ReleaseDetails) []int {
	byDisc := make(map[int][]int) // disc 0: the whole release
	for i := range release.Tracks {
		disc := trackDisc(&release.Tracks[i])
		byDisc[disc] = append(byDisc[disc], i)
		byDisc[0] = append(byDisc[0], i)
	}

	matches := make([]int, len(files))
	positions := make(map[int]int) // files seen so far per disc
	for i := range files {
		disc := folderDisc(&files[i])
		matches[i] = -1
		if pos := positions[disc]; pos < len(byDisc[disc]) {
			matches[i] = byDisc[disc][pos]
		}
		positions[disc]++
	}
	return matches
}
