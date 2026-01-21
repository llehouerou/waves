package player

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	errInvalidOggMagic   = errors.New("ogg: invalid capture pattern")
	errInvalidOggVersion = errors.New("ogg: unsupported version")
	errInvalidOpusHead   = errors.New("opus: invalid OpusHead packet")
	errUnsupportedOpus   = errors.New("opus: unsupported version")
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
	if string(buf[0:4]) != "OggS" {
		return nil, errInvalidOggMagic
	}

	// Check version (must be 0)
	if buf[4] != 0 {
		return nil, errInvalidOggVersion
	}

	hdr := &oggPageHeader{
		GranulePos:   int64(binary.LittleEndian.Uint64(buf[6:14])),
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
func parseOpusHead(data []byte) (*opusHead, error) {
	if len(data) < 19 {
		return nil, errInvalidOpusHead
	}

	// Check magic "OpusHead"
	if string(data[0:8]) != "OpusHead" {
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

// OggReader reads Ogg/Opus streams with seeking support.
type OggReader struct {
	r         io.ReadSeeker
	fileSize  int64
	head      *opusHead
	dataStart int64 // byte offset where audio pages begin
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
