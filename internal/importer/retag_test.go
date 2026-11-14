package importer

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// albumFiles writes one file per tag in a folder of its own, and returns
// their paths in the order of the tags.
func albumFiles(t *testing.T, fileTags ...tags.Tag) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, len(fileTags))
	for i := range fileTags {
		paths[i] = filepath.Join(dir, string(rune('a'+i))+".mp3")
		tagstest.WriteMP3(t, paths[i], fileTags[i])
	}
	return paths
}

func TestRetag_WritesOnEachFileTheTagsOfItsTrack(t *testing.T) {
	// The files are not in track order: their track numbers say which is which.
	paths := albumFiles(t,
		tags.Tag{Title: "second", TrackNumber: 2, Genre: "Slowcore"},
		tags.Tag{Title: "third", TrackNumber: 3, Genre: "Slowcore"},
		tags.Tag{Title: "first", TrackNumber: 1, Genre: "Slowcore"},
	)
	group := &musicbrainz.ReleaseGroup{ID: "rg", PrimaryType: "Album", FirstRelease: "1994"}
	var reports [][]FileState

	done := Retag(paths, release("A"), group, nil, func(files []FileProgress) {
		reports = append(reports, states(files))
	}, nil)

	if want := []FileState{FileDone, FileDone, FileDone}; !slices.Equal(states(done), want) {
		t.Fatalf("files ended %v (%+v), want all done", states(done), done)
	}
	for i, title := range []string{"Two", "Three", "One"} {
		tag, err := tags.Read(paths[i])
		if err != nil {
			t.Fatalf("read %s: %v", paths[i], err)
		}
		if done[i].Path != paths[i] || tag.Title != title || tag.Album != "Album A" || tag.MBReleaseID != "A" || tag.MBReleaseGroupID != "rg" {
			t.Errorf("file %d (%s) tagged %q on %q (release %q, group %q), want %q on Album A (release A, group rg)",
				i, done[i].Path, tag.Title, tag.Album, tag.MBReleaseID, tag.MBReleaseGroupID, title)
		}
		// MusicBrainz gives the release no genre: the file keeps its own.
		if tag.Genre != "Slowcore" {
			t.Errorf("file %d: genre %q, want its own kept", i, tag.Genre)
		}
	}
	want := [][]FileState{
		{FileWorking, FilePending, FilePending},
		{FileDone, FileWorking, FilePending},
		{FileDone, FileDone, FileWorking},
	}
	if !slices.EqualFunc(reports, want, slices.Equal[[]FileState]) {
		t.Errorf("reports = %v, want %v", reports, want)
	}
}

// Files with no track number are taken in order. One the release has no track
// for is left as it is: it used to be written as another file's track, or to
// take the retag down with an index out of range.
func TestRetag_AFileTheReleaseHasNoTrackForIsLeftAsItIs(t *testing.T) {
	paths := albumFiles(t,
		tags.Tag{Title: "first"}, tags.Tag{Title: "second"}, tags.Tag{Title: "third"},
		tags.Tag{Title: "hidden track"},
	)

	done := Retag(paths, release("A"), &musicbrainz.ReleaseGroup{}, nil, nil, nil)

	if want := []FileState{FileDone, FileDone, FileDone, FileFailed}; !slices.Equal(states(done), want) {
		t.Fatalf("files ended %v, want the three tracks done and the fourth file failed", states(done))
	}
	if err := done[3].Err; err == nil || !strings.Contains(err.Error(), "no MusicBrainz track") {
		t.Errorf("the fourth file failed with %v, want it to say it has no MusicBrainz track", err)
	}
	for i, title := range []string{"One", "Two", "Three", "hidden track"} {
		if tag, err := tags.Read(paths[i]); err != nil || tag.Title != title {
			t.Errorf("file %d is titled %q (err %v), want %q", i, tag.Title, err, title)
		}
	}
}

// Two discs have a track 1: the file's disc tag tells which one it is. With
// no disc tag, or one the release doesn't have, the first track 1 is taken.
func TestRetag_ADiscTagPicksTheTrackOfThatDisc(t *testing.T) {
	paths := albumFiles(t,
		tags.Tag{TrackNumber: 1, DiscNumber: 2},
		tags.Tag{TrackNumber: 2, DiscNumber: 2},
		tags.Tag{TrackNumber: 1},
		tags.Tag{TrackNumber: 2, DiscNumber: 5},
	)

	Retag(paths, doubleAlbum(), &musicbrainz.ReleaseGroup{}, nil, nil, nil)

	for i, want := range []struct {
		title string
		disc  int
	}{{"Two A", 2}, {"Two B", 2}, {"One A", 1}, {"One B", 1}} {
		tag, err := tags.Read(paths[i])
		if err != nil {
			t.Fatalf("read %s: %v", paths[i], err)
		}
		if tag.Title != want.title || tag.DiscNumber != want.disc || tag.TotalDiscs != 2 {
			t.Errorf("file %d tagged %q, disc %d of %d; want %q, disc %d of 2", i, tag.Title, tag.DiscNumber, tag.TotalDiscs, want.title, want.disc)
		}
	}
}

func TestRetag_StopsBeforeTheNextFileWhenAsked(t *testing.T) {
	paths := albumFiles(t, tags.Tag{Title: "first"}, tags.Tag{Title: "second"})
	asked := 0

	done := Retag(paths, release("A"), &musicbrainz.ReleaseGroup{}, nil, nil, func() bool {
		asked++
		return asked > 1 // after the first file
	})

	if want := []FileState{FileDone, FilePending}; !slices.Equal(states(done), want) {
		t.Fatalf("files ended %v, want the retag stopped after the first", states(done))
	}
	if tag, err := tags.Read(paths[1]); err != nil || tag.Title != "second" {
		t.Errorf("the file not reached is titled %q (err %v), want it untouched", tag.Title, err)
	}
}

// An album tagged as a reissue keeps, retagged as that reissue with the release
// group MusicBrainz gives it, the original date, the release type and the
// release group ID of that group.
func TestRetag_ARetagWithTheReleaseGroupOfTheReleaseKeepsWhatTheGroupSays(t *testing.T) {
	paths := albumFiles(t, tags.Tag{
		Title: "one", TrackNumber: 1, Date: "2011-11-08", OriginalDate: "1975-09-12",
		ReleaseType: "album; Live", MBReleaseID: "A", MBReleaseGroupID: "rg",
	})
	reissue := release("A")
	reissue.Date = "2011-11-08"
	reissue.ReleaseGroup = &musicbrainz.ReleaseGroup{
		ID: "rg", PrimaryType: "Album", SecondaryTypes: []string{"Live"}, FirstRelease: "1975-09-12",
	}

	done := Retag(paths, reissue, reissue.ReleaseGroup, nil, nil, nil)

	if done[0].State != FileDone {
		t.Fatalf("file ended %+v, want done", done[0])
	}
	tag, err := tags.Read(paths[0])
	if err != nil {
		t.Fatalf("read %s: %v", paths[0], err)
	}
	if tag.Date != "2011-11-08" || tag.OriginalDate != "1975-09-12" || tag.ReleaseType != "album; Live" || tag.MBReleaseGroupID != "rg" {
		t.Errorf("file tagged date %q, original date %q, release type %q, release group %q; want 2011-11-08, 1975-09-12, album; Live, rg",
			tag.Date, tag.OriginalDate, tag.ReleaseType, tag.MBReleaseGroupID)
	}
}
