package importer

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// fakeMB serves releases and covers by release ID, and records what is asked.
type fakeMB struct {
	releases map[string]*musicbrainz.ReleaseDetails
	covers   map[string][]byte
	asked    []string // "release <id>" or "cover <id>", in order
}

func (f *fakeMB) GetRelease(id string) (*musicbrainz.ReleaseDetails, error) {
	f.asked = append(f.asked, "release "+id)
	if r, ok := f.releases[id]; ok {
		return r, nil
	}
	return nil, errors.New("unavailable")
}

func (f *fakeMB) GetCoverArt(id string) ([]byte, error) {
	f.asked = append(f.asked, "cover "+id)
	return f.covers[id], nil
}

// A release split into disc folders goes onto its discs: each file gets the
// track, the tags and the path of its own disc, and the plan walks the files
// disc by disc whatever the order the download holds them in.
func TestPrepare_DiscFoldersGoOntoTheirDisc(t *testing.T) {
	completed := t.TempDir()
	d := &downloads.Download{
		ID:               7,
		MBReleaseDetails: doubleAlbum(),
		MBReleaseGroup:   &musicbrainz.ReleaseGroup{ID: "rg", PrimaryType: "Album"},
		// The download's own order: by filename, so both discs' 01 come first.
		Files: files(`@@bob\Double\CD1\01.flac`, `@@bob\Double\CD2\01.flac`,
			`@@bob\Double\CD1\02.flac`, `@@bob\Double\CD2\02.flac`),
	}

	plan, err := Prepare(d, completed, &fakeMB{}, rename.DefaultConfig())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if plan.DownloadID != 7 || plan.Release.ID != "double" || plan.ReleaseGroup.ID != "rg" {
		t.Errorf("plan = download %d, release %q, group %q; want 7, double, rg", plan.DownloadID, plan.Release.ID, plan.ReleaseGroup.ID)
	}
	want := []struct {
		folder, name, numbered string
		track                  int
	}{
		{"CD1", "01.flac", "01.01 · One A", 0},
		{"CD1", "02.flac", "01.02 · One B", 1},
		{"CD2", "01.flac", "02.01 · Two A", 2},
		{"CD2", "02.flac", "02.02 · Two B", 3},
	}
	if len(plan.Files) != len(want) {
		t.Fatalf("%d files planned, want %d", len(plan.Files), len(want))
	}
	for i, w := range want {
		f := plan.Files[i]
		if source := filepath.Join(completed, w.folder, w.name); f.Source != source {
			t.Errorf("file %d: source %s, want %s", i, f.Source, source)
		}
		if f.Track != w.track {
			t.Errorf("file %d (%s): track %d, want %d", i, f.Source, f.Track, w.track)
		}
		if filepath.IsAbs(f.Dest) || !strings.Contains(f.Dest, w.numbered) || !strings.HasSuffix(f.Dest, ".flac") {
			t.Errorf("file %d (%s): destination %q, want a path under the library source holding %q", i, f.Source, f.Dest, w.numbered)
		}
	}
}

// release is a three-track release that carries every field an import reads.
func release(id string) *musicbrainz.ReleaseDetails {
	return &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{ID: id, Title: "Album " + id, Artist: "Artist", ArtistID: "artist", Status: "Official"},
		Tracks:  []musicbrainz.Track{{Position: 1, Title: "One"}, {Position: 2, Title: "Two"}, {Position: 3, Title: "Three"}},
	}
}

// downloaded is a download of release A whose three files slskd wrote under
// completed, each tagged with tag.
func downloaded(t *testing.T, completed string, tag tags.Tag) *downloads.Download {
	t.Helper()
	d := &downloads.Download{
		ID:               1,
		MBReleaseDetails: release("A"),
		MBReleaseGroup:   &musicbrainz.ReleaseGroup{ID: "rg", PrimaryType: "Album", FirstRelease: "1994"},
		Files:            files(`@@bob\Album\01 one.mp3`, `@@bob\Album\02 two.mp3`, `@@bob\Album\03 three.mp3`),
	}
	for _, f := range d.Files {
		tagstest.WriteMP3(t, downloads.ExpectedDiskPath(completed, f.Filename), tag)
	}
	return d
}

// Files tagged as another release are imported as that release, with its
// cover: the cover of the release the download was queued as is not asked for.
func TestPrepare_FilesOfAnotherReleaseAreImportedAsThatRelease(t *testing.T) {
	completed := t.TempDir()
	mb := &fakeMB{
		releases: map[string]*musicbrainz.ReleaseDetails{"B": release("B")},
		covers:   map[string][]byte{"A": []byte("cover A"), "B": []byte("cover B")},
	}

	plan, err := Prepare(downloaded(t, completed, tags.Tag{MBReleaseID: "B"}), completed, mb, rename.DefaultConfig())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if plan.Release.ID != "B" || string(plan.Cover) != "cover B" {
		t.Errorf("plan = release %q with cover %q, want B with cover B", plan.Release.ID, plan.Cover)
	}
	if want := []string{"release B", "cover B"}; !slices.Equal(mb.asked, want) {
		t.Errorf("asked MusicBrainz for %v, want %v", mb.asked, want)
	}
	if !strings.Contains(plan.Files[0].Dest, "Album B") {
		t.Errorf("destination %q is not in release B's album", plan.Files[0].Dest)
	}
}

// The release the download was queued as is kept when the files name it, name
// none, or name one MusicBrainz doesn't return. Its cover is asked for once.
func TestPrepare_KeepsTheQueuedRelease(t *testing.T) {
	tests := []struct {
		name   string
		tagged string // the release ID in the files' tags
		asked  []string
	}{
		{"files tagged as it", "A", []string{"cover A"}},
		{"files naming no release", "", []string{"cover A"}},
		{"files naming a release MusicBrainz doesn't return", "B", []string{"release B", "cover A"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completed := t.TempDir()
			mb := &fakeMB{covers: map[string][]byte{"A": []byte("cover A")}}

			plan, err := Prepare(downloaded(t, completed, tags.Tag{MBReleaseID: tt.tagged}), completed, mb, rename.DefaultConfig())
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}

			if plan.Release.ID != "A" || string(plan.Cover) != "cover A" {
				t.Errorf("plan = release %q with cover %q, want A with cover A", plan.Release.ID, plan.Cover)
			}
			if !slices.Equal(mb.asked, tt.asked) {
				t.Errorf("asked MusicBrainz for %v, want %v", mb.asked, tt.asked)
			}
		})
	}
}

// A download queued before waves kept the label, the status and the artist ID
// of a release is read again, so that the import writes them.
func TestPrepare_ReadsAgainAReleaseMissingWhatTheImportWrites(t *testing.T) {
	completed := t.TempDir()
	d := downloaded(t, completed, tags.Tag{})
	d.MBReleaseDetails.ArtistID = ""
	full := release("A")
	full.Label = "Kranky"
	mb := &fakeMB{releases: map[string]*musicbrainz.ReleaseDetails{"A": full}}

	plan, err := Prepare(d, completed, mb, rename.DefaultConfig())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if plan.Release.Label != "Kranky" || plan.Release.ArtistID != "artist" {
		t.Errorf("plan release = %+v, want the one read again", plan.Release.Release)
	}
}

func TestPrepare_RefusesADownloadWithNoRelease(t *testing.T) {
	d := &downloads.Download{ID: 1, Files: files(`@@bob\Album\01.mp3`)}

	if _, err := Prepare(d, t.TempDir(), &fakeMB{}, rename.DefaultConfig()); !errors.Is(err, ErrNoRelease) {
		t.Errorf("Prepare = %v, want ErrNoRelease", err)
	}
}
