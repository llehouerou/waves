package tags

import (
	"testing"
)

// textFields lists every text tag of a Tag, by name.
func textFields(t *Tag) map[string]*string {
	return map[string]*string{
		"Title": &t.Title, "Artist": &t.Artist, "AlbumArtist": &t.AlbumArtist,
		"Album": &t.Album, "Genre": &t.Genre, "Date": &t.Date, "OriginalDate": &t.OriginalDate,
		"ArtistSortName": &t.ArtistSortName, "Label": &t.Label, "CatalogNumber": &t.CatalogNumber,
		"Barcode": &t.Barcode, "Media": &t.Media, "ReleaseStatus": &t.ReleaseStatus,
		"ReleaseType": &t.ReleaseType, "Script": &t.Script, "Country": &t.Country, "ISRC": &t.ISRC,
		"MBArtistID": &t.MBArtistID, "MBReleaseID": &t.MBReleaseID,
		"MBReleaseGroupID": &t.MBReleaseGroupID, "MBRecordingID": &t.MBRecordingID,
		"MBTrackID": &t.MBTrackID,
	}
}

// dirtyTags returns fullTestTags with a tab, a bell, a newline and a
// non-breaking space in every text tag.
func dirtyTags() *Tag {
	tag := fullTestTags()
	for _, field := range textFields(tag) {
		v := *field
		*field = v[:2] + "\t\x07" + v[2:] + "\n\u00a0"
	}
	return tag
}

// assertEveryTextTagClean checks that every text tag read is the one of
// fullTestTags, with the control characters and the non-breaking space gone.
func assertEveryTextTagClean(t *testing.T, path string) {
	t.Helper()
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	gotFields := textFields(got)
	for name, want := range textFields(fullTestTags()) {
		if *gotFields[name] != *want {
			t.Errorf("%s = %q, want %q", name, *gotFields[name], *want)
		}
	}
}

func TestRead_CleansEveryTextTag_MP3(t *testing.T) {
	assertEveryTextTagClean(t, createTestMP3(t, t.TempDir(), dirtyTags()))
}

func TestRead_CleansEveryTextTag_FLAC(t *testing.T) {
	assertEveryTextTagClean(t, createTestFLAC(t, t.TempDir(), dirtyTags()))
}
