package popup

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// doubleAlbum is a 2-disc release: two tracks on disc 1, two on disc 2.
func doubleAlbum() *musicbrainz.ReleaseDetails {
	return &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{ID: "double", Title: "Double", Artist: "Artist", DiscCount: 2},
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

func tagged(discTracks ...[2]int) []tags.FileInfo {
	out := make([]tags.FileInfo, len(discTracks))
	for i, dt := range discTracks {
		out[i] = tags.FileInfo{Tag: tags.Tag{DiscNumber: dt[0], TrackNumber: dt[1]}}
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
		tags    []tags.FileInfo
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
			if got := matchTracks(tt.files, tt.tags, tt.release); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("matchTracks = %v, want %v", got, tt.want)
			}
		})
	}
}

// A release split into disc folders imports each disc onto its own tracks:
// the preview, the tags and the path the import gets are disc 2's for CD2.
func TestImport_DiscFoldersImportOntoTheirDisc(t *testing.T) {
	dl := sampleDownload()
	dl.MBReleaseDetails = doubleAlbum()
	dl.SlskdDirectory = `@@testuser\Double`
	// The download's own order: by filename, so both discs' 01 come first
	dl.Files = files(`@@testuser\Double\CD1\01.flac`, `@@testuser\Double\CD2\01.flac`,
		`@@testuser\Double\CD1\02.flac`, `@@testuser\Double\CD2\02.flac`)
	completed := t.TempDir()
	m := New(dl, completed, []string{"/music"}, nil, rename.DefaultConfig())
	m.SetSize(120, 40)
	h := testutil.NewPopupHarness(m)
	h.SendMsg(TagsReadMsg{DownloadID: dl.ID, Tags: make([]tags.FileInfo, len(dl.Files))})
	h.SendMsg(CoverArtFetchedMsg{DownloadID: dl.ID, ReleaseID: "double"})
	h.SendEnter() // path preview

	wantTitles := []string{"One A", "One B", "Two A", "Two B"}
	wantNumbers := []string{"01.01", "01.02", "02.01", "02.02"}
	wantDiscs := []int{1, 1, 2, 2}
	wantFolders := []string{"CD1", "CD1", "CD2", "CD2"}
	if len(m.filePaths) != len(wantTitles) {
		t.Fatalf("%d paths, want %d", len(m.filePaths), len(wantTitles))
	}
	for i, pm := range m.filePaths {
		wantSource := filepath.Join(completed, wantFolders[i], filepath.Base(pm.OldPath))
		if pm.OldPath != wantSource {
			t.Errorf("file %d: source %s, want %s", i, pm.OldPath, wantSource)
		}
		if !strings.Contains(pm.NewPath, wantNumbers[i]+" · "+wantTitles[i]) {
			t.Errorf("file %d (%s): destination %s, want track %s %q", i, pm.OldPath, pm.NewPath, wantNumbers[i], wantTitles[i])
		}
		params := m.importParams("/music", pm.TrackIndex, pm.OldPath)
		if got := params.Release.Tracks[params.TrackIndex].Title; got != wantTitles[i] || params.DiscNumber != wantDiscs[i] {
			t.Errorf("file %d: imported as %q disc %d, want %q disc %d", i, got, params.DiscNumber, wantTitles[i], wantDiscs[i])
		}
	}

	// The import walks the files in the same order as the preview
	h.SendEnter()
	for i := range m.importStatus {
		if want := m.filePaths[i].OldPath; downloads.ExpectedDiskPath(completed, m.importStatus[i].Filename) != want {
			t.Errorf("import %d is %s, preview %s", i, m.importStatus[i].Filename, want)
		}
	}
}

// The tag preview compares each file's title with its own track's: disc 2's
// files already titled as disc 2's tracks have nothing to change.
func TestImport_TagPreviewComparesEachFileWithItsTrack(t *testing.T) {
	dl := sampleDownload()
	dl.MBReleaseDetails = doubleAlbum()
	dl.Files = files(`@@testuser\Double\CD2\01.flac`, `@@testuser\Double\CD2\02.flac`)
	m := New(dl, t.TempDir(), []string{"/music"}, nil, rename.DefaultConfig())
	h := testutil.NewPopupHarness(m)

	fileTags := tagged([2]int{2, 1}, [2]int{2, 2})
	fileTags[0].Title, fileTags[1].Title = "Two A", "Two B"
	h.SendMsg(TagsReadMsg{DownloadID: dl.ID, Tags: fileTags})

	for _, d := range m.tagDiffs {
		if d.Field == "Track Titles" && d.Changed {
			t.Error("titles reported changed though each file has its track's title")
		}
	}

	fileTags[1].Title = "One B"
	h.SendMsg(TagsReadMsg{DownloadID: dl.ID, Tags: fileTags})
	for _, d := range m.tagDiffs {
		if d.Field == "Track Titles" && !d.Changed {
			t.Error("a wrong title not reported")
		}
	}
}
