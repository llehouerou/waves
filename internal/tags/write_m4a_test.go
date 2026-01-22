package tags

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Sorrow446/go-mp4tag"
	"go.senan.xyz/taglib"
)

// createTestM4A creates a minimal M4A file for testing.
func createTestM4A(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.m4a")

	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac", path)
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	return path
}

// TestWriteM4ATags_Basic tests basic tag writing.
func TestWriteM4ATags_Basic(t *testing.T) {
	path := createTestM4A(t)

	data := &Tag{
		Title:       "Test Track",
		Artist:      "Test Artist",
		Album:       "Test Album",
		AlbumArtist: "Test Album Artist",
		TrackNumber: 3,
		TotalTracks: 12,
		DiscNumber:  1,
		TotalDiscs:  2,
		Date:        "2023-06-15",
	}

	if err := writeM4ATags(path, data); err != nil {
		t.Fatalf("writeM4ATags: %v", err)
	}

	// Read back and verify
	result, err := taglib.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	assertTag(t, result, "TITLE", "Test Track")
	assertTag(t, result, "ARTIST", "Test Artist")
	assertTag(t, result, "ALBUM", "Test Album")
	assertTag(t, result, "ALBUMARTIST", "Test Album Artist")
	assertTag(t, result, "DATE", "2023-06-15")
}

// TestWriteM4ATags_OriginalDate tests ORIGINALDATE writing on fresh file.
func TestWriteM4ATags_OriginalDate(t *testing.T) {
	path := createTestM4A(t)

	data := &Tag{
		Title:        "Test Track",
		Artist:       "Test Artist",
		Date:         "2023-06-15",
		OriginalDate: "1999-12-31",
	}

	if err := writeM4ATags(path, data); err != nil {
		t.Fatalf("writeM4ATags: %v", err)
	}

	result, err := taglib.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	assertTag(t, result, "DATE", "2023-06-15")
	assertTag(t, result, "ORIGINALDATE", "1999-12-31")
	assertTag(t, result, "ORIGINALYEAR", "1999")
}

// TestWriteM4ATags_UpdateOriginalDate tests updating ORIGINALDATE on existing file.
func TestWriteM4ATags_UpdateOriginalDate(t *testing.T) {
	path := createTestM4A(t)

	// First write
	data1 := &Tag{
		Title:        "Test Track",
		Artist:       "Test Artist",
		Date:         "2020-01-01",
		OriginalDate: "1981-11-23",
	}
	if err := writeM4ATags(path, data1); err != nil {
		t.Fatalf("writeM4ATags (first): %v", err)
	}

	result1, _ := taglib.ReadTags(path)
	t.Logf("After first write: DATE=%v, ORIGINALDATE=%v", result1["DATE"], result1["ORIGINALDATE"])

	// Second write with different dates
	data2 := &Tag{
		Title:        "Test Track",
		Artist:       "Test Artist",
		Date:         "2099-06-15",
		OriginalDate: "2099-12-31",
	}
	if err := writeM4ATags(path, data2); err != nil {
		t.Fatalf("writeM4ATags (second): %v", err)
	}

	result2, err := taglib.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	t.Logf("After second write: DATE=%v, ORIGINALDATE=%v", result2["DATE"], result2["ORIGINALDATE"])

	assertTag(t, result2, "DATE", "2099-06-15")
	assertTag(t, result2, "ORIGINALDATE", "2099-12-31")
}

// TestWriteM4ATags_FullTagSet tests writing all tags used by waves.
func TestWriteM4ATags_FullTagSet(t *testing.T) {
	path := createTestM4A(t)

	data := &Tag{
		Title:            "Test Track",
		Artist:           "Test Artist",
		Album:            "Test Album",
		AlbumArtist:      "Test Album Artist",
		ArtistSortName:   "Artist, Test",
		TrackNumber:      3,
		TotalTracks:      12,
		DiscNumber:       1,
		TotalDiscs:       2,
		Date:             "2023-06-15",
		OriginalDate:     "1999-12-31",
		Genre:            "Rock",
		Label:            "Test Label",
		CatalogNumber:    "CAT-001",
		Barcode:          "1234567890123",
		Media:            "CD",
		ReleaseStatus:    "Official",
		ReleaseType:      "album",
		Script:           "Latn",
		Country:          "US",
		MBArtistID:       "artist-uuid",
		MBReleaseID:      "album-uuid",
		MBReleaseGroupID: "rg-uuid",
		MBTrackID:        "track-uuid",
		MBRecordingID:    "recording-uuid",
		ISRC:             "USRC12345678",
	}

	if err := writeM4ATags(path, data); err != nil {
		t.Fatalf("writeM4ATags: %v", err)
	}

	result, err := taglib.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	// Basic tags
	assertTag(t, result, "TITLE", "Test Track")
	assertTag(t, result, "ARTIST", "Test Artist")
	assertTag(t, result, "ALBUM", "Test Album")
	assertTag(t, result, "ALBUMARTIST", "Test Album Artist")
	assertTag(t, result, "ARTISTSORT", "Artist, Test")
	assertTag(t, result, "GENRE", "Rock")

	// Date tags
	assertTag(t, result, "DATE", "2023-06-15")
	assertTag(t, result, "ORIGINALDATE", "1999-12-31")
	assertTag(t, result, "ORIGINALYEAR", "1999")

	// Release info
	assertTag(t, result, "LABEL", "Test Label")
	assertTag(t, result, "CATALOGNUMBER", "CAT-001")
	assertTag(t, result, "BARCODE", "1234567890123")
	assertTag(t, result, "MEDIA", "CD")
	assertTag(t, result, "RELEASESTATUS", "Official")
	assertTag(t, result, "RELEASETYPE", "album")
	assertTag(t, result, "SCRIPT", "Latn")
	assertTag(t, result, "RELEASECOUNTRY", "US")

	// MusicBrainz IDs (go-mp4tag uses space-separated names)
	assertTag(t, result, "MUSICBRAINZ ARTIST ID", "artist-uuid")
	assertTag(t, result, "MUSICBRAINZ ALBUM ID", "album-uuid")
	assertTag(t, result, "MUSICBRAINZ RELEASE GROUP ID", "rg-uuid")
	assertTag(t, result, "MUSICBRAINZ RELEASE TRACK ID", "track-uuid")
	assertTag(t, result, "MUSICBRAINZ TRACK ID", "recording-uuid")
	assertTag(t, result, "ISRC", "USRC12345678")
}

// TestWriteM4ATags_UpdateLowercaseOriginalDate tests updating ORIGINALDATE on files
// that have lowercase atom names (created by older tools).
// This is the bug case - TagLib can't update lowercase freeform atoms.
func TestWriteM4ATags_UpdateLowercaseOriginalDate(t *testing.T) {
	path := createTestM4A(t)

	// Use go-mp4tag with UpperCustom(false) to write lowercase atoms (simulating older tools)
	mp4, err := mp4tag.Open(path)
	if err != nil {
		t.Fatalf("mp4tag.Open: %v", err)
	}
	mp4.UpperCustom(false) // Write lowercase atom names
	initialTags := &mp4tag.MP4Tags{
		Title:  "Test Track",
		Artist: "Test Artist",
		Date:   "2020-01-01",
		Custom: map[string]string{
			"originaldate": "1981-11-23", // lowercase
			"originalyear": "1981",       // lowercase
		},
	}
	if err := mp4.Write(initialTags, nil); err != nil {
		t.Fatalf("mp4tag.Write: %v", err)
	}
	mp4.Close()

	// Verify lowercase atoms were written
	result1, _ := taglib.ReadTags(path)
	t.Logf("After go-mp4tag (lowercase atoms): DATE=%v, ORIGINALDATE=%v", result1["DATE"], result1["ORIGINALDATE"])

	// Now try to update using our writeM4ATags
	data := &Tag{
		Title:        "Test Track",
		Artist:       "Test Artist",
		Date:         "2099-06-15",
		OriginalDate: "2099-12-31",
	}
	if err := writeM4ATags(path, data); err != nil {
		t.Fatalf("writeM4ATags: %v", err)
	}

	result2, err := taglib.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	t.Logf("After writeM4ATags: DATE=%v, ORIGINALDATE=%v", result2["DATE"], result2["ORIGINALDATE"])

	assertTag(t, result2, "DATE", "2099-06-15")
	assertTag(t, result2, "ORIGINALDATE", "2099-12-31") // This will fail with TagLib
}

func assertTag(t *testing.T, tags map[string][]string, key, want string) {
	t.Helper()
	got := tags[key]
	if len(got) == 0 || got[0] != want {
		t.Errorf("%s = %v, want [%s]", key, got, want)
	}
}
