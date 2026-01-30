package tags

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/bogem/id3v2/v2"
	"go.senan.xyz/taglib"
)

// Format constants for testing
const (
	formatAAC    = "AAC"
	formatALAC   = "ALAC"
	formatM4A    = "M4A"
	formatOPUS   = "OPUS"
	formatVORBIS = "VORBIS"
	formatFLAC   = "FLAC"
	formatMP3    = "MP3"
)

// isM4AFormat returns true if the format is a valid M4A audio format.
func isM4AFormat(format string) bool {
	return format == formatAAC || format == formatALAC || format == formatM4A
}

// Test file creation helpers

// createTestMP3 creates a minimal MP3 file with optional tags.
func createTestMP3(t *testing.T, dir string, tags *Tag) string {
	t.Helper()
	path := filepath.Join(dir, "test.mp3")

	// Create minimal MP3 frame (MPEG1 Layer3, 128kbps, 44100Hz, stereo)
	mp3Frame := make([]byte, 417)
	mp3Frame[0] = 0xff
	mp3Frame[1] = 0xfb
	mp3Frame[2] = 0x90
	mp3Frame[3] = 0x00

	if err := os.WriteFile(path, mp3Frame, 0o600); err != nil {
		t.Fatalf("failed to create test MP3: %v", err)
	}

	if tags != nil {
		if err := writeMP3Tags(path, tags); err != nil {
			t.Fatalf("failed to write MP3 tags: %v", err)
		}
	}

	return path
}

// createTestOpus creates a test Opus file using ffmpeg.
func createTestOpus(t *testing.T, dir string, tags *Tag) string {
	t.Helper()
	path := filepath.Join(dir, "test.opus")

	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "libopus", path)
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	if tags != nil {
		if err := writeOggTags(path, tags); err != nil {
			t.Fatalf("failed to write Opus tags: %v", err)
		}
	}

	return path
}

// createTestVorbis creates a test Vorbis (.ogg) file using ffmpeg.
func createTestVorbis(t *testing.T, dir string, tags *Tag) string {
	t.Helper()
	path := filepath.Join(dir, "test.ogg")

	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "libvorbis", path)
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	if tags != nil {
		// Vorbis uses the same Vorbis comments as Opus
		if err := writeOggTags(path, tags); err != nil {
			t.Fatalf("failed to write Vorbis tags: %v", err)
		}
	}

	return path
}

// createTestOGA creates a test OGA (Ogg Audio) file using ffmpeg with Vorbis codec.
func createTestOGA(t *testing.T, dir string, tags *Tag) string {
	t.Helper()
	path := filepath.Join(dir, "test.oga")

	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "libvorbis", path)
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	if tags != nil {
		// OGA uses Vorbis comments like .ogg
		if err := writeOggTags(path, tags); err != nil {
			t.Fatalf("failed to write OGA tags: %v", err)
		}
	}

	return path
}

// createTestFLAC creates a test FLAC file using ffmpeg.
func createTestFLAC(t *testing.T, dir string, tags *Tag) string {
	t.Helper()
	path := filepath.Join(dir, "test.flac")

	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "flac", path)
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	if tags != nil {
		if err := writeFLACTags(path, tags); err != nil {
			t.Fatalf("failed to write FLAC tags: %v", err)
		}
	}

	return path
}

// createTestM4AWithTags creates a test M4A file using ffmpeg with optional tags.
func createTestM4AWithTags(t *testing.T, dir string, tags *Tag) string {
	t.Helper()
	path := filepath.Join(dir, "test.m4a")

	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac", path)
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	if tags != nil {
		if err := writeM4ATags(path, tags); err != nil {
			t.Fatalf("failed to write M4A tags: %v", err)
		}
	}

	return path
}

// fullTestTags returns a Tag with all fields populated for testing.
func fullTestTags() *Tag {
	return &Tag{
		Title:            "Test Title",
		Artist:           "Test Artist",
		Album:            "Test Album",
		AlbumArtist:      "Test Album Artist",
		Genre:            "Rock",
		TrackNumber:      3,
		TotalTracks:      12,
		DiscNumber:       1,
		TotalDiscs:       2,
		Date:             "2023-06-15",
		OriginalDate:     "1999-12-31",
		ArtistSortName:   "Artist, Test",
		Label:            "Test Label",
		CatalogNumber:    "CAT-001",
		Barcode:          "1234567890123",
		Media:            "CD",
		ReleaseStatus:    "Official",
		ReleaseType:      "album",
		Script:           "Latn",
		Country:          "US",
		ISRC:             "USRC12345678",
		MBArtistID:       "artist-uuid-1234",
		MBReleaseID:      "release-uuid-1234",
		MBReleaseGroupID: "rg-uuid-1234",
		MBRecordingID:    "recording-uuid-1234",
		MBTrackID:        "track-uuid-1234",
	}
}

// Tests for Read() entry point

func TestRead_MP3(t *testing.T) {
	dir := t.TempDir()
	tags := fullTestTags()
	path := createTestMP3(t, dir, tags)

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify basic tags
	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)
	assertEqual(t, "Album", result.Album, tags.Album)
	assertEqual(t, "AlbumArtist", result.AlbumArtist, tags.AlbumArtist)
	assertEqual(t, "Genre", result.Genre, tags.Genre)
	assertEqual(t, "TrackNumber", result.TrackNumber, tags.TrackNumber)
	assertEqual(t, "TotalTracks", result.TotalTracks, tags.TotalTracks)
	assertEqual(t, "DiscNumber", result.DiscNumber, tags.DiscNumber)
	assertEqual(t, "TotalDiscs", result.TotalDiscs, tags.TotalDiscs)

	// Verify extended tags
	assertEqual(t, "Date", result.Date, tags.Date)
	assertEqual(t, "OriginalDate", result.OriginalDate, tags.OriginalDate)
	assertEqual(t, "ArtistSortName", result.ArtistSortName, tags.ArtistSortName)
	assertEqual(t, "Label", result.Label, tags.Label)
	assertEqual(t, "Media", result.Media, tags.Media)
	assertEqual(t, "ISRC", result.ISRC, tags.ISRC)

	// Verify MusicBrainz IDs
	assertEqual(t, "MBArtistID", result.MBArtistID, tags.MBArtistID)
	assertEqual(t, "MBReleaseID", result.MBReleaseID, tags.MBReleaseID)
	assertEqual(t, "MBReleaseGroupID", result.MBReleaseGroupID, tags.MBReleaseGroupID)
	assertEqual(t, "MBRecordingID", result.MBRecordingID, tags.MBRecordingID)
	assertEqual(t, "MBTrackID", result.MBTrackID, tags.MBTrackID)

	// Verify path is set
	assertEqual(t, "Path", result.Path, path)
}

func TestRead_FLAC(t *testing.T) {
	dir := t.TempDir()
	tags := fullTestTags()
	path := createTestFLAC(t, dir, tags)

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify basic tags
	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)
	assertEqual(t, "Album", result.Album, tags.Album)
	assertEqual(t, "AlbumArtist", result.AlbumArtist, tags.AlbumArtist)

	// Verify extended tags
	assertEqual(t, "Date", result.Date, tags.Date)
	assertEqual(t, "OriginalDate", result.OriginalDate, tags.OriginalDate)
	assertEqual(t, "MBArtistID", result.MBArtistID, tags.MBArtistID)
	assertEqual(t, "MBReleaseID", result.MBReleaseID, tags.MBReleaseID)
}

func TestRead_Opus(t *testing.T) {
	dir := t.TempDir()
	tags := fullTestTags()
	path := createTestOpus(t, dir, tags)

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify basic tags
	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)
	assertEqual(t, "Album", result.Album, tags.Album)
	assertEqual(t, "AlbumArtist", result.AlbumArtist, tags.AlbumArtist)

	// Verify extended tags
	assertEqual(t, "Date", result.Date, tags.Date)
	assertEqual(t, "OriginalDate", result.OriginalDate, tags.OriginalDate)
	assertEqual(t, "MBArtistID", result.MBArtistID, tags.MBArtistID)
}

func TestRead_Vorbis(t *testing.T) {
	dir := t.TempDir()
	tags := fullTestTags()
	path := createTestVorbis(t, dir, tags)

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify basic tags
	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)
	assertEqual(t, "Album", result.Album, tags.Album)
	assertEqual(t, "AlbumArtist", result.AlbumArtist, tags.AlbumArtist)

	// Verify extended tags (Vorbis uses the same Vorbis comments as Opus)
	assertEqual(t, "Date", result.Date, tags.Date)
	assertEqual(t, "OriginalDate", result.OriginalDate, tags.OriginalDate)
	assertEqual(t, "MBArtistID", result.MBArtistID, tags.MBArtistID)
}

func TestRead_OGA(t *testing.T) {
	dir := t.TempDir()
	tags := fullTestTags()
	path := createTestOGA(t, dir, tags)

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify basic tags
	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)
	assertEqual(t, "Album", result.Album, tags.Album)
	assertEqual(t, "AlbumArtist", result.AlbumArtist, tags.AlbumArtist)

	// Verify extended tags (OGA uses Vorbis comments)
	assertEqual(t, "Date", result.Date, tags.Date)
	assertEqual(t, "OriginalDate", result.OriginalDate, tags.OriginalDate)
	assertEqual(t, "MBArtistID", result.MBArtistID, tags.MBArtistID)
}

func TestRead_M4A(t *testing.T) {
	dir := t.TempDir()
	tags := fullTestTags()
	path := createTestM4AWithTags(t, dir, tags)

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify basic tags
	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)
	assertEqual(t, "Album", result.Album, tags.Album)
	assertEqual(t, "AlbumArtist", result.AlbumArtist, tags.AlbumArtist)

	// Verify extended tags
	assertEqual(t, "Date", result.Date, tags.Date)
	assertEqual(t, "OriginalDate", result.OriginalDate, tags.OriginalDate)
}

func TestRead_NonexistentFile(t *testing.T) {
	_, err := Read("/nonexistent/path/file.mp3")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestRead_TitleFallbackToFilename(t *testing.T) {
	dir := t.TempDir()
	// Create MP3 without title tag
	tags := &Tag{
		Artist: "Test Artist",
		Album:  "Test Album",
	}
	path := createTestMP3(t, dir, tags)

	// Rename to a specific filename
	newPath := filepath.Join(dir, "My Song.mp3")
	if err := os.Rename(path, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	result, err := Read(newPath)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Title should fall back to filename
	if result.Title != "My Song.mp3" {
		t.Errorf("Title = %q, want %q", result.Title, "My Song.mp3")
	}
}

func TestRead_AlbumArtistFallbackToArtist(t *testing.T) {
	dir := t.TempDir()
	// Create MP3 with artist but no album artist
	tags := &Tag{
		Title:  "Test",
		Artist: "Solo Artist",
		Album:  "Test Album",
	}
	path := createTestMP3(t, dir, tags)

	// Clear album artist by re-reading raw tags
	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	id3tag.DeleteFrames("TPE2") // Remove album artist frame
	if err := id3tag.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	id3tag.Close()

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Album artist should fall back to artist
	if result.AlbumArtist != "Solo Artist" {
		t.Errorf("AlbumArtist = %q, want %q", result.AlbumArtist, "Solo Artist")
	}
}

// Tests for ReadWithAudio()

func TestReadWithAudio_MP3(t *testing.T) {
	dir := t.TempDir()
	tags := &Tag{Title: "Test", Artist: "Test Artist"}
	path := createTestMP3(t, dir, tags)

	result, err := ReadWithAudio(path)
	if err != nil {
		t.Fatalf("ReadWithAudio() error: %v", err)
	}

	// Verify tags were read
	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)

	// Verify audio info
	if result.Format != "MP3" {
		t.Errorf("Format = %q, want %q", result.Format, "MP3")
	}
	if result.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want %d", result.SampleRate, 44100)
	}
}

func TestReadWithAudio_FLAC(t *testing.T) {
	dir := t.TempDir()
	tags := &Tag{Title: "Test", Artist: "Test Artist"}
	path := createTestFLAC(t, dir, tags)

	result, err := ReadWithAudio(path)
	if err != nil {
		t.Fatalf("ReadWithAudio() error: %v", err)
	}

	if result.Format != "FLAC" {
		t.Errorf("Format = %q, want %q", result.Format, "FLAC")
	}
	// FLAC should have duration > 0
	if result.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", result.Duration)
	}
}

func TestReadWithAudio_Opus(t *testing.T) {
	dir := t.TempDir()
	tags := &Tag{Title: "Test", Artist: "Test Artist"}
	path := createTestOpus(t, dir, tags)

	result, err := ReadWithAudio(path)
	if err != nil {
		t.Fatalf("ReadWithAudio() error: %v", err)
	}

	if result.Format != formatOPUS {
		t.Errorf("Format = %q, want %q", result.Format, formatOPUS)
	}
	if result.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want %d (Opus always decodes to 48kHz)", result.SampleRate, 48000)
	}
}

func TestReadWithAudio_Vorbis(t *testing.T) {
	dir := t.TempDir()
	tags := &Tag{Title: "Test", Artist: "Test Artist"}
	path := createTestVorbis(t, dir, tags)

	result, err := ReadWithAudio(path)
	if err != nil {
		t.Fatalf("ReadWithAudio() error: %v", err)
	}

	// Vorbis files should be detected correctly with actual sample rate
	if result.Format != formatVORBIS {
		t.Errorf("Format = %q, want %q", result.Format, formatVORBIS)
	}
	// ffmpeg sine filter defaults to 44100 Hz
	if result.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want %d", result.SampleRate, 44100)
	}
	if result.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", result.Duration)
	}
}

func TestReadWithAudio_OGA(t *testing.T) {
	dir := t.TempDir()
	tags := &Tag{Title: "Test", Artist: "Test Artist"}
	path := createTestOGA(t, dir, tags)

	result, err := ReadWithAudio(path)
	if err != nil {
		t.Fatalf("ReadWithAudio() error: %v", err)
	}

	// OGA files with Vorbis codec should be detected correctly
	if result.Format != formatVORBIS {
		t.Errorf("Format = %q, want %q", result.Format, formatVORBIS)
	}
	// ffmpeg sine filter defaults to 44100 Hz
	if result.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want %d", result.SampleRate, 44100)
	}
	if result.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", result.Duration)
	}
}

func TestReadWithAudio_M4A(t *testing.T) {
	dir := t.TempDir()
	tags := &Tag{Title: "Test", Artist: "Test Artist"}
	path := createTestM4AWithTags(t, dir, tags)

	result, err := ReadWithAudio(path)
	if err != nil {
		t.Fatalf("ReadWithAudio() error: %v", err)
	}

	if !isM4AFormat(result.Format) {
		t.Errorf("Format = %q, want %s/%s/%s", result.Format, formatAAC, formatALAC, formatM4A)
	}
	if result.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", result.Duration)
	}
}

// Tests for Write() entry point

func TestWrite_MP3_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := createTestMP3(t, dir, nil)

	original := fullTestTags()
	if err := Write(path, original); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	verifyTagsMatch(t, result, original)
}

func TestWrite_FLAC_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := createTestFLAC(t, dir, nil)

	original := fullTestTags()
	if err := Write(path, original); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	verifyTagsMatch(t, result, original)
}

func TestWrite_Opus_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := createTestOpus(t, dir, nil)

	original := fullTestTags()
	if err := Write(path, original); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	verifyTagsMatch(t, result, original)
}

func TestWrite_Vorbis_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := createTestVorbis(t, dir, nil)

	original := fullTestTags()
	if err := Write(path, original); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	verifyTagsMatch(t, result, original)
}

func TestWrite_OGA_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := createTestOGA(t, dir, nil)

	original := fullTestTags()
	if err := Write(path, original); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	verifyTagsMatch(t, result, original)
}

func TestWrite_M4A_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := createTestM4AWithTags(t, dir, nil)

	original := fullTestTags()
	if err := Write(path, original); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	verifyTagsMatch(t, result, original)
}

func TestWrite_NonexistentFile(t *testing.T) {
	err := Write("/nonexistent/path/file.mp3", &Tag{})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestWrite_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")
	if err := os.WriteFile(path, []byte("RIFF"), 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	err := Write(path, &Tag{})
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

// Tests for Unicode support

func TestRead_Unicode(t *testing.T) {
	dir := t.TempDir()
	tags := &Tag{
		Title:       "日本語タイトル",
		Artist:      "アーティスト名",
		Album:       "Альбом на русском",
		AlbumArtist: "Künstler mit Umlauten",
	}
	path := createTestMP3(t, dir, tags)

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	assertEqual(t, "Title", result.Title, tags.Title)
	assertEqual(t, "Artist", result.Artist, tags.Artist)
	assertEqual(t, "Album", result.Album, tags.Album)
	assertEqual(t, "AlbumArtist", result.AlbumArtist, tags.AlbumArtist)
}

// Tests for yearToDate helper

func TestYearToDate(t *testing.T) {
	tests := []struct {
		year int
		want string
	}{
		{0, ""},
		{2023, "2023"},
		{1999, "1999"},
		{1, "1"},
	}

	for _, tt := range tests {
		got := yearToDate(tt.year)
		if got != tt.want {
			t.Errorf("yearToDate(%d) = %q, want %q", tt.year, got, tt.want)
		}
	}
}

// Tests for detectMimeType

func TestDetectMimeType(t *testing.T) {
	// JPEG header
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	if got := detectMimeType(jpegData); got != mimeJPEG {
		t.Errorf("detectMimeType(JPEG) = %q, want %q", got, mimeJPEG)
	}

	// PNG header
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if got := detectMimeType(pngData); got != mimePNG {
		t.Errorf("detectMimeType(PNG) = %q, want %q", got, mimePNG)
	}

	// Unknown data defaults to JPEG
	unknownData := []byte{0x00, 0x01, 0x02, 0x03}
	if got := detectMimeType(unknownData); got != mimeJPEG {
		t.Errorf("detectMimeType(unknown) = %q, want %q", got, mimeJPEG)
	}

	// Empty data defaults to JPEG
	if got := detectMimeType(nil); got != mimeJPEG {
		t.Errorf("detectMimeType(nil) = %q, want %q", got, mimeJPEG)
	}
}

// Tests for ReadAudioInfo

func TestReadAudioInfo_MP3(t *testing.T) {
	dir := t.TempDir()
	path := createTestMP3(t, dir, nil)

	info, err := ReadAudioInfo(path)
	if err != nil {
		t.Fatalf("ReadAudioInfo() error: %v", err)
	}

	if info.Format != "MP3" {
		t.Errorf("Format = %q, want %q", info.Format, "MP3")
	}
	if info.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want %d", info.SampleRate, 44100)
	}
	if info.BitDepth != 16 {
		t.Errorf("BitDepth = %d, want %d", info.BitDepth, 16)
	}
}

func TestReadAudioInfo_FLAC(t *testing.T) {
	dir := t.TempDir()
	path := createTestFLAC(t, dir, nil)

	info, err := ReadAudioInfo(path)
	if err != nil {
		t.Fatalf("ReadAudioInfo() error: %v", err)
	}

	if info.Format != "FLAC" {
		t.Errorf("Format = %q, want %q", info.Format, "FLAC")
	}
	if info.SampleRate <= 0 {
		t.Errorf("SampleRate = %d, want > 0", info.SampleRate)
	}
	if info.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", info.Duration)
	}
}

func TestReadAudioInfo_Opus(t *testing.T) {
	dir := t.TempDir()
	path := createTestOpus(t, dir, nil)

	info, err := ReadAudioInfo(path)
	if err != nil {
		t.Fatalf("ReadAudioInfo() error: %v", err)
	}

	if info.Format != formatOPUS {
		t.Errorf("Format = %q, want %q", info.Format, formatOPUS)
	}
	// Opus always reports 48kHz
	if info.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want %d", info.SampleRate, 48000)
	}
}

func TestReadAudioInfo_Vorbis(t *testing.T) {
	dir := t.TempDir()
	path := createTestVorbis(t, dir, nil)

	info, err := ReadAudioInfo(path)
	if err != nil {
		t.Fatalf("ReadAudioInfo() error: %v", err)
	}

	// Vorbis files should be detected correctly with actual sample rate
	if info.Format != formatVORBIS {
		t.Errorf("Format = %q, want %q", info.Format, formatVORBIS)
	}
	// ffmpeg sine filter defaults to 44100 Hz
	if info.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want %d", info.SampleRate, 44100)
	}
	if info.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", info.Duration)
	}
}

func TestReadAudioInfo_OGA(t *testing.T) {
	dir := t.TempDir()
	path := createTestOGA(t, dir, nil)

	info, err := ReadAudioInfo(path)
	if err != nil {
		t.Fatalf("ReadAudioInfo() error: %v", err)
	}

	// OGA files with Vorbis codec should be detected correctly
	if info.Format != formatVORBIS {
		t.Errorf("Format = %q, want %q", info.Format, formatVORBIS)
	}
	// ffmpeg sine filter defaults to 44100 Hz
	if info.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want %d", info.SampleRate, 44100)
	}
	if info.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", info.Duration)
	}
}

func TestReadAudioInfo_M4A(t *testing.T) {
	dir := t.TempDir()
	path := createTestM4AWithTags(t, dir, nil)

	info, err := ReadAudioInfo(path)
	if err != nil {
		t.Fatalf("ReadAudioInfo() error: %v", err)
	}

	// M4A with AAC codec
	if !isM4AFormat(info.Format) {
		t.Errorf("Format = %q, want %s/%s/%s", info.Format, formatAAC, formatALAC, formatM4A)
	}
	if info.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", info.Duration)
	}
}

func TestReadAudioInfo_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")
	if err := os.WriteFile(path, []byte("RIFF"), 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	_, err := ReadAudioInfo(path)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

// Test helpers

func assertEqual[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

func verifyTagsMatch(t *testing.T, got, want *Tag) {
	t.Helper()

	// Basic tags
	assertEqual(t, "Title", got.Title, want.Title)
	assertEqual(t, "Artist", got.Artist, want.Artist)
	assertEqual(t, "Album", got.Album, want.Album)
	assertEqual(t, "AlbumArtist", got.AlbumArtist, want.AlbumArtist)
	assertEqual(t, "Genre", got.Genre, want.Genre)
	assertEqual(t, "TrackNumber", got.TrackNumber, want.TrackNumber)
	assertEqual(t, "TotalTracks", got.TotalTracks, want.TotalTracks)
	assertEqual(t, "DiscNumber", got.DiscNumber, want.DiscNumber)
	assertEqual(t, "TotalDiscs", got.TotalDiscs, want.TotalDiscs)

	// Date tags
	assertEqual(t, "Date", got.Date, want.Date)
	assertEqual(t, "OriginalDate", got.OriginalDate, want.OriginalDate)

	// Extended tags
	assertEqual(t, "ArtistSortName", got.ArtistSortName, want.ArtistSortName)
	assertEqual(t, "Label", got.Label, want.Label)
	assertEqual(t, "CatalogNumber", got.CatalogNumber, want.CatalogNumber)
	assertEqual(t, "Barcode", got.Barcode, want.Barcode)
	assertEqual(t, "Media", got.Media, want.Media)
	assertEqual(t, "ReleaseStatus", got.ReleaseStatus, want.ReleaseStatus)
	assertEqual(t, "ReleaseType", got.ReleaseType, want.ReleaseType)
	assertEqual(t, "Script", got.Script, want.Script)
	assertEqual(t, "Country", got.Country, want.Country)
	assertEqual(t, "ISRC", got.ISRC, want.ISRC)

	// MusicBrainz IDs
	assertEqual(t, "MBArtistID", got.MBArtistID, want.MBArtistID)
	assertEqual(t, "MBReleaseID", got.MBReleaseID, want.MBReleaseID)
	assertEqual(t, "MBReleaseGroupID", got.MBReleaseGroupID, want.MBReleaseGroupID)
	assertEqual(t, "MBRecordingID", got.MBRecordingID, want.MBRecordingID)
	assertEqual(t, "MBTrackID", got.MBTrackID, want.MBTrackID)
}

// Tests for MP3 ID3v2.3 date format parsing

func TestRead_MP3_ID3v23DateFormat(t *testing.T) {
	dir := t.TempDir()
	path := createTestMP3(t, dir, nil)

	// Write ID3v2.3 format date tags (TYER + TDAT)
	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	id3tag.SetTitle("Test")
	id3tag.AddTextFrame("TYER", id3v2.EncodingUTF8, "2023")
	id3tag.AddTextFrame("TDAT", id3v2.EncodingUTF8, "1506") // DDMM format: 15th June
	id3tag.AddTextFrame("TORY", id3v2.EncodingUTF8, "1999") // Original year (ID3v2.3)

	if err := id3tag.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	id3tag.Close()

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Date should be parsed as YYYY-MM-DD from TYER + TDAT
	if result.Date != "2023-06-15" {
		t.Errorf("Date = %q, want %q", result.Date, "2023-06-15")
	}

	// Original date should come from TORY
	if result.OriginalDate != "1999" {
		t.Errorf("OriginalDate = %q, want %q", result.OriginalDate, "1999")
	}
}

// Tests for FLAC with Vorbis comments

func TestRead_FLAC_AllExtendedTags(t *testing.T) {
	dir := t.TempDir()
	path := createTestFLAC(t, dir, nil)

	// Write all extended tags
	tags := map[string][]string{
		"TITLE":                      {"Test Title"},
		"ARTIST":                     {"Test Artist"},
		"ALBUM":                      {"Test Album"},
		"ALBUMARTIST":                {"Test Album Artist"},
		"DATE":                       {"2023-06-15"},
		"ORIGINALDATE":               {"1999-12-31"},
		"ARTISTSORT":                 {"Artist, Test"},
		"LABEL":                      {"Test Label"},
		"CATALOGNUMBER":              {"CAT-001"},
		"BARCODE":                    {"1234567890123"},
		"MEDIA":                      {"CD"},
		"RELEASESTATUS":              {"Official"},
		"RELEASETYPE":                {"album"},
		"SCRIPT":                     {"Latn"},
		"RELEASECOUNTRY":             {"US"},
		"ISRC":                       {"USRC12345678"},
		"MUSICBRAINZ_ARTISTID":       {"artist-uuid"},
		"MUSICBRAINZ_ALBUMID":        {"album-uuid"},
		"MUSICBRAINZ_RELEASEGROUPID": {"rg-uuid"},
		"MUSICBRAINZ_TRACKID":        {"recording-uuid"},
		"MUSICBRAINZ_RELEASETRACKID": {"track-uuid"},
	}

	if err := taglib.WriteTags(path, tags, taglib.Clear); err != nil {
		t.Fatalf("WriteTags: %v", err)
	}

	result, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	assertEqual(t, "Date", result.Date, "2023-06-15")
	assertEqual(t, "OriginalDate", result.OriginalDate, "1999-12-31")
	assertEqual(t, "ArtistSortName", result.ArtistSortName, "Artist, Test")
	assertEqual(t, "Label", result.Label, "Test Label")
	assertEqual(t, "CatalogNumber", result.CatalogNumber, "CAT-001")
	assertEqual(t, "Barcode", result.Barcode, "1234567890123")
	assertEqual(t, "Media", result.Media, "CD")
	assertEqual(t, "ReleaseStatus", result.ReleaseStatus, "Official")
	assertEqual(t, "ReleaseType", result.ReleaseType, "album")
	assertEqual(t, "Script", result.Script, "Latn")
	assertEqual(t, "Country", result.Country, "US")
	assertEqual(t, "ISRC", result.ISRC, "USRC12345678")
	assertEqual(t, "MBArtistID", result.MBArtistID, "artist-uuid")
	assertEqual(t, "MBReleaseID", result.MBReleaseID, "album-uuid")
	assertEqual(t, "MBReleaseGroupID", result.MBReleaseGroupID, "rg-uuid")
	assertEqual(t, "MBRecordingID", result.MBRecordingID, "recording-uuid")
	assertEqual(t, "MBTrackID", result.MBTrackID, "track-uuid")
}

// Test for Write with cover art

func TestWrite_MP3_WithCoverArt(t *testing.T) {
	dir := t.TempDir()
	path := createTestMP3(t, dir, nil)

	// Create a minimal JPEG (just header bytes for testing)
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}

	tags := &Tag{
		Title:    "Test",
		Artist:   "Test Artist",
		CoverArt: jpegData,
	}

	if err := Write(path, tags); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify cover art was written by reading it back
	data, mimeType, err := ExtractCoverArt(path)
	if err != nil {
		t.Fatalf("ExtractCoverArt() error: %v", err)
	}

	if data == nil {
		t.Error("expected cover art data, got nil")
	}
	if mimeType != mimeJPEG {
		t.Errorf("mimeType = %q, want %q", mimeType, mimeJPEG)
	}
}

// Test for duration parsing

func TestReadAudioInfo_Duration(t *testing.T) {
	dir := t.TempDir()

	// Create 1 second audio files
	opusPath := createTestOpus(t, dir, nil)
	flacPath := createTestFLAC(t, dir, nil)
	m4aPath := createTestM4AWithTags(t, dir, nil)
	vorbisPath := createTestVorbis(t, dir, nil)
	ogaPath := createTestOGA(t, dir, nil)

	tests := []struct {
		name string
		path string
	}{
		{"Opus", opusPath},
		{"FLAC", flacPath},
		{"M4A", m4aPath},
		{"Vorbis", vorbisPath},
		{"OGA", ogaPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ReadAudioInfo(tt.path)
			if err != nil {
				t.Fatalf("ReadAudioInfo() error: %v", err)
			}

			// Duration should be approximately 1 second (test files are 1s)
			if info.Duration < 900*time.Millisecond || info.Duration > 1100*time.Millisecond {
				t.Errorf("Duration = %v, want approximately 1s", info.Duration)
			}
		})
	}
}
