package tags

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-flac/go-flac"
)

// Tests of a tag write that replaces the file of the track: a FLAC, a FLAC
// that starts with an ID3v2 header, an MP3 whose tag is ID3v2.2, an M4A and
// an Ogg file.

// wholeWriteFixture makes, in dir, a track whose tag write replaces its file.
type wholeWriteFixture struct {
	name string
	make func(t *testing.T, dir string) string
}

var wholeWriteFixtures = []wholeWriteFixture{
	{"FLAC", func(t *testing.T, dir string) string { return createTestFLAC(t, dir, nil) }},
	{"FLAC with an ID3v2 header", createTestFLACWithID3v2Header},
	{"MP3 with an ID3v2.2 tag", createTestMP3WithID3v22Tag},
	{"M4A", func(t *testing.T, dir string) string { return createTestM4AWithTags(t, dir, nil) }},
	{"Opus", func(t *testing.T, dir string) string { return createTestOpus(t, dir, nil) }},
	{"Vorbis", func(t *testing.T, dir string) string { return createTestVorbis(t, dir, nil) }},
	{"FLAC in Ogg", createTestOggFLAC},
}

// newTags is what the tests write on a track.
func newTags() *Tag {
	return &Tag{Title: "New Title", Artist: "New Artist"}
}

// testCover returns a cover of size bytes: a JPEG header, then bytes that do
// not compress.
func testCover(size int) []byte {
	cover := make([]byte, size)
	copy(cover, []byte{0xff, 0xd8, 0xff, 0xe0})
	state := uint32(1)
	for i := 4; i < size; i++ {
		state = state*1664525 + 1013904223
		cover[i] = byte(state >> 24)
	}
	return cover
}

// createTestFLACWithID3v2Header creates a FLAC that starts with an ID3v2
// header, as some tools write them.
func createTestFLACWithID3v2Header(t *testing.T, dir string) string {
	t.Helper()
	path := createTestFLAC(t, dir, nil)
	flacData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read FLAC: %v", err)
	}
	data := make([]byte, 0, 20+len(flacData))
	data = append(data,
		'I', 'D', '3', 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0A,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	)
	data = append(data, flacData...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write FLAC: %v", err)
	}
	return path
}

// createTestMP3WithID3v22Tag creates an MP3 of a hundred frames, each with its
// own content, behind an ID3v2.2 tag.
func createTestMP3WithID3v22Tag(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.mp3")
	data := []byte{
		'I', 'D', '3', 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0A,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	for i := range 100 {
		// MPEG1 Layer 3, 128 kbps, 44100 Hz, stereo.
		frame := bytes.Repeat([]byte{byte(i)}, 417)
		frame[0], frame[1], frame[2], frame[3] = 0xff, 0xfb, 0x90, 0x00
		data = append(data, frame...)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write MP3: %v", err)
	}
	return path
}

// trackAudio returns the audio of the track at path: what follows its ID3v2
// tag, and for a FLAC its frames, past the metadata; the packets of an Ogg
// file but its comment header; the audio packets of an M4A, as ffmpeg reads
// them.
func trackAudio(t *testing.T, path string) []byte {
	t.Helper()
	if isOggPath(path) {
		return oggAudio(t, path)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ExtM4A, ExtMP4:
		out, err := exec.Command("ffmpeg", "-loglevel", "error", "-i", path, //nolint:gosec // a test file
			"-map", "0:a", "-c", "copy", "-f", "data", "-").Output()
		if err != nil {
			t.Fatalf("ffmpeg: read the audio of %s: %v", path, err)
		}
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) >= 10 && string(data[:3]) == id3Magic {
		size := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9]) + 10
		if data[5]&0x10 != 0 {
			size += 10
		}
		data = data[size:]
	}
	if !strings.EqualFold(filepath.Ext(path), ExtFLAC) {
		return data
	}
	f, err := flac.ParseBytes(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse FLAC %s: %v", path, err)
	}
	return f.Frames
}

// assertOnlyFile fails unless the folder dir holds the file name and nothing else.
func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != name {
		t.Errorf("folder holds %q, want only %q", names, name)
	}
}

// assertNewTags fails unless the track at path has the tags of newTags and
// the audio given. The tags of an Ogg file are looked for in its comment
// header: Read cannot read a FLAC in Ogg.
func assertNewTags(t *testing.T, path string, audio []byte) {
	t.Helper()
	if !bytes.Equal(trackAudio(t, path), audio) {
		t.Error("the audio of the track changed")
	}
	if isOggPath(path) {
		packets := testOggPackets(t, path)
		if len(packets) < 2 {
			t.Fatalf("%s has %d packets, no comment header", path, len(packets))
		}
		for _, field := range []string{"TITLE=New Title", "ARTIST=New Artist"} {
			if !bytes.Contains(packets[1], []byte(field)) {
				t.Errorf("the comment header holds no %q", field)
			}
		}
		return
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got.Title != "New Title" || got.Artist != "New Artist" {
		t.Errorf("tags = %q by %q, want %q by %q", got.Title, got.Artist, "New Title", "New Artist")
	}
}

// isOggPath reports whether the track at path is an Ogg file.
func isOggPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ExtOPUS, ExtOGG, ExtOGA:
		return true
	}
	return false
}

func TestWrite_InvalidCoverLeavesFLACWithID3v2HeaderAsItWas(t *testing.T) {
	dir := t.TempDir()
	path := createTestFLACWithID3v2Header(t, dir)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	tag := newTags()
	tag.CoverArt = []byte("not an image")
	if err := Write(path, tag); err == nil {
		t.Fatal("Write() with a cover that is not an image succeeded, want an error")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Error("the failed write changed the file")
	}
	assertOnlyFile(t, dir, filepath.Base(path))
}

func TestWrite_ReplacesTrackWholeKeepingItsPermissions(t *testing.T) {
	for _, fx := range wholeWriteFixtures {
		// 0444: a track its owner may not write is still tagged, as it
		// is replaced.
		for _, mode := range []os.FileMode{0o600, 0o444} {
			t.Run(fmt.Sprintf("%s/%o", fx.name, mode), func(t *testing.T) {
				dir := t.TempDir()
				path := fx.make(t, dir)
				if err := os.Chmod(path, mode); err != nil {
					t.Fatal(err)
				}
				audio := trackAudio(t, path)

				if err := Write(path, newTags()); err != nil {
					t.Fatalf("Write() error: %v", err)
				}

				assertNewTags(t, path, audio)
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if perm := info.Mode().Perm(); perm != mode {
					t.Errorf("mode = %o, want %o", perm, mode)
				}
				assertOnlyFile(t, dir, filepath.Base(path))
			})
		}
	}
}

func TestWrite_StartsAfreshOverLeftoverOfWriteCutShort(t *testing.T) {
	for _, fx := range wholeWriteFixtures {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			path := fx.make(t, dir)
			audio := trackAudio(t, path)
			leftover := bytes.Repeat([]byte("half a track "), 100000)
			if err := os.WriteFile(path+".part", leftover, 0o600); err != nil {
				t.Fatal(err)
			}

			if err := Write(path, newTags()); err != nil {
				t.Fatalf("Write() error: %v", err)
			}

			assertNewTags(t, path, audio)
			assertOnlyFile(t, dir, filepath.Base(path))
		})
	}
}

func TestWrite_SymlinkToTrackTagsFileItPointsTo(t *testing.T) {
	for _, fx := range wholeWriteFixtures {
		t.Run(fx.name, func(t *testing.T) {
			trackDir, linkDir := t.TempDir(), t.TempDir()
			track := fx.make(t, trackDir)
			audio := trackAudio(t, track)
			linkName := "link" + filepath.Ext(track)
			link := filepath.Join(linkDir, linkName)
			if err := os.Symlink(track, link); err != nil {
				t.Skipf("symlink: %v", err)
			}

			if err := Write(link, newTags()); err != nil {
				t.Fatalf("Write() error: %v", err)
			}

			info, err := os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Error("the link is no longer a symbolic link")
			}
			assertNewTags(t, track, audio)
			assertOnlyFile(t, trackDir, filepath.Base(track))
			assertOnlyFile(t, linkDir, linkName)
		})
	}
}
