package tags

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
	"go.senan.xyz/taglib"
)

// Tests for MP3 tag writing edge cases

func TestWriteMP3_ID3v22Handling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mp3")

	// Create MP3 with ID3v2.2 header (which the id3v2 library doesn't support directly)
	// ID3v2.2 header: "ID3" + version (0x02 0x00) + flags + size
	id3v22Header := []byte{
		'I', 'D', '3', // Magic
		0x02, 0x00, // Version 2.0
		0x00,                   // Flags
		0x00, 0x00, 0x00, 0x0A, // Size (syncsafe: 10 bytes)
		// Minimal tag data (10 bytes padding)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	// MP3 frame after the ID3v2.2 tag
	mp3Frame := make([]byte, 417)
	mp3Frame[0] = 0xff
	mp3Frame[1] = 0xfb
	mp3Frame[2] = 0x90
	mp3Frame[3] = 0x00

	// Combine ID3v2.2 header with MP3 data
	data := make([]byte, 0, len(id3v22Header)+len(mp3Frame))
	data = append(data, id3v22Header...)
	data = append(data, mp3Frame...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// Write should strip ID3v2.2 and create ID3v2.4
	tags := &Tag{
		Title:  "Test Title",
		Artist: "Test Artist",
	}

	if err := Write(path, tags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify tags were written
	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if result.Title != tags.Title {
		t.Errorf("Title = %q, want %q", result.Title, tags.Title)
	}
}

func TestWriteMP3_ClearsExistingTags(t *testing.T) {
	dir := t.TempDir()
	path := createTestMP3(t, dir, &Tag{
		Title:       "Old Title",
		Artist:      "Old Artist",
		Album:       "Old Album",
		Genre:       "Old Genre",
		TrackNumber: 99,
		MBArtistID:  "old-uuid",
	})

	// Write new tags (without some fields)
	newTags := &Tag{
		Title:  "New Title",
		Artist: "New Artist",
	}

	if err := Write(path, newTags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read back and verify old tags were cleared
	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if result.Title != "New Title" {
		t.Errorf("Title = %q, want %q", result.Title, "New Title")
	}
	if result.Album != "" {
		t.Errorf("Album = %q, want empty (should be cleared)", result.Album)
	}
	if result.Genre != "" {
		t.Errorf("Genre = %q, want empty (should be cleared)", result.Genre)
	}
	if result.TrackNumber != 0 {
		t.Errorf("TrackNumber = %d, want 0 (should be cleared)", result.TrackNumber)
	}
	if result.MBArtistID != "" {
		t.Errorf("MBArtistID = %q, want empty (should be cleared)", result.MBArtistID)
	}
}

// Tests for FLAC tag writing edge cases

func TestWriteFLAC_ID3v2HeaderStripping(t *testing.T) {
	dir := t.TempDir()

	// Create a FLAC file
	path := createTestFLAC(t, dir, nil)

	// Read the FLAC data
	flacData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read FLAC: %v", err)
	}

	// Prepend an ID3v2 header (some tools incorrectly add these to FLAC)
	id3v2Header := []byte{
		'I', 'D', '3', // Magic
		0x04, 0x00, // Version 4.0
		0x00,                   // Flags
		0x00, 0x00, 0x00, 0x0A, // Size (syncsafe: 10 bytes)
		// Minimal tag data (10 bytes padding)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	// Write FLAC with ID3v2 prepended
	dataWithID3 := make([]byte, 0, len(id3v2Header)+len(flacData))
	dataWithID3 = append(dataWithID3, id3v2Header...)
	dataWithID3 = append(dataWithID3, flacData...)
	if err := os.WriteFile(path, dataWithID3, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Write tags - should strip ID3v2 header
	tags := &Tag{
		Title:  "Test Title",
		Artist: "Test Artist",
	}

	if err := Write(path, tags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify file no longer starts with ID3
	finalData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}

	if string(finalData[:3]) == id3Magic {
		t.Error("ID3v2 header was not stripped from FLAC file")
	}
	if string(finalData[:4]) != "fLaC" {
		t.Error("FLAC file should start with fLaC marker")
	}

	// Verify tags were written
	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if result.Title != tags.Title {
		t.Errorf("Title = %q, want %q", result.Title, tags.Title)
	}
}

func TestWriteFLAC_ReplacesExistingVorbisComments(t *testing.T) {
	dir := t.TempDir()
	path := createTestFLAC(t, dir, &Tag{
		Title:      "Old Title",
		Artist:     "Old Artist",
		MBArtistID: "old-uuid",
	})

	// Write new tags
	newTags := &Tag{
		Title:  "New Title",
		Artist: "New Artist",
	}

	if err := Write(path, newTags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read and verify old tags were replaced
	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if result.Title != "New Title" {
		t.Errorf("Title = %q, want %q", result.Title, "New Title")
	}
	if result.MBArtistID != "" {
		t.Errorf("MBArtistID = %q, want empty (old tag should be cleared)", result.MBArtistID)
	}
}

// Tests for Opus tag writing

func TestWriteOpus_FullTagSet(t *testing.T) {
	dir := t.TempDir()
	path := createTestOpus(t, dir, nil)

	tags := fullTestTags()
	if err := Write(path, tags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read back using taglib to verify all tags
	result, err := taglib.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags() error: %v", err)
	}

	assertTaglibTag(t, result, "TITLE", tags.Title)
	assertTaglibTag(t, result, "ARTIST", tags.Artist)
	assertTaglibTag(t, result, "ALBUM", tags.Album)
	assertTaglibTag(t, result, "ALBUMARTIST", tags.AlbumArtist)
	assertTaglibTag(t, result, "DATE", tags.Date)
	assertTaglibTag(t, result, "ORIGINALDATE", tags.OriginalDate)
	assertTaglibTag(t, result, "LABEL", tags.Label)
	assertTaglibTag(t, result, "MUSICBRAINZ_ARTISTID", tags.MBArtistID)
}

func TestWriteOpus_ClearsPreviousTags(t *testing.T) {
	dir := t.TempDir()
	path := createTestOpus(t, dir, &Tag{
		Title:      "Old Title",
		Artist:     "Old Artist",
		MBArtistID: "old-uuid",
	})

	// Write new tags (without MBArtistID)
	newTags := &Tag{
		Title:  "New Title",
		Artist: "New Artist",
	}

	if err := Write(path, newTags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read and verify old tags were cleared
	result, err := taglib.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags() error: %v", err)
	}

	assertTaglibTag(t, result, "TITLE", "New Title")
	if val := result["MUSICBRAINZ_ARTISTID"]; len(val) > 0 && val[0] != "" {
		t.Errorf("MBArtistID should be cleared, got %v", val)
	}
}

// Tests for safeInt16 function

func TestSafeInt16(t *testing.T) {
	tests := []struct {
		input int
		want  int16
	}{
		{0, 0},
		{1, 1},
		{32767, 32767},
		{32768, 32767}, // Overflow clamped
		{-1, -1},
		{-32768, -32768},
		{-32769, -32768}, // Underflow clamped
		{100000, 32767},  // Large overflow
	}

	for _, tt := range tests {
		got := safeInt16(tt.input)
		if got != tt.want {
			t.Errorf("safeInt16(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// Tests for parseVorbisComments

func TestParseVorbisComments(t *testing.T) {
	// Build a simple Vorbis comment block
	// Format: vendor_length (4) + vendor + comment_count (4) + comments
	vendor := []byte("test vendor")
	comment1 := []byte("ARTIST=Test Artist")
	comment2 := []byte("TITLE=Test Title")
	comment3 := []byte("DATE=2023")

	// Preallocate with estimated size
	dataLen := 4 + len(vendor) + 4 + 4 + len(comment1) + 4 + len(comment2) + 4 + len(comment3)
	data := make([]byte, 0, dataLen)
	// Vendor length (little-endian) + vendor
	data = append(data, byte(len(vendor)), 0, 0, 0)
	data = append(data, vendor...)
	// Comment count + first comment length
	data = append(data, 3, 0, 0, 0, byte(len(comment1)), 0, 0, 0)
	data = append(data, comment1...)
	data = append(data, byte(len(comment2)), 0, 0, 0)
	data = append(data, comment2...)
	data = append(data, byte(len(comment3)), 0, 0, 0)
	data = append(data, comment3...)

	result := parseVorbisComments(data)

	if result["ARTIST"] != "Test Artist" {
		t.Errorf("ARTIST = %q, want %q", result["ARTIST"], "Test Artist")
	}
	if result["TITLE"] != "Test Title" {
		t.Errorf("TITLE = %q, want %q", result["TITLE"], "Test Title")
	}
	if result["DATE"] != "2023" {
		t.Errorf("DATE = %q, want %q", result["DATE"], "2023")
	}
}

func TestParseVorbisComments_Empty(t *testing.T) {
	result := parseVorbisComments([]byte{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseVorbisComments_ShortData(t *testing.T) {
	result := parseVorbisComments([]byte{0, 1, 2})
	if len(result) != 0 {
		t.Errorf("expected empty map for short data, got %d entries", len(result))
	}
}

func TestParseVorbisComments_CaseNormalization(t *testing.T) {
	// Keys should be normalized to uppercase
	vendor := []byte("")
	comment := []byte("ArTiSt=Mixed Case Value")

	// Preallocate with estimated size
	dataLen := 4 + len(vendor) + 4 + 4 + len(comment)
	data := make([]byte, 0, dataLen)
	data = append(data, byte(len(vendor)), 0, 0, 0)
	data = append(data, vendor...)
	data = append(data, 1, 0, 0, 0, byte(len(comment)), 0, 0, 0)
	data = append(data, comment...)

	result := parseVorbisComments(data)

	// Key should be uppercase, value preserved
	if result["ARTIST"] != "Mixed Case Value" {
		t.Errorf("ARTIST = %q, want %q", result["ARTIST"], "Mixed Case Value")
	}
}

// Test for stripID3v2Tag

func TestStripID3v2Tag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mp3")

	// Create file with ID3v2 header + MP3 data
	id3v2Header := []byte{
		'I', 'D', '3', // Magic
		0x04, 0x00, // Version 4.0
		0x00,                   // Flags
		0x00, 0x00, 0x00, 0x0A, // Size (syncsafe: 10 bytes)
		// Tag data (10 bytes)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	mp3Data := []byte{0xFF, 0xFB, 0x90, 0x00} // MP3 frame header

	data := make([]byte, 0, len(id3v2Header)+len(mp3Data))
	data = append(data, id3v2Header...)
	data = append(data, mp3Data...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	if err := stripID3v2Tag(path); err != nil {
		t.Fatalf("stripID3v2Tag() error: %v", err)
	}

	// Verify ID3v2 was stripped
	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if string(result[:3]) == id3Magic {
		t.Error("ID3v2 header should have been stripped")
	}
	if result[0] != 0xFF || result[1] != 0xFB {
		t.Error("MP3 frame header should be at start of file")
	}
}

func TestStripID3v2Tag_NoTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mp3")

	// Create file without ID3v2 header
	mp3Data := []byte{0xFF, 0xFB, 0x90, 0x00}
	if err := os.WriteFile(path, mp3Data, 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// Should be a no-op
	if err := stripID3v2Tag(path); err != nil {
		t.Fatalf("stripID3v2Tag() error: %v", err)
	}

	// Verify file unchanged
	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if len(result) != 4 {
		t.Errorf("file length = %d, want 4", len(result))
	}
}

// Test for MP3 with UFID frame (MusicBrainz recording ID)

func TestWriteMP3_UFIDFrame(t *testing.T) {
	dir := t.TempDir()
	path := createTestMP3(t, dir, nil)

	tags := &Tag{
		Title:         "Test",
		MBRecordingID: "recording-uuid-1234",
	}

	if err := Write(path, tags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read back using id3v2 to check UFID frame directly
	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer id3tag.Close()

	frames := id3tag.GetFrames("UFID")
	if len(frames) == 0 {
		t.Fatal("expected UFID frame")
	}

	ufid, ok := frames[0].(id3v2.UFIDFrame)
	if !ok {
		t.Fatal("expected UFIDFrame type")
	}

	if ufid.OwnerIdentifier != "http://musicbrainz.org" {
		t.Errorf("OwnerIdentifier = %q, want %q", ufid.OwnerIdentifier, "http://musicbrainz.org")
	}
	if string(ufid.Identifier) != "recording-uuid-1234" {
		t.Errorf("Identifier = %q, want %q", string(ufid.Identifier), "recording-uuid-1234")
	}
}

// Helper function

func assertTaglibTag(t *testing.T, tags map[string][]string, key, want string) {
	t.Helper()
	got := tags[key]
	if len(got) == 0 || got[0] != want {
		t.Errorf("%s = %v, want [%s]", key, got, want)
	}
}
