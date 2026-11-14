package importer

import (
	"reflect"
	"testing"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/tags"
)

// doubleAlbum is a 2-disc release: two tracks on disc 1, two on disc 2.
func doubleAlbum() *musicbrainz.ReleaseDetails {
	return &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{ID: "double", Title: "Double", Artist: "Artist", ArtistID: "artist", Status: "Official", DiscCount: 2},
		Tracks: []musicbrainz.Track{
			{DiscNumber: 1, Position: 1, Title: "One A"},
			{DiscNumber: 1, Position: 2, Title: "One B"},
			{DiscNumber: 2, Position: 1, Title: "Two A"},
			{DiscNumber: 2, Position: 2, Title: "Two B"},
		},
	}
}

func files(names ...string) []downloads.DownloadFile {
	out := make([]downloads.DownloadFile, len(names))
	for i, n := range names {
		out[i] = downloads.DownloadFile{ID: int64(i + 1), Filename: n, Status: downloads.StatusCompleted}
	}
	return out
}

func tagged(discTracks ...[2]int) []tags.Tag {
	out := make([]tags.Tag, len(discTracks))
	for i, dt := range discTracks {
		out[i] = tags.Tag{DiscNumber: dt[0], TrackNumber: dt[1]}
	}
	return out
}

func TestMatchTracks(t *testing.T) {
	discFolders := files(`@@u\Double\CD1\01.flac`, `@@u\Double\CD1\02.flac`, `@@u\Double\CD2\01.flac`, `@@u\Double\CD2\02.flac`)
	oneFolder := files(`@@u\Double\01 a.flac`, `@@u\Double\01 b.flac`, `@@u\Double\01 c.flac`, `@@u\Double\01 d.flac`)
	single := &musicbrainz.ReleaseDetails{Release: musicbrainz.Release{DiscCount: 1}, Tracks: []musicbrainz.Track{{Position: 1}, {Position: 2}}}

	tests := []struct {
		name    string
		files   []downloads.DownloadFile
		tags    []tags.Tag
		release *musicbrainz.ReleaseDetails
		want    []int
	}{
		{"disc folders, no tags", discFolders, nil, doubleAlbum(), []int{0, 1, 2, 3}},
		{"one folder, no tags: the release in order", oneFolder, nil, doubleAlbum(), []int{0, 1, 2, 3}},
		{
			"complete tags beat the filenames", oneFolder,
			tagged([2]int{2, 2}, [2]int{1, 1}, [2]int{2, 1}, [2]int{1, 2}), doubleAlbum(), []int{3, 0, 2, 1},
		},
		{
			"a missing track number falls back to folder and position", oneFolder,
			tagged([2]int{2, 2}, [2]int{1, 1}, [2]int{2, 0}, [2]int{1, 2}), doubleAlbum(), []int{0, 1, 2, 3},
		},
		{
			"duplicate tags fall back to folder and position", discFolders,
			tagged([2]int{1, 1}, [2]int{1, 1}, [2]int{2, 1}, [2]int{2, 2}), doubleAlbum(), []int{0, 1, 2, 3},
		},
		{
			"a track not in the release falls back", discFolders,
			tagged([2]int{1, 1}, [2]int{1, 2}, [2]int{2, 1}, [2]int{3, 1}), doubleAlbum(), []int{0, 1, 2, 3},
		},
		{
			"no disc tag on a multi-disc release falls back", oneFolder,
			tagged([2]int{0, 2}, [2]int{0, 1}, [2]int{2, 1}, [2]int{2, 2}), doubleAlbum(), []int{0, 1, 2, 3},
		},
		{
			"no disc tag on a single-disc release means disc 1", files(`@@u\A\x.flac`, `@@u\A\y.flac`),
			tagged([2]int{0, 2}, [2]int{0, 1}), single, []int{1, 0},
		},
		{
			"a disc folder beyond the release has no tracks", files(`@@u\A\CD3\01.flac`),
			nil, doubleAlbum(), []int{-1},
		},
		{
			"a disc folder with more files than its disc", files(`@@u\A\CD1\01.flac`, `@@u\A\CD1\02.flac`, `@@u\A\CD1\03.flac`),
			nil, doubleAlbum(), []int{0, 1, -1},
		},
		{
			"two folders naming one disc share its tracks", files(`@@u\A\CD1\01.flac`, `@@u\A\Disc 1\01.flac`, `@@u\A\Disc 1\02.flac`),
			nil, doubleAlbum(), []int{0, 1, -1},
		},
		{
			"a lone disc folder is that disc", files(`@@u\A\CD2\01.flac`, `@@u\A\CD2\02.flac`),
			nil, doubleAlbum(), []int{2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchTracks(tt.files, tt.tags, nil, tt.release); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("matchTracks = %v, want %v", got, tt.want)
			}
		})
	}
}

// A file an earlier import moved out of the download folder has no tags left:
// the files still there match by their tags, and the gone one takes the track
// its filename names, or the next free one of its disc.
func TestMatchTracks_FilesGoneFromTheDownloadFolder(t *testing.T) {
	four := &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{DiscCount: 1},
		Tracks:  []musicbrainz.Track{{Position: 1}, {Position: 2}, {Position: 3}, {Position: 4}},
	}
	noTag := [2]int{0, 0}

	tests := []struct {
		name    string
		files   []downloads.DownloadFile
		tags    []tags.Tag
		gone    map[int]bool
		release *musicbrainz.ReleaseDetails
		want    []int
	}{
		{
			"a gone file takes the track its filename names", files(`@@u\A\01.mp3`, `@@u\A\02.mp3`, `@@u\A\04.mp3`),
			tagged(noTag, noTag, [2]int{0, 4}), map[int]bool{0: true, 1: true}, four, []int{0, 1, 3},
		},
		{
			"a gone file with no number takes the next free track", files(`@@u\A\01.mp3`, `@@u\A\03.mp3`, `@@u\A\bonus.mp3`),
			tagged([2]int{0, 1}, [2]int{0, 3}, noTag), map[int]bool{2: true}, four, []int{0, 2, 1},
		},
		{
			"a gone file whose number another file took takes the next free track", files(`@@u\A\01.mp3`, `@@u\A\02.mp3`),
			tagged([2]int{0, 2}, noTag), map[int]bool{1: true}, four, []int{1, 0},
		},
		{
			"a gone file in a disc folder takes a track of that disc", files(`@@u\A\CD1\01.flac`, `@@u\A\CD2\x.flac`),
			tagged([2]int{1, 1}, noTag), map[int]bool{1: true}, doubleAlbum(), []int{0, 2},
		},
		{
			"a gone file with no free track has none", files(`@@u\A\01.flac`, `@@u\A\02.flac`, `@@u\A\03.flac`),
			tagged([2]int{0, 1}, [2]int{0, 2}, noTag), map[int]bool{2: true},
			&musicbrainz.ReleaseDetails{Release: musicbrainz.Release{DiscCount: 1}, Tracks: []musicbrainz.Track{{Position: 1}, {Position: 2}}},
			[]int{0, 1, -1},
		},
		{
			"a file still there with no track tag falls back to folder and position", files(`@@u\A\01.mp3`, `@@u\A\02.mp3`, `@@u\A\04.mp3`),
			tagged(noTag, noTag, [2]int{0, 4}), map[int]bool{1: true}, four, []int{0, 1, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchTracks(tt.files, tt.tags, tt.gone, tt.release); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("matchTracks = %v, want %v", got, tt.want)
			}
		})
	}
}
