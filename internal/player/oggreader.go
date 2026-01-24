package player

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

const oggMagic = "OggS"

var (
	errInvalidOggMagic   = errors.New("ogg: invalid capture pattern")
	errInvalidOggVersion = errors.New("ogg: unsupported version")
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
// Returns complete packets and any partial packet that continues to the next page.
func readOggPageBody(r io.Reader, hdr *oggPageHeader) (packets [][]byte, partial []byte, err error) {
	// Calculate total body size
	var totalSize int
	for _, seg := range hdr.SegmentTable {
		totalSize += int(seg)
	}

	// Read entire body
	body := make([]byte, totalSize)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, nil, err
	}

	// Extract packets from segments
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
	// Return it as partial for the caller to join with the next page
	if len(currentPacket) > 0 {
		partial = currentPacket
	}

	return packets, partial, nil
}

// IsOpusCodec checks if an .ogg file contains Opus codec (not Vorbis).
// Returns true for Opus files, false for Vorbis or invalid files.
func IsOpusCodec(path string) bool {
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
	packets, _, err := readOggPageBody(f, hdr)
	if err != nil || len(packets) == 0 {
		return false
	}

	// Check if it starts with "OpusHead"
	if len(packets[0]) < 8 {
		return false
	}
	return string(packets[0][:8]) == opusHeadMagic
}

// OggReader reads Ogg streams with seeking support.
// It is codec-agnostic - the caller is responsible for parsing codec headers.
type OggReader struct {
	r           io.ReadSeeker
	fileSize    int64
	dataStart   int64 // byte offset where audio pages begin
	lastGranule int64 // cached from last page
	sampleRate  int   // for duration calculation
	preSkip     int   // samples to skip at start (Opus only, 0 for Vorbis)

	partial []byte // partial packet from previous page (continues on next page)
}

// NewOggReader creates a new OggReader from a seekable stream.
// sampleRate and preSkip are needed for duration/seeking calculations.
// The caller is responsible for parsing codec headers before calling this.
func NewOggReader(r io.ReadSeeker, sampleRate, preSkip int) (*OggReader, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	return &OggReader{
		r:          r,
		fileSize:   size,
		sampleRate: sampleRate,
		preSkip:    preSkip,
	}, nil
}

// SetDataStart sets the byte offset where audio data begins.
// Called after header parsing is complete.
func (o *OggReader) SetDataStart(offset int64) {
	o.dataStart = offset
}

// SampleRate returns the sample rate used for duration calculations.
func (o *OggReader) SampleRate() int {
	return o.sampleRate
}

// PreSkip returns the number of samples to skip at the start.
func (o *OggReader) PreSkip() int {
	return o.preSkip
}

// OggPage represents a decoded Ogg page with its audio packets.
type OggPage struct {
	GranulePos int64
	Packets    [][]byte
	ByteOffset int64
}

// ReadPage reads the next Ogg page from the stream.
// Handles packets that span multiple pages by joining partial data.
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

	packets, partial, err := readOggPageBody(o.r, hdr)
	if err != nil {
		return nil, err
	}

	// If we have a partial packet from the previous page, prepend it to the first packet
	if len(o.partial) > 0 {
		switch {
		case len(packets) > 0:
			// Join partial with first complete packet
			packets[0] = append(o.partial, packets[0]...)
		case partial != nil:
			// No complete packets, join partial with new partial
			partial = append(o.partial, partial...)
		default:
			// This shouldn't happen in valid Ogg streams
			packets = append(packets, o.partial)
		}
		o.partial = nil
	}

	// Store new partial for next page
	o.partial = partial

	return &OggPage{
		GranulePos: hdr.GranulePos,
		Packets:    packets,
		ByteOffset: offset,
	}, nil
}

// Reset seeks back to the start of audio data.
func (o *OggReader) Reset() error {
	o.partial = nil // Clear any partial packet
	_, err := o.r.Seek(o.dataStart, io.SeekStart)
	return err
}

// ScanLastGranule finds the granule position of the last page.
// This should be called after header parsing to enable duration calculation.
func (o *OggReader) ScanLastGranule() error {
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
	return o.lastGranule - int64(o.preSkip)
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
