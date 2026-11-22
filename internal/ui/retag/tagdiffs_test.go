package retag

import (
	"testing"

	"github.com/llehouerou/waves/internal/core"
)

// previewLines are the lines of the tag preview of the files lib holds, on the
// release mb gives, by field.
func previewLines(t *testing.T, lib *fakeLibrary, mb fakeMB) map[string]TagDiff {
	t.Helper()
	mb.covers = trustCovers()
	_, m := previewingPopupOnMB(t, lib, mb)
	lines := make(map[string]TagDiff, len(m.tagDiffs))
	for _, d := range m.tagDiffs {
		lines[d.Field] = d
	}
	return lines
}

// trustTagged are the tags of the three files of "trust" holding every value
// the retag as the release "release" writes on them.
func trustTagged() []core.FileTags {
	fileTags := albumTags()
	for i, title := range []string{"Amazing Grace", "Canada", "Candy Girl"} {
		f := &fileTags[i]
		f.Artist, f.AlbumArtist, f.Album, f.Title = "Low", "Low", "Trust", title
		f.Date, f.OriginalDate, f.Genre, f.ReleaseType = "2011-11-08", "2002-09-24", "Slowcore", "album; live"
		f.MBReleaseGroupID = "rg"
	}
	return fileTags
}

func TestRetag_FilesHoldingWhatTheRetagWritesHaveNoLineChanged(t *testing.T) {
	lines := previewLines(t, &fakeLibrary{tags: trustTagged()}, fakeMB{})

	for field, d := range lines {
		if d.Changed {
			t.Errorf("%s: %q -> %q marked changed", field, d.OldValue, d.NewValue)
		}
	}
}

// A file the release has no track for is left as it is by the retag: tagged
// as another album, it changes no line and shows in no current value.
func TestRetag_AFileWithNoTrackCountsInNoLine(t *testing.T) {
	other := albumTags()[0]
	other.TrackNumber = 0 // and no fourth track to be taken as
	other.Artist, other.AlbumArtist, other.Album, other.Title = "Slowdive", "Slowdive", "Souvlaki", "Alison"
	other.Date, other.OriginalDate, other.Genre, other.ReleaseType = "1993-05-17", "1993-05-17", "Shoegaze", "album"
	other.Label, other.Media, other.MBArtistID, other.MBReleaseID, other.MBReleaseGroupID = "Creation", "CD", "idS", "souvlaki", "rgS"

	alone := previewLines(t, &fakeLibrary{tags: trustTagged()}, fakeMB{})
	lines := previewLines(t, &fakeLibrary{tags: append(trustTagged(), other)}, fakeMB{})

	for field, d := range lines {
		if d.Changed {
			t.Errorf("%s: %q -> %q marked changed", field, d.OldValue, d.NewValue)
		}
		if d.OldValue != alone[field].OldValue {
			t.Errorf("%s: current value %q, want %q as with the three files alone", field, d.OldValue, alone[field].OldValue)
		}
	}
}

// One file holding the release's value and two holding none: the retag
// writes it on the two others.
func TestRetag_FilesWithNoValueAreChanged(t *testing.T) {
	fileTags := albumTags()
	fileTags[0].AlbumArtist, fileTags[0].Album, fileTags[0].Date = "Low", "Trust", "2011-11-08"
	for i := 1; i < len(fileTags); i++ {
		fileTags[i].AlbumArtist, fileTags[i].Album, fileTags[i].MBReleaseID = "", "", ""
	}
	lines := previewLines(t, &fakeLibrary{tags: fileTags}, fakeMB{})

	for _, field := range []string{"Album Artist", "Album", "Date", "MB Release ID"} {
		if d := lines[field]; d.OldValue != "(multiple)" || !d.Changed {
			t.Errorf("%s: %q -> %q (changed %v), want (multiple), changed", field, d.OldValue, d.NewValue, d.Changed)
		}
	}
}

// The lines written from each file's track count the files with no value too.
func TestRetag_FilesWithNoValueCountInTheTrackLines(t *testing.T) {
	tracks := []core.MBTrack{
		{Position: 1, Title: "Amazing Grace", MediumFormat: "CD", ArtistIDs: []string{"idA"}},
		{Position: 2, Title: "Canada", MediumFormat: "CD", ArtistIDs: []string{"idA"}},
		{Position: 3, Title: "Candy Girl", MediumFormat: "CD", ArtistIDs: []string{"idA"}},
	}
	fileTags := albumTags()
	fileTags[0].Media, fileTags[0].MBArtistID = "CD", "idA"
	lines := previewLines(t, &fakeLibrary{tags: fileTags}, fakeMB{tracks: tracks})

	for _, field := range []string{"Media", "MB Artist ID"} {
		if d := lines[field]; d.OldValue != "(multiple)" || !d.Changed {
			t.Errorf("%s: %q -> %q (changed %v), want (multiple), changed", field, d.OldValue, d.NewValue, d.Changed)
		}
	}
}

// The Artist line compares each file with the artist of its track, the
// release's when the track has none.
func TestRetag_TheArtistOfEachFileIsComparedWithItsTracksArtist(t *testing.T) {
	// artistLine is the Artist line of files tagged with artists, on a release
	// by Low whose first track is by Low & Mimi and whose two others have no
	// artist of their own.
	artistLine := func(artists ...string) TagDiff {
		tracks := []core.MBTrack{
			{Position: 1, Title: "Amazing Grace", Artist: "Low & Mimi"},
			{Position: 2, Title: "Canada"},
			{Position: 3, Title: "Candy Girl"},
		}
		fileTags := trustTagged()
		for i, artist := range artists {
			fileTags[i].Artist = artist
		}
		return previewLines(t, &fakeLibrary{tags: fileTags}, fakeMB{tracks: tracks})["Artist"]
	}

	if d := artistLine("Low & Mimi", "Low", "Low"); d.OldValue != "(multiple)" || d.NewValue != "(multiple)" || d.Changed {
		t.Errorf("files tagged as their tracks: %q -> %q (changed %v), want (multiple) unchanged", d.OldValue, d.NewValue, d.Changed)
	}
	if d := artistLine("Low & Mimi", "Low", "Slowdive"); !d.Changed {
		t.Error("a file tagged with another artist not reported changed")
	}
	if d := artistLine("Low & Mimi", "Low", ""); !d.Changed {
		t.Error("a file with no artist not reported changed")
	}
	if d := artistLine("Low", "Low & Mimi", "Low"); !d.Changed {
		t.Error("files tagged with each other's artist not reported changed")
	}

	if d := previewLines(t, &fakeLibrary{tags: trustTagged()}, fakeMB{})["Artist"]; d.OldValue != "Low" || d.NewValue != "Low" || d.Changed {
		t.Errorf("files tagged Low, tracks by the release's artist: %q -> %q (changed %v), want Low -> Low unchanged", d.OldValue, d.NewValue, d.Changed)
	}
}

// The Track Titles line compares each file with the title of the track its
// track number names, whatever its place in the album.
func TestRetag_TheTitleOfEachFileIsComparedWithItsTracksTitle(t *testing.T) {
	type file struct {
		track int
		title string
	}
	titlesChanged := func(files ...file) bool {
		fileTags := make([]core.FileTags, len(files))
		for i, f := range files {
			fileTags[i] = trustTagged()[0]
			fileTags[i].TrackNumber, fileTags[i].Title = f.track, f.title
		}
		return previewLines(t, &fakeLibrary{tags: fileTags}, fakeMB{})["Track Titles"].Changed
	}

	if titlesChanged(file{2, "Canada"}, file{1, "Amazing Grace"}, file{3, "Candy Girl"}) {
		t.Error("files out of track order titled as their tracks reported changed")
	}
	if !titlesChanged(file{2, "Canada"}, file{1, "Amazing Grace"}, file{3, "Candy"}) {
		t.Error("a file with another title not reported changed")
	}
	if !titlesChanged(file{2, "Amazing Grace"}, file{1, "Canada"}, file{3, "Candy Girl"}) {
		t.Error("files titled as each other's tracks not reported changed")
	}
}

// A release and a release group with no genre leave each file its own: the
// Genre line never changes. With a genre, every file must hold it.
func TestRetag_TheGenreLineChangesOnlyWhenTheRetagWritesAGenre(t *testing.T) {
	genreLine := func(mb fakeMB, genres ...string) TagDiff {
		fileTags := trustTagged()
		for i, genre := range genres {
			fileTags[i].Genre = genre
		}
		return previewLines(t, &fakeLibrary{tags: fileTags}, mb)["Genre"]
	}
	none := fakeMB{noGenre: true}

	if d := genreLine(none, "Slowcore", "Slowcore", "Slowcore"); d.NewValue != "Slowcore (preserved)" || d.Changed {
		t.Errorf("no genre, files of one genre: %q -> %q (changed %v), want Slowcore (preserved) unchanged", d.OldValue, d.NewValue, d.Changed)
	}
	if d := genreLine(none, "Slowcore", "Rock", ""); d.Changed {
		t.Errorf("no genre, files of several genres: %q -> %q marked changed", d.OldValue, d.NewValue)
	}
	if d := genreLine(none, "", "", ""); d.Changed {
		t.Errorf("no genre, files with none: %q -> %q marked changed", d.OldValue, d.NewValue)
	}
	if d := genreLine(fakeMB{}, "Slowcore", "Slowcore", "Rock"); !d.Changed {
		t.Error("a file of another genre not reported changed")
	}
	if d := genreLine(fakeMB{}, "Slowcore", "Slowcore", ""); !d.Changed {
		t.Error("a file with no genre not reported changed")
	}
}
