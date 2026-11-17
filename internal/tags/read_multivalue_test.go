package tags

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bogem/id3v2/v2"
	"go.senan.xyz/taglib"
)

// multiValues lists the values written for each tag of a multi-valued file,
// by the property names taglib uses for Vorbis comments and MP4 atoms. The
// file has no album artist.
var multiValues = map[string][]string{
	taglib.Title:               {"T1", "T2"},
	taglib.Artist:              {"A1", "A2"},
	taglib.Album:               {"Al1", "Al2"},
	taglib.Genre:               {"Rock", "Pop"},
	taglib.ReleaseType:         {"album", "live"},
	taglib.Label:               {"L1", "L2"},
	taglib.CatalogNumber:       {"C1", "C2"},
	taglib.ISRC:                {"I1", "I2"},
	taglib.MusicBrainzArtistID: {"id1", "id2"},
	taglib.Media:               {"X", "", "X"},
	taglib.Script:              {"B", "A", "B"},
	taglib.ReleaseCountry:      {"x", "X"},
	taglib.Barcode:             {"B1; B2"},
}

// multiValueFrames gives, for an MP3, the frame each tag of multiValues is
// written to: a text frame ID, or the description of a TXXX frame.
var multiValueFrames = map[string]string{
	taglib.Title:               "TIT2",
	taglib.Artist:              "TPE1",
	taglib.Album:               "TALB",
	taglib.Genre:               "TCON",
	taglib.ReleaseType:         "TXXX:MusicBrainz Album Type",
	taglib.Label:               "TPUB",
	taglib.CatalogNumber:       "TXXX:CATALOGNUMBER",
	taglib.ISRC:                "TSRC",
	taglib.MusicBrainzArtistID: "TXXX:MusicBrainz Artist Id",
	taglib.Media:               "TMED",
	taglib.Script:              "TXXX:SCRIPT",
	taglib.ReleaseCountry:      "TXXX:MusicBrainz Album Release Country",
	taglib.Barcode:             "TXXX:BARCODE",
}

// assertMultiValuesRead checks the fields read from a file holding multiValues.
func assertMultiValuesRead(t *testing.T, got *Tag) {
	t.Helper()
	assertEqual(t, "Title", got.Title, "T1; T2")
	assertEqual(t, "Artist", got.Artist, "A1; A2")
	assertEqual(t, "AlbumArtist", got.AlbumArtist, "A1; A2")
	assertEqual(t, "Album", got.Album, "Al1; Al2")
	assertEqual(t, "Genre", got.Genre, "Rock; Pop")
	assertEqual(t, "ReleaseType", got.ReleaseType, "album; live")
	assertEqual(t, "Label", got.Label, "L1; L2")
	assertEqual(t, "CatalogNumber", got.CatalogNumber, "C1; C2")
	assertEqual(t, "ISRC", got.ISRC, "I1; I2")
	assertEqual(t, "MBArtistID", got.MBArtistID, "id1; id2")
	assertEqual(t, "Media", got.Media, "X")
	assertEqual(t, "Script", got.Script, "B; A")
	assertEqual(t, "Country", got.Country, "x; X")
	assertEqual(t, "Barcode", got.Barcode, "B1; B2")
}

// writeTaglibMultiValues writes multiValues to a FLAC, Ogg or M4A file.
func writeTaglibMultiValues(t *testing.T, path string) {
	t.Helper()
	if err := taglib.WriteTags(path, multiValues, taglib.Clear); err != nil {
		t.Fatalf("WriteTags: %v", err)
	}
}

// createMultiValueMP3 writes an MP3 whose frames hold multiValues, separated
// by a NUL, in an ID3v2 tag of the given version.
func createMultiValueMP3(t *testing.T, version byte, encoding id3v2.Encoding) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.mp3")
	createMinimalMP3(t, path)

	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer id3tag.Close()
	id3tag.SetVersion(version)
	for key, values := range multiValues {
		text := strings.Join(values, "\x00")
		frame := multiValueFrames[key]
		if description, ok := strings.CutPrefix(frame, "TXXX:"); ok {
			id3tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
				Encoding:    encoding,
				Description: description,
				Value:       text,
			})
			continue
		}
		id3tag.AddTextFrame(frame, encoding, text)
	}
	if err := id3tag.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	return path
}

// assertNoNUL checks that no text field of a tag holds a NUL byte.
func assertNoNUL(t *testing.T, got *Tag) {
	t.Helper()
	fields := map[string]string{
		"Title": got.Title, "Artist": got.Artist, "AlbumArtist": got.AlbumArtist,
		"Album": got.Album, "Genre": got.Genre, "Date": got.Date,
		"OriginalDate": got.OriginalDate, "ArtistSortName": got.ArtistSortName,
		"Label": got.Label, "CatalogNumber": got.CatalogNumber, "Barcode": got.Barcode,
		"Media": got.Media, "ReleaseStatus": got.ReleaseStatus,
		"ReleaseType": got.ReleaseType, "Script": got.Script, "Country": got.Country,
		"ISRC": got.ISRC, "MBArtistID": got.MBArtistID, "MBReleaseID": got.MBReleaseID,
		"MBReleaseGroupID": got.MBReleaseGroupID, "MBRecordingID": got.MBRecordingID,
		"MBTrackID": got.MBTrackID,
	}
	for name, value := range fields {
		if strings.Contains(value, "\x00") {
			t.Errorf("%s = %q holds a NUL", name, value)
		}
	}
}

// readBoth reads a file with Read and with the given fallback reader, checks
// that the fallback reader returns the same fields, and returns what Read read.
func readBoth(t *testing.T, path string, fallback func(string) (*Tag, error)) *Tag {
	t.Helper()
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	fromFallback, err := fallback(path)
	if err != nil {
		t.Fatalf("fallback reader error: %v", err)
	}
	verifyTagsMatch(t, fromFallback, got)
	return got
}

func TestRead_MultipleValues_FLAC(t *testing.T) {
	path := createTestFLAC(t, t.TempDir(), nil)
	writeTaglibMultiValues(t, path)

	assertMultiValuesRead(t, readBoth(t, path, readFLACWithTaglib))
}

func TestRead_MultipleValues_Vorbis(t *testing.T) {
	path := createTestVorbis(t, t.TempDir(), nil)
	writeTaglibMultiValues(t, path)

	assertMultiValuesRead(t, readBoth(t, path, readOggWithTaglib))
}

func TestRead_MultipleValues_Opus(t *testing.T) {
	path := createTestOpus(t, t.TempDir(), nil)
	writeTaglibMultiValues(t, path)

	assertMultiValuesRead(t, readBoth(t, path, readOggWithTaglib))
}

func TestRead_MultipleValues_M4A(t *testing.T) {
	path := createTestM4AWithTags(t, t.TempDir(), nil)
	writeTaglibMultiValues(t, path)

	assertMultiValuesRead(t, readBoth(t, path, readM4AWithTaglib))
}

func TestRead_MultipleValues_MP3(t *testing.T) {
	tests := []struct {
		name     string
		version  byte
		encoding id3v2.Encoding
	}{
		{"ID3v2.4", 4, id3v2.EncodingUTF8},
		{"ID3v2.3", 3, id3v2.EncodingISO},
		{"ID3v2.3 UTF-16", 3, id3v2.EncodingUTF16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createMultiValueMP3(t, tt.version, tt.encoding)

			got := readBoth(t, path, readMP3WithID3v2Fallback)
			assertMultiValuesRead(t, got)
			assertNoNUL(t, got)
		})
	}
}
