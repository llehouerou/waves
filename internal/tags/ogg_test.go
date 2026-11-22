package tags

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.senan.xyz/taglib"
)

// oggKindsWithCover are the Ogg tracks TagLib writes a cover on.
var oggKindsWithCover = []wholeWriteFixture{
	{"Opus", func(t *testing.T, dir string) string { return createTestOpus(t, dir, nil) }},
	{"Vorbis", func(t *testing.T, dir string) string { return createTestVorbis(t, dir, nil) }},
}

// oggKinds are the Ogg tracks the check is tested on.
var oggKinds = []wholeWriteFixture{
	oggKindsWithCover[0],
	oggKindsWithCover[1],
	{oggFLAC, createTestOggFLAC},
	chainedVorbis,
}

// chainedVorbis is a Vorbis file of two logical streams, one after the other.
var chainedVorbis = wholeWriteFixture{"chained Vorbis", createTestChainedVorbis}

// oggFLAC names a FLAC in Ogg among oggKinds.
const oggFLAC = "FLAC in Ogg"

// createTestChainedVorbis creates a Vorbis file of two logical streams, one
// after the other.
func createTestChainedVorbis(t *testing.T, dir string) string {
	t.Helper()
	serials := []string{"1000", "2000"}
	chain := make([]byte, 0, len(serials)*16*1024)
	for i, serial := range serials {
		part := filepath.Join(dir, fmt.Sprintf("part%d.ogg", i))
		cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", //nolint:gosec // a test file
			"-c:a", "libvorbis", "-serial_offset", serial, part)
		if err := cmd.Run(); err != nil {
			t.Skipf("ffmpeg not available: %v", err)
		}
		chain = append(chain, readTrack(t, part)...)
		if err := os.Remove(part); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "chained.ogg")
	if err := os.WriteFile(path, chain, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// firstStreamEnd returns the offset in data past the page that ends the
// first logical stream.
func firstStreamEnd(t *testing.T, data []byte) int {
	t.Helper()
	pages, _ := testOggPages(t, data)
	for _, p := range pages {
		if p.data[5]&0x04 != 0 {
			return p.offset + len(p.data)
		}
	}
	t.Fatal("no page ends the first logical stream")
	return 0
}

// id3v1Trailer is what some tools append to an Ogg file: an ID3v1 tag.
var id3v1Trailer = append([]byte("TAG"), make([]byte, 125)...)

// writeCopy writes data as the copy of the track at path, and returns the
// path of the copy.
func writeCopy(t *testing.T, path string, data []byte) string {
	t.Helper()
	cp := path + partSuffix
	if err := os.WriteFile(cp, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return cp
}

// readTrack returns the bytes of the track at path.
func readTrack(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// setPageChecksum gives the page its right checksum.
func setPageChecksum(p testOggPage) {
	lacing, body := p.segments()
	binary.LittleEndian.PutUint32(p.data[22:26], oggChecksum(p.data[:oggHeaderSize], lacing, body))
}

func TestCheckOggCopy_RefusesDamagedCopy(t *testing.T) {
	keep := func(_ *testing.T, data []byte) []byte { return data }
	damages := []struct {
		name string
		// track makes the track, from the one ffmpeg wrote; copy damages
		// the copy of the track.
		track, copy func(t *testing.T, data []byte) []byte
	}{
		{"cut short", keep, func(_ *testing.T, track []byte) []byte {
			return track[:len(track)-1500]
		}},
		{"a byte flipped in the audio", keep, func(t *testing.T, track []byte) []byte {
			return flipByteInPage(t, track, len(mustPages(t, track))-1)
		}},
		{"a byte flipped in the checksum of an audio page", keep, func(t *testing.T, track []byte) []byte {
			pages := mustPages(t, append([]byte(nil), track...))
			pages[len(pages)-1].data[22] ^= 0xff
			return joinOggPages(pages)
		}},
		{"a byte flipped in the comment header", keep, func(t *testing.T, track []byte) []byte {
			// The comment header may change: only the checksum of its
			// page tells.
			return flipByteInPage(t, track, 1)
		}},
		{"the flawed comment header page of the track kept", func(t *testing.T, track []byte) []byte {
			return flipByteInPage(t, track, 1)
		}, keep},
		{"a byte flipped past the last page", func(_ *testing.T, track []byte) []byte {
			return append(track, id3v1Trailer...)
		}, func(_ *testing.T, track []byte) []byte {
			damaged := append([]byte(nil), track...)
			damaged[len(damaged)-1] ^= 0xff
			return damaged
		}},
		{"a page removed", keep, func(t *testing.T, track []byte) []byte {
			pages := mustPages(t, track)
			i := len(pages) / 2
			return joinOggPages(append(pages[:i:i], pages[i+1:]...))
		}},
		{"a page sequence number skipped", keep, func(t *testing.T, track []byte) []byte {
			pages := mustPages(t, append([]byte(nil), track...))
			for _, p := range pages[len(pages)/2:] {
				binary.LittleEndian.PutUint32(p.data[18:22], p.seq()+1)
				setPageChecksum(p)
			}
			return joinOggPages(pages)
		}},
	}
	for _, kind := range oggKinds {
		for _, d := range damages {
			t.Run(kind.name+"/"+d.name, func(t *testing.T) {
				path := kind.make(t, t.TempDir())
				track := d.track(t, readTrack(t, path))
				if err := os.WriteFile(path, track, 0o600); err != nil {
					t.Fatal(err)
				}
				cp := writeCopy(t, path, d.copy(t, append([]byte(nil), track...)))

				if err := checkOggCopy(path, cp); err == nil {
					t.Error("checkOggCopy() of a damaged copy succeeded, want an error")
				}
			})
		}
	}
}

// flipByteInPage returns data with a byte flipped in the middle of the body
// of its page i.
func flipByteInPage(t *testing.T, data []byte, i int) []byte {
	t.Helper()
	p := mustPages(t, data)[i]
	_, body := p.segments()
	damaged := append([]byte(nil), data...)
	damaged[p.offset+len(p.data)-len(body)/2] ^= 0xff
	return damaged
}

// mustPages returns the pages of data, all of them whole.
func mustPages(t *testing.T, data []byte) []testOggPage {
	t.Helper()
	pages, rest := testOggPages(t, data)
	if len(rest) > 0 {
		t.Fatalf("%d bytes past the last whole page", len(rest))
	}
	return pages
}

func TestCheckOggCopy_AcceptsCopyTaggedNormally(t *testing.T) {
	covers := []struct {
		name  string
		cover []byte
	}{
		{"no cover", nil},
		{"a cover over several pages", testCover(100 * 1024)},
	}
	for _, kind := range oggKinds {
		for _, c := range covers {
			t.Run(kind.name+"/"+c.name, func(t *testing.T) {
				path := kind.make(t, t.TempDir())
				cp := writeCopy(t, path, readTrack(t, path))
				// The cover first: TagLib writes a FLAC in Ogg once, then
				// cannot open it again.
				if c.cover != nil {
					if err := taglib.WriteImage(cp, c.cover); err != nil {
						t.Fatalf("WriteImage() error: %v", err)
					}
				}
				if c.cover == nil || kind.name != oggFLAC {
					if err := taglib.WriteTags(cp, oggTags(newTags()), taglib.Clear); err != nil {
						t.Fatalf("WriteTags() error: %v", err)
					}
				}

				if err := checkOggCopy(path, cp); err != nil {
					t.Errorf("checkOggCopy() of a tagged copy: %v", err)
				}
			})
		}
	}
}

func TestCheckOggCopy_AcceptsCopyOfItself(t *testing.T) {
	for _, kind := range oggKinds {
		t.Run(kind.name, func(t *testing.T) {
			path := kind.make(t, t.TempDir())
			cp := writeCopy(t, path, readTrack(t, path))

			if err := checkOggCopy(path, cp); err != nil {
				t.Errorf("checkOggCopy() of an exact copy: %v", err)
			}
		})
	}
}

func TestWrite_DamagedOggTrackStaysTaggable(t *testing.T) {
	damages := []struct {
		name   string
		damage func(t *testing.T, track []byte) []byte
	}{
		{"cut 1500 bytes short", func(_ *testing.T, track []byte) []byte {
			return track[:len(track)-1500]
		}},
		{"a byte flipped in the audio of a page", func(t *testing.T, track []byte) []byte {
			return flipByteInPage(t, track, len(mustPages(t, track))-1)
		}},
	}
	covers := []struct {
		name  string
		cover []byte
	}{
		{"no cover", nil},
		{"a cover", testCover(100 * 1024)},
	}
	for _, kind := range oggKindsWithCover {
		for _, d := range damages {
			for _, c := range covers {
				t.Run(kind.name+"/"+d.name+"/"+c.name, func(t *testing.T) {
					dir := t.TempDir()
					path := kind.make(t, dir)
					if err := os.WriteFile(path, d.damage(t, readTrack(t, path)), 0o600); err != nil {
						t.Fatal(err)
					}
					audio := trackAudio(t, path)

					tag := newTags()
					tag.CoverArt = c.cover
					if err := Write(path, tag); err != nil {
						t.Fatalf("Write() on a damaged track: %v", err)
					}

					assertNewTags(t, path, audio)
					got, err := Read(path)
					if err != nil {
						t.Fatalf("Read() error: %v", err)
					}
					if got.Title != "New Title" {
						t.Errorf("Title = %q, want %q", got.Title, "New Title")
					}
					assertOnlyFile(t, dir, filepath.Base(path))
				})
			}
		}
	}
}

func TestWrite_OggCoverNotWrittenLeavesOldTags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.oga")
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", //nolint:gosec // a test file
		"-c:a", "flac", "-f", "ogg", "-metadata", "title=Old Title", path)
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg not available or doesn't support Ogg FLAC: %v", err)
	}
	before := readTrack(t, path)

	// TagLib writes the tags of a FLAC in Ogg made by ffmpeg, never its
	// cover.
	tag := newTags()
	tag.CoverArt = testCover(10 * 1024)
	err := Write(path, tag)
	if err == nil {
		t.Fatal("Write() of a cover TagLib cannot write succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "write cover art") {
		t.Errorf("Write() error = %q, want one from the write of the cover", err)
	}

	if after := readTrack(t, path); !bytes.Equal(after, before) {
		t.Error("the failed write changed the track: the new tags reached it without the cover")
	}
	assertOnlyFile(t, dir, filepath.Base(path))
}

func TestWrite_OggTrackWithMoreThanItsStreamStaysTaggable(t *testing.T) {
	kinds := []struct {
		fixture wholeWriteFixture
		// past returns where what follows the first logical stream starts.
		past func(t *testing.T, data []byte) int
	}{
		{chainedVorbis, firstStreamEnd},
		{wholeWriteFixture{"Vorbis with an ID3v1 tag", func(t *testing.T, dir string) string {
			path := createTestVorbis(t, dir, nil)
			if err := os.WriteFile(path, append(readTrack(t, path), id3v1Trailer...), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}}, func(_ *testing.T, data []byte) int { return len(data) - len(id3v1Trailer) }},
	}
	for _, kind := range kinds {
		for _, cover := range [][]byte{nil, testCover(100 * 1024)} {
			t.Run(fmt.Sprintf("%s/cover of %d bytes", kind.fixture.name, len(cover)), func(t *testing.T) {
				dir := t.TempDir()
				path := kind.fixture.make(t, dir)
				before := readTrack(t, path)
				rest := before[kind.past(t, before):]
				audio := trackAudio(t, path)

				tag := newTags()
				tag.CoverArt = cover
				if err := Write(path, tag); err != nil {
					t.Fatalf("Write() error: %v", err)
				}

				assertNewTags(t, path, audio)
				after := readTrack(t, path)
				if !bytes.Equal(after[kind.past(t, after):], rest) {
					t.Error("what follows the first logical stream changed")
				}
				assertOnlyFile(t, dir, filepath.Base(path))
			})
		}
	}
}
