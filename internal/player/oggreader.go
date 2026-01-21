package player

import (
	"encoding/binary"
	"errors"
	"io"
)

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
