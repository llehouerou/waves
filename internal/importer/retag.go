package importer

import (
	"slices"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/tags"
)

// Retag writes the tags of a release on the files of a library album, in
// place, one file after the other, and returns how each ended. cover, when
// there is one, is embedded in each file.
//
// report, when not nil, is called before each file with where every file is.
// stop, when not nil, is asked before each file whether to end there: the
// files not reached stay pending.
func Retag(paths []string, release *musicbrainz.ReleaseDetails, group *musicbrainz.ReleaseGroup, cover []byte, report func([]FileProgress), stop func() bool) []FileProgress {
	files := make([]FileProgress, len(paths))
	for i, path := range paths {
		files[i] = FileProgress{Path: path}
	}

	for i, path := range paths {
		if stop != nil && stop() {
			break
		}
		files[i].State = FileWorking
		if report != nil {
			report(slices.Clone(files))
		}

		// A file whose tags can't be read has none to go by, nor to keep.
		current := ReadTags([]string{path})[0]
		track := retagTrack(&current, i, release)
		if track < 0 {
			files[i].State, files[i].Err = FileFailed, errNoTrack
			continue
		}
		data := BuildTagData(BuildTagDataParams{
			ReleaseGroup:  group,
			Release:       release,
			Track:         &release.Tracks[track],
			DiscNumber:    max(release.Tracks[track].DiscNumber, 1),
			TotalDiscs:    max(release.DiscCount, 1),
			ExistingGenre: current.Genre,
			CoverArt:      cover,
		})
		if err := RetagFile(path, data); err != nil {
			files[i].State, files[i].Err = FileFailed, err
			continue
		}
		files[i].State = FileDone
	}
	return files
}

// retagTrack is the index in release.Tracks of the track the i-th file of a
// library album is retagged as: the one its track and disc numbers name (the
// first of that number on any disc when its disc names none), or the i-th
// when it has no track number. -1 when the release has no track for it.
func retagTrack(current *tags.Tag, i int, release *musicbrainz.ReleaseDetails) int {
	if current.TrackNumber > 0 {
		anyDisc := -1
		for j := range release.Tracks {
			track := &release.Tracks[j]
			if track.Position != current.TrackNumber {
				continue
			}
			if current.DiscNumber == 0 || track.DiscNumber == current.DiscNumber {
				return j
			}
			if anyDisc < 0 {
				anyDisc = j
			}
		}
		if anyDisc >= 0 {
			return anyDisc
		}
	}
	if i < len(release.Tracks) {
		return i
	}
	return -1
}
