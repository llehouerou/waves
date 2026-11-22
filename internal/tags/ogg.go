package tags

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
)

// The check of an Ogg file tagged on a copy: TagLib reports nothing when its
// own writes fail, so the copy is read back and compared with the track
// before it replaces it. A tag write changes the comment header, the second
// packet of the first logical stream, and nothing else: every other packet of
// that stream in the copy must be the track's. Every page of that stream in
// the copy must be whole, with a right checksum, and follow the one before
// it, unless the track has that very page, flaw included: a track already
// damaged stays taggable. What is not of that stream (the pages of another
// stream, chained or multiplexed, and bytes that are no page) must be in the
// copy as it is in the track.

// oggCommentPacket is the index of the comment header in the packets of an
// Opus, Vorbis or FLAC stream.
const oggCommentPacket = 1

// oggHeaderSize is the size of an Ogg page header, before its segment table.
const oggHeaderSize = 27

// checkOggCopy returns an error unless the Ogg file at copyPath, tagged on a
// copy of the track at trackPath, holds the track's packets but its comment
// header, in pages with no flaw the track has not at the same place.
func checkOggCopy(trackPath, copyPath string) error {
	track, err := scanOggFile(trackPath)
	if err != nil {
		return fmt.Errorf("read the track: %w", err)
	}
	cp, err := scanOggFile(copyPath)
	if err != nil {
		return fmt.Errorf("read the copy: %w", err)
	}
	return compareOgg(track, cp)
}

// oggPlace is where a page starts in the packets of its stream: the packet
// its body starts in, or starts, and the offset in that packet. nth tells
// apart the pages with no body that start at the same place.
type oggPlace struct {
	packet int
	offset int
	nth    int
}

// oggPage is what the check knows of a page of the first logical stream.
type oggPage struct {
	offset int64 // in the file
	at     oggPlace
	// sum is the digest of the bytes of the page in the file, fewer than
	// its own when it is cut short, but its sequence number and checksum.
	sum   [sha256.Size]byte
	whole bool
	// comment reports a page that carries part of the comment header.
	comment bool
	// sumOK reports a right checksum, for a whole page.
	sumOK bool
	// follows reports a page sequence number one past the page before; a
	// first page, or one whose header is cut short, follows.
	follows bool
}

// oggPacket is a packet, by its digest.
type oggPacket struct {
	sum   [sha256.Size]byte
	size  int
	whole bool
}

// oggScan is an Ogg file read for the check.
type oggScan struct {
	pages   []oggPage
	packets []oggPacket
	// others is the digest of what is not of the first logical stream, in
	// the order of the file.
	others [sha256.Size]byte
}

// scanOggFile reads the Ogg file at path.
func scanOggFile(path string) (*oggScan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return scanOgg(f)
}

// scanOgg reads the pages of the first logical stream of an Ogg file from r,
// and the packets they carry; the rest goes into a digest. It fails on what
// it cannot follow in that stream: a page that continues no packet or a
// packet that a page does not continue.
func scanOgg(r io.Reader) (*oggScan, error) {
	s := &oggScanner{
		r:      bufio.NewReader(r),
		packet: sha256.New(),
		others: sha256.New(),
		nth:    map[[2]int]int{},
	}
	for {
		done, err := s.page()
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
	}
	if s.pending {
		s.endPacket(false)
	}
	if len(s.scan.pages) == 0 {
		return nil, errors.New("no Ogg page")
	}
	s.others.Sum(s.scan.others[:0])
	return &s.scan, nil
}

// oggScanner is the state of scanOgg.
type oggScanner struct {
	r      *bufio.Reader
	scan   oggScan
	offset int64

	// The first logical stream: its serial number, the sequence number of
	// its last page so far, and whether a page has ended it.
	serial  uint32
	lastSeq uint32
	ended   bool

	// The packet being read: its digest so far, its size, and whether it
	// waits for the next page.
	packet  hash.Hash
	size    int
	pending bool

	// others is the digest of what is not of the first logical stream.
	others hash.Hash

	// nth counts the pages that started at each place.
	nth map[[2]int]int
}

// page reads the next page, and reports done at the end of the file, or
// past bytes that are no page.
func (s *oggScanner) page() (done bool, err error) {
	header := make([]byte, oggHeaderSize)
	n, cut, err := readFull(s.r, header)
	if err != nil || n == 0 {
		return true, err
	}
	first := len(s.scan.pages) == 0
	if capture := min(n, 4); string(header[:capture]) != "OggS"[:capture] {
		if first {
			return false, errors.New("no Ogg page at the start of the file")
		}
		// No page: the rest of the file is kept as it is.
		s.others.Write(header[:n])
		_, err := io.Copy(s.others, s.r)
		return true, err
	}
	if cut && s.ended {
		s.others.Write(header[:n])
		return true, nil
	}
	serial := binary.LittleEndian.Uint32(header[14:18])
	if !cut && !first && (s.ended || serial != s.serial) {
		return s.otherPage(header)
	}
	page := oggPage{offset: s.offset, at: s.place(), follows: true}
	if cut {
		// The header is cut short: the file ends.
		s.addPage(page, header[:n])
		s.pending = true
		return true, nil
	}
	if header[4] != 0 {
		return false, fmt.Errorf("page at offset %d: Ogg version %d", s.offset, header[4])
	}
	continued := header[5]&0x01 != 0
	s.ended = header[5]&0x04 != 0
	seq := binary.LittleEndian.Uint32(header[18:22])
	if first {
		s.serial = serial
	} else {
		page.follows = seq == s.lastSeq+1
	}
	s.lastSeq = seq
	if continued != s.pending {
		if continued {
			return false, fmt.Errorf("page at offset %d continues no packet", s.offset)
		}
		return false, fmt.Errorf("page at offset %d does not continue the packet before it", s.offset)
	}

	lacing, body, cut, err := s.readSegments(header)
	if err != nil {
		return false, err
	}
	page.whole = !cut
	if page.whole {
		page.sumOK = oggChecksum(header, lacing, body) == binary.LittleEndian.Uint32(header[22:26])
	}
	firstPacket := len(s.scan.packets)
	done = s.addPackets(lacing, body)
	lastPacket := len(s.scan.packets) - 1
	if s.pending {
		lastPacket++
	}
	page.comment = firstPacket <= oggCommentPacket && lastPacket >= oggCommentPacket
	s.addPage(page, header, lacing, body)
	return done || cut, nil
}

// readSegments reads the segment table of the page whose header is given,
// and its body, and reports cut when the file ends first. What was read is
// returned.
func (s *oggScanner) readSegments(header []byte) (lacing, body []byte, cut bool, err error) {
	lacing = make([]byte, header[26])
	n, cut, err := readFull(s.r, lacing)
	if err != nil || cut {
		return lacing[:n], nil, cut, err
	}
	bodySize := 0
	for _, l := range lacing {
		bodySize += int(l)
	}
	body = make([]byte, bodySize)
	n, cut, err = readFull(s.r, body)
	return lacing, body[:n], cut, err
}

// addPackets adds the segments of a page to the packets; body is cut short
// when the page is. It reports done when the page ends in a packet it cuts.
func (s *oggScanner) addPackets(lacing, body []byte) (done bool) {
	for _, l := range lacing {
		if int(l) > len(body) {
			s.addToPacket(body)
			s.pending = true
			return true
		}
		s.addToPacket(body[:l])
		body = body[l:]
		s.pending = l == 255
		if !s.pending {
			s.endPacket(true)
		}
	}
	return false
}

// addPage adds a page of the first logical stream, made of header and the
// parts that follow it. Its digest leaves out its sequence number and its
// checksum, which TagLib writes anew when it renumbers the pages.
func (s *oggScanner) addPage(page oggPage, header []byte, parts ...[]byte) {
	h := sha256.New()
	kept := append([]byte(nil), header...)
	clear(kept[min(len(kept), 18):min(len(kept), 26)])
	h.Write(kept)
	s.offset += int64(len(header))
	for _, part := range parts {
		h.Write(part)
		s.offset += int64(len(part))
	}
	h.Sum(page.sum[:0])
	s.scan.pages = append(s.scan.pages, page)
}

// otherPage reads a page that is not of the first logical stream, whose
// header is given, into the digest of the others.
func (s *oggScanner) otherPage(header []byte) (done bool, err error) {
	lacing, body, cut, err := s.readSegments(header)
	if err != nil {
		return false, err
	}
	for _, part := range [][]byte{header, lacing, body} {
		s.others.Write(part)
		s.offset += int64(len(part))
	}
	return cut, nil
}

// readFull reads len(buf) bytes from r into buf, and reports cut when the
// stream ends first, after n bytes.
func readFull(r io.Reader, buf []byte) (n int, cut bool, err error) {
	n, err = io.ReadFull(r, buf)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return n, true, nil
	}
	return n, false, err
}

// place returns where the next page starts in the packets, and counts it.
func (s *oggScanner) place() oggPlace {
	key := [2]int{len(s.scan.packets), s.size}
	nth := s.nth[key]
	s.nth[key] = nth + 1
	return oggPlace{packet: key[0], offset: key[1], nth: nth}
}

// addToPacket adds data to the packet being read.
func (s *oggScanner) addToPacket(data []byte) {
	s.packet.Write(data)
	s.size += len(data)
}

// endPacket ends the packet being read, whole or cut short.
func (s *oggScanner) endPacket(whole bool) {
	p := oggPacket{size: s.size, whole: whole}
	s.packet.Sum(p.sum[:0])
	s.scan.packets = append(s.scan.packets, p)
	s.packet.Reset()
	s.size = 0
	s.pending = false
}

// compareOgg returns an error unless the copy holds the packets of the track
// but its comment header, which must be whole, has no flaw in its pages that
// the track has not in the same page, and is the track outside its first
// logical stream.
func compareOgg(track, cp *oggScan) error {
	if cp.others != track.others {
		return errors.New("the copy is not the track outside its first logical stream")
	}
	if len(cp.packets) != len(track.packets) {
		return fmt.Errorf("the copy has %d packets, the track %d", len(cp.packets), len(track.packets))
	}
	for i, p := range cp.packets {
		if i == oggCommentPacket {
			if !p.whole {
				return errors.New("the comment header of the copy is cut short")
			}
			continue
		}
		if p != track.packets[i] {
			return fmt.Errorf("packet %d of the copy is not the track's", i)
		}
	}
	trackPages := make(map[oggPlace]oggPage, len(track.pages))
	for _, p := range track.pages {
		trackPages[p.at] = p
	}
	for _, p := range cp.pages {
		tp, inTrack := trackPages[p.at]
		if err := checkOggPage(p, tp, inTrack); err != nil {
			return err
		}
	}
	return nil
}

// checkOggPage returns an error when the page p of the copy has a flaw that
// tp, the page of the track at the same place if inTrack, has not: a flawed
// page is kept only as it is in the track, renumbered at most, and never when
// it carries the comment header, which the copy must have written anew.
func checkOggPage(p, tp oggPage, inTrack bool) error {
	asInTrack := inTrack && !p.comment && p.sum == tp.sum
	// The digest leaves the checksum out: a flaw is kept only where the
	// track's page has the same one.
	cutInTrack := asInTrack && !tp.whole
	badSumInTrack := asInTrack && tp.whole && !tp.sumOK
	switch {
	case !p.whole && !cutInTrack:
		return fmt.Errorf("the page at offset %d of the copy is cut short", p.offset)
	case p.whole && !p.sumOK && !badSumInTrack:
		return fmt.Errorf("the page at offset %d of the copy fails its checksum", p.offset)
	case !p.follows && (!inTrack || tp.follows):
		return fmt.Errorf("the page at offset %d of the copy does not follow the one before it", p.offset)
	}
	return nil
}

// oggCRCTable is the table of the Ogg checksum: a CRC-32 of polynomial
// 0x04c11db7, with no reflection.
var oggCRCTable = func() (table [256]uint32) {
	for i := range table {
		r := uint32(i) << 24 //nolint:gosec // i < 256
		for range 8 {
			if r&0x80000000 != 0 {
				r = r<<1 ^ 0x04c11db7
			} else {
				r <<= 1
			}
		}
		table[i] = r
	}
	return table
}()

// oggChecksum returns the checksum of a page, computed with its checksum
// field at zero.
func oggChecksum(header, lacing, body []byte) uint32 {
	var crc uint32
	update := func(data []byte) {
		for _, b := range data {
			crc = crc<<8 ^ oggCRCTable[byte(crc>>24)^b]
		}
	}
	update(header[:22])
	update([]byte{0, 0, 0, 0})
	update(header[26:])
	update(lacing)
	update(body)
	return crc
}
