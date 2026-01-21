package player

import (
	"bytes"
	"testing"
)

func TestParseOggPageHeader(t *testing.T) {
	// Valid Ogg page header (27 bytes minimum)
	// "OggS" + version(1) + flags(1) + granule(8) + serial(4) + sequence(4) + checksum(4) + segments(1)
	header := []byte{
		'O', 'g', 'g', 'S', // capture pattern
		0,                      // version
		0,                      // flags
		0, 0, 0, 0, 0, 0, 0, 0, // granule position (0)
		1, 0, 0, 0, // serial number
		0, 0, 0, 0, // sequence number
		0, 0, 0, 0, // checksum (ignored for now)
		1,   // 1 segment
		255, // segment table: 255 bytes
	}

	r := bytes.NewReader(header)
	hdr, err := parseOggPageHeader(r)
	if err != nil {
		t.Fatalf("parseOggPageHeader failed: %v", err)
	}

	if hdr.GranulePos != 0 {
		t.Errorf("GranulePos = %d, want 0", hdr.GranulePos)
	}
	if hdr.SerialNumber != 1 {
		t.Errorf("SerialNumber = %d, want 1", hdr.SerialNumber)
	}
	if hdr.NumSegments != 1 {
		t.Errorf("NumSegments = %d, want 1", hdr.NumSegments)
	}
	if len(hdr.SegmentTable) != 1 || hdr.SegmentTable[0] != 255 {
		t.Errorf("SegmentTable = %v, want [255]", hdr.SegmentTable)
	}
}

func TestParseOggPageHeader_InvalidMagic(t *testing.T) {
	header := []byte{'B', 'a', 'd', 'S', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	r := bytes.NewReader(header)
	_, err := parseOggPageHeader(r)
	if err == nil {
		t.Error("expected error for invalid magic, got nil")
	}
}

func TestParseOggPageHeader_GranulePosition(t *testing.T) {
	// Test with specific granule position: 48000 (1 second at 48kHz)
	header := []byte{
		'O', 'g', 'g', 'S',
		0,
		0,
		0x80, 0xBB, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 48000 in little-endian
		1, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, // 0 segments
	}

	r := bytes.NewReader(header)
	hdr, err := parseOggPageHeader(r)
	if err != nil {
		t.Fatalf("parseOggPageHeader failed: %v", err)
	}

	if hdr.GranulePos != 48000 {
		t.Errorf("GranulePos = %d, want 48000", hdr.GranulePos)
	}
}
