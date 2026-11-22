package tags

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// Ogg pages and packets as the tests see them, read with no check, to tell
// what a tag write did to an Ogg file.

// testOggPage is a page of an Ogg file: its bytes, and where they start.
type testOggPage struct {
	offset int
	data   []byte
}

// seq returns the page sequence number of the page.
func (p testOggPage) seq() uint32 { return binary.LittleEndian.Uint32(p.data[18:22]) }

// segments returns the segment table of the page and its body.
func (p testOggPage) segments() (lacing, body []byte) {
	n := int(p.data[26])
	return p.data[27 : 27+n], p.data[27+n:]
}

// testOggPages splits data into its whole pages; what follows the last one,
// a page cut short or bytes that are no page, is returned apart.
func testOggPages(t *testing.T, data []byte) (pages []testOggPage, rest []byte) {
	t.Helper()
	at := 0
	for at < len(data) {
		if len(data)-at < 27 {
			return pages, data[at:]
		}
		if string(data[at:at+4]) != "OggS" {
			return pages, data[at:]
		}
		n := int(data[at+26])
		if len(data)-at < 27+n {
			return pages, data[at:]
		}
		size := 27 + n
		for _, l := range data[at+27 : at+27+n] {
			size += int(l)
		}
		if len(data)-at < size {
			return pages, data[at:]
		}
		pages = append(pages, testOggPage{offset: at, data: data[at : at+size]})
		at += size
	}
	return pages, nil
}

// testOggPackets returns the packets that end on a whole page of the Ogg file
// at path.
func testOggPackets(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	pages, _ := testOggPages(t, data)
	var packets [][]byte
	var packet []byte
	for _, p := range pages {
		lacing, body := p.segments()
		for _, l := range lacing {
			packet = append(packet, body[:l]...)
			body = body[l:]
			if l < 255 {
				packets = append(packets, packet)
				packet = nil
			}
		}
	}
	return packets
}

// oggAudio returns the packets of the Ogg file at path but its comment
// header, each behind its length.
func oggAudio(t *testing.T, path string) []byte {
	t.Helper()
	var audio []byte
	for i, p := range testOggPackets(t, path) {
		if i == 1 {
			continue
		}
		audio = binary.BigEndian.AppendUint32(audio, uint32(len(p))) //nolint:gosec // a packet of a test file
		audio = append(audio, p...)
	}
	return audio
}

// joinOggPages returns the bytes of pages, one after the other.
func joinOggPages(pages []testOggPage) []byte {
	var b bytes.Buffer
	for _, p := range pages {
		b.Write(p.data)
	}
	return b.Bytes()
}
