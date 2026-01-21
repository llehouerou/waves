package player

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

const oggMagic = "OggS"

var (
	errInvalidOggMagic    = errors.New("ogg: invalid capture pattern")
	errInvalidOggVersion  = errors.New("ogg: unsupported version")
	errInvalidOpusHead    = errors.New("opus: invalid OpusHead packet")
	errUnsupportedOpus    = errors.New("opus: unsupported version")
	errVorbisNotSupported = errors.New("ogg: Vorbis codec not supported (only Opus)")
)

// oggPageHeader represents the header of an Ogg page.
type oggPageHeader struct {
	GranulePos   int64
	SerialNumber uint32
	SequenceNum  uint32
	NumSegments  uint8
	SegmentTable []uint8
}

// parseOggPageHeader reads and parses an Ogg page header from the reader.
func parseOggPageHeader(r io.Reader) (*oggPageHeader, error) {
	// Read fixed header (27 bytes)
	var buf [27]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, err
	}

	// Check capture pattern "OggS"
	if string(buf[0:4]) != oggMagic {
		return nil, errInvalidOggMagic
	}

	// Check version (must be 0)
	if buf[4] != 0 {
		return nil, errInvalidOggVersion
	}

	hdr := &oggPageHeader{
		GranulePos:   int64(binary.LittleEndian.Uint64(buf[6:14])), //nolint:gosec // granule position is semantically signed (-1 valid) but stored as unsigned
		SerialNumber: binary.LittleEndian.Uint32(buf[14:18]),
		SequenceNum:  binary.LittleEndian.Uint32(buf[18:22]),
		// checksum at buf[22:26] - skip validation for now
		NumSegments: buf[26],
	}

	// Read segment table
	if hdr.NumSegments > 0 {
		hdr.SegmentTable = make([]uint8, hdr.NumSegments)
		if _, err := io.ReadFull(r, hdr.SegmentTable); err != nil {
			return nil, err
		}
	}

	return hdr, nil
}

// readOggPageBody reads the page body and extracts packets.
// Packets are delimited by segment sizes: a segment of 255 bytes continues
// to the next segment, while a segment < 255 terminates the packet.
func readOggPageBody(r io.Reader, hdr *oggPageHeader) ([][]byte, error) {
	// Calculate total body size
	var totalSize int
	for _, seg := range hdr.SegmentTable {
		totalSize += int(seg)
	}

	// Read entire body
	body := make([]byte, totalSize)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	// Extract packets from segments
	var packets [][]byte
	var currentPacket []byte
	offset := 0

	for _, segSize := range hdr.SegmentTable {
		currentPacket = append(currentPacket, body[offset:offset+int(segSize)]...)
		offset += int(segSize)

		// Segment < 255 terminates the packet
		if segSize < 255 {
			packets = append(packets, currentPacket)
			currentPacket = nil
		}
	}

	// If last segment was 255, packet continues to next page (incomplete)
	// For now, include incomplete packet if any data remains
	if len(currentPacket) > 0 {
		packets = append(packets, currentPacket)
	}

	return packets, nil
}

// opusHead contains the Opus identification header data.
type opusHead struct {
	Channels   uint8
	PreSkip    uint16
	SampleRate uint32
}

// parseOpusHead parses an OpusHead identification packet.
// Returns errVorbisNotSupported if the packet contains Vorbis identification.
func parseOpusHead(data []byte) (*opusHead, error) {
	if len(data) < 8 {
		return nil, errInvalidOpusHead
	}

	// Check for Vorbis codec (starts with \x01vorbis)
	if len(data) >= 7 && data[0] == 0x01 && string(data[1:7]) == "vorbis" {
		return nil, errVorbisNotSupported
	}

	// Check magic "OpusHead"
	if string(data[0:8]) != "OpusHead" {
		return nil, errInvalidOpusHead
	}

	if len(data) < 19 {
		return nil, errInvalidOpusHead
	}

	// Check version (must be 1)
	if data[8] != 1 {
		return nil, errUnsupportedOpus
	}

	return &opusHead{
		Channels:   data[9],
		PreSkip:    binary.LittleEndian.Uint16(data[10:12]),
		SampleRate: binary.LittleEndian.Uint32(data[12:16]),
	}, nil
}

// IsValidOpusFile checks if an .ogg file contains Opus codec (not Vorbis).
// Returns true for Opus files, false for Vorbis or invalid files.
func IsValidOpusFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Read first page header
	hdr, err := parseOggPageHeader(f)
	if err != nil {
		return false
	}

	// Read first packet (codec identification)
	packets, err := readOggPageBody(f, hdr)
	if err != nil || len(packets) == 0 {
		return false
	}

	// Check if it's Opus
	_, err = parseOpusHead(packets[0])
	return err == nil
}

// OggReader reads Ogg/Opus streams with seeking support.
type OggReader struct {
	r           io.ReadSeeker
	fileSize    int64
	head        *opusHead
	dataStart   int64 // byte offset where audio pages begin
	lastGranule int64 // cached from last page
}

// NewOggReader creates a new OggReader from a seekable stream.
// It parses the Opus headers and prepares for reading/seeking.
func NewOggReader(r io.ReadSeeker) (*OggReader, error) {
	// Get file size
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	ogr := &OggReader{
		r:        r,
		fileSize: size,
	}

	// Read first page (must contain OpusHead)
	hdr, err := parseOggPageHeader(r)
	if err != nil {
		return nil, err
	}
	packets, err := readOggPageBody(r, hdr)
	if err != nil {
		return nil, err
	}
	if len(packets) == 0 {
		return nil, errInvalidOpusHead
	}
	ogr.head, err = parseOpusHead(packets[0])
	if err != nil {
		return nil, err
	}

	// Read second page (OpusTags) - skip it
	hdr, err = parseOggPageHeader(r)
	if err != nil {
		return nil, err
	}
	if _, err := readOggPageBody(r, hdr); err != nil {
		return nil, err
	}

	// Record where audio data starts
	ogr.dataStart, err = r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// Scan to find last page's granule position for duration
	if err := ogr.scanLastGranule(); err != nil {
		return nil, err
	}

	// Seek back to start of audio data
	if _, err := r.Seek(ogr.dataStart, io.SeekStart); err != nil {
		return nil, err
	}

	return ogr, nil
}

// Channels returns the number of audio channels.
func (o *OggReader) Channels() int {
	return int(o.head.Channels)
}

// PreSkip returns the number of samples to skip at the start.
func (o *OggReader) PreSkip() int {
	return int(o.head.PreSkip)
}

// OggPage represents a decoded Ogg page with its audio packets.
type OggPage struct {
	GranulePos int64
	Packets    [][]byte
	ByteOffset int64
}

// ReadPage reads the next Ogg page from the stream.
// Returns io.EOF when no more pages are available.
func (o *OggReader) ReadPage() (*OggPage, error) {
	offset, err := o.r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	hdr, err := parseOggPageHeader(o.r)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, io.EOF
		}
		return nil, err
	}

	packets, err := readOggPageBody(o.r, hdr)
	if err != nil {
		return nil, err
	}

	return &OggPage{
		GranulePos: hdr.GranulePos,
		Packets:    packets,
		ByteOffset: offset,
	}, nil
}

// Reset seeks back to the start of audio data.
func (o *OggReader) Reset() error {
	_, err := o.r.Seek(o.dataStart, io.SeekStart)
	return err
}

// scanLastGranule finds the granule position of the last page.
func (o *OggReader) scanLastGranule() error {
	// Seek near end of file and scan for last page
	searchSize := min(int64(65536), o.fileSize) // Search last 64KB

	if _, err := o.r.Seek(o.fileSize-searchSize, io.SeekStart); err != nil {
		return err
	}

	// Read and scan for "OggS" magic
	buf := make([]byte, searchSize)
	n, err := io.ReadFull(o.r, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	buf = buf[:n]

	// Find last "OggS" occurrence
	lastOggS := -1
	for i := len(buf) - 4; i >= 0; i-- {
		if string(buf[i:i+4]) == oggMagic {
			lastOggS = i
			break
		}
	}

	if lastOggS == -1 {
		return errors.New("ogg: no page found at end of file")
	}

	// Parse granule position from that header
	if lastOggS+14 > len(buf) {
		return errors.New("ogg: incomplete last page header")
	}
	o.lastGranule = int64(binary.LittleEndian.Uint64(buf[lastOggS+6 : lastOggS+14])) //nolint:gosec // granule position is defined as unsigned but used as signed for duration calculations

	return nil
}

// Duration returns the total number of audio samples (excluding pre-skip).
func (o *OggReader) Duration() int64 {
	return o.lastGranule - int64(o.head.PreSkip)
}

// SeekToGranule seeks to the page containing or just before the target granule position.
// Uses bisection search for efficiency on large files.
func (o *OggReader) SeekToGranule(target int64) error {
	// Handle seek to start
	if target <= 0 {
		return o.Reset()
	}

	// Bisection search
	low := o.dataStart
	high := o.fileSize

	bestOffset := o.dataStart
	var bestGranule int64

	for high-low > 4096 { // Stop when range is small enough
		mid := (low + high) / 2

		offset, granule, err := o.findPageNear(mid)
		if err != nil {
			// No page found in upper half, search lower
			high = mid
			continue
		}

		if granule <= target {
			// This page is valid, remember it and search higher
			bestOffset = offset
			bestGranule = granule
			low = offset + 1
		} else {
			// This page is past target, search lower
			high = mid
		}
	}

	// Linear scan from best known position to find exact page
	if _, err := o.r.Seek(bestOffset, io.SeekStart); err != nil {
		return err
	}

	// Scan forward to find the last page with granule ≤ target
	for {
		offset, err := o.r.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		hdr, err := parseOggPageHeader(o.r)
		if err != nil {
			break
		}

		// Skip page body
		var bodySize int
		for _, seg := range hdr.SegmentTable {
			bodySize += int(seg)
		}
		if _, err := o.r.Seek(int64(bodySize), io.SeekCurrent); err != nil {
			break
		}

		if hdr.GranulePos > target {
			// Went past target, seek back to previous page
			if _, err := o.r.Seek(bestOffset, io.SeekStart); err != nil {
				return err
			}
			break
		}

		if hdr.GranulePos >= 0 { // -1 means no granule
			bestOffset = offset
			bestGranule = hdr.GranulePos
		}
	}

	// Seek to best page found
	_, err := o.r.Seek(bestOffset, io.SeekStart)
	_ = bestGranule // Used for debugging if needed
	return err
}

// findPageNear finds an Ogg page starting at or after the given offset.
// Returns the page's byte offset and granule position.
func (o *OggReader) findPageNear(offset int64) (pageOffset, granule int64, err error) {
	if _, err := o.r.Seek(offset, io.SeekStart); err != nil {
		return 0, 0, err
	}

	// Read a chunk and scan for "OggS"
	buf := make([]byte, 4096)
	n, readErr := o.r.Read(buf)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return 0, 0, readErr
	}
	buf = buf[:n]

	for i := range len(buf) - 27 {
		if string(buf[i:i+4]) == oggMagic && buf[i+4] == 0 { // version must be 0
			pageOffset = offset + int64(i)
			granule = int64(binary.LittleEndian.Uint64(buf[i+6 : i+14])) //nolint:gosec // granule position is semantically signed
			return pageOffset, granule, nil
		}
	}

	return 0, 0, errors.New("ogg: no page found")
}
