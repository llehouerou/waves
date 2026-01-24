package player

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestReadOggPageBody_MultiPagePacket tests that packets spanning multiple Ogg pages
// are correctly identified as partial and can be joined.
//
// Bug: Our original implementation treated each page's content as separate packets,
// but Ogg allows a single packet to span multiple pages. When a segment is 255 bytes,
// the packet continues onto the next segment (or next page if it's the last segment).
func TestReadOggPageBody_MultiPagePacket(t *testing.T) {
	// Create a page where the last segment is 255 bytes (packet continues to next page)
	// Segment table: [255, 255, 100] means:
	//   - First 255 bytes: part of packet, continues
	//   - Next 255 bytes: part of packet, continues
	//   - Next 100 bytes: end of packet (< 255 terminates)
	// Then [255] at end means packet continues to next page

	t.Run("complete packet within page", func(t *testing.T) {
		// Segment table [100] = one complete 100-byte packet
		page := buildOggPage(t, 0, 0, []uint8{100})
		r := bytes.NewReader(page)

		hdr, err := parseOggPageHeader(r)
		if err != nil {
			t.Fatalf("parseOggPageHeader: %v", err)
		}

		packets, partial, err := readOggPageBody(r, hdr)
		if err != nil {
			t.Fatalf("readOggPageBody: %v", err)
		}

		if len(packets) != 1 {
			t.Errorf("expected 1 packet, got %d", len(packets))
		}
		if len(packets[0]) != 100 {
			t.Errorf("expected packet length 100, got %d", len(packets[0]))
		}
		if partial != nil {
			t.Errorf("expected no partial, got %d bytes", len(partial))
		}
	})

	t.Run("packet continues to next page", func(t *testing.T) {
		// Segment table [255, 255] = 510 bytes, packet continues (last seg is 255)
		page := buildOggPage(t, 0, 0, []uint8{255, 255})
		r := bytes.NewReader(page)

		hdr, err := parseOggPageHeader(r)
		if err != nil {
			t.Fatalf("parseOggPageHeader: %v", err)
		}

		packets, partial, err := readOggPageBody(r, hdr)
		if err != nil {
			t.Fatalf("readOggPageBody: %v", err)
		}

		if len(packets) != 0 {
			t.Errorf("expected 0 complete packets, got %d", len(packets))
		}
		if partial == nil {
			t.Fatal("expected partial packet, got nil")
		}
		if len(partial) != 510 {
			t.Errorf("expected partial length 510, got %d", len(partial))
		}
	})

	t.Run("mixed complete and partial", func(t *testing.T) {
		// Segment table [100, 255, 255] =
		//   - packet 1: 100 bytes (complete)
		//   - partial: 510 bytes (continues to next page)
		page := buildOggPage(t, 0, 0, []uint8{100, 255, 255})
		r := bytes.NewReader(page)

		hdr, err := parseOggPageHeader(r)
		if err != nil {
			t.Fatalf("parseOggPageHeader: %v", err)
		}

		packets, partial, err := readOggPageBody(r, hdr)
		if err != nil {
			t.Fatalf("readOggPageBody: %v", err)
		}

		if len(packets) != 1 {
			t.Errorf("expected 1 complete packet, got %d", len(packets))
		}
		if len(packets[0]) != 100 {
			t.Errorf("expected packet length 100, got %d", len(packets[0]))
		}
		if partial == nil {
			t.Fatal("expected partial packet, got nil")
		}
		if len(partial) != 510 {
			t.Errorf("expected partial length 510, got %d", len(partial))
		}
	})

	t.Run("255-byte packet is complete when followed by 0", func(t *testing.T) {
		// Segment table [255, 0] = one complete 255-byte packet
		// A 0-length segment terminates the packet
		page := buildOggPage(t, 0, 0, []uint8{255, 0})
		r := bytes.NewReader(page)

		hdr, err := parseOggPageHeader(r)
		if err != nil {
			t.Fatalf("parseOggPageHeader: %v", err)
		}

		packets, partial, err := readOggPageBody(r, hdr)
		if err != nil {
			t.Fatalf("readOggPageBody: %v", err)
		}

		if len(packets) != 1 {
			t.Errorf("expected 1 complete packet, got %d", len(packets))
		}
		if len(packets[0]) != 255 {
			t.Errorf("expected packet length 255, got %d", len(packets[0]))
		}
		if partial != nil {
			t.Errorf("expected no partial, got %d bytes", len(partial))
		}
	})
}

// TestJoinMultiPagePackets tests joining packets across multiple pages.
func TestJoinMultiPagePackets(t *testing.T) {
	// Simulate a large packet spanning 3 pages:
	// Page 1: [255, 255] = 510 bytes, continues
	// Page 2: [255, 255] = 510 bytes, continues
	// Page 3: [100] = 100 bytes, complete
	// Total packet: 1120 bytes

	page1 := buildOggPageWithData(t, 0, 0, []uint8{255, 255}, bytes.Repeat([]byte{0x01}, 510))
	page2 := buildOggPageWithData(t, 0, 1, []uint8{255, 255}, bytes.Repeat([]byte{0x02}, 510))
	page3 := buildOggPageWithData(t, 0, 2, []uint8{100}, bytes.Repeat([]byte{0x03}, 100))

	// Read page 1
	r1 := bytes.NewReader(page1)
	hdr1, _ := parseOggPageHeader(r1)
	packets1, partial1, _ := readOggPageBody(r1, hdr1)

	if len(packets1) != 0 {
		t.Errorf("page1: expected 0 complete packets, got %d", len(packets1))
	}
	if len(partial1) != 510 {
		t.Errorf("page1: expected 510 byte partial, got %d", len(partial1))
	}

	// Read page 2, prepending partial from page 1
	r2 := bytes.NewReader(page2)
	hdr2, _ := parseOggPageHeader(r2)
	packets2, partial2, _ := readOggPageBody(r2, hdr2)

	// Join partial1 with first data from page2
	if len(packets2) != 0 {
		t.Errorf("page2: expected 0 complete packets, got %d", len(packets2))
	}
	partial1 = append(partial1, partial2...)
	if len(partial1) != 1020 {
		t.Errorf("after page2: expected 1020 bytes joined, got %d", len(partial1))
	}

	// Read page 3, complete the packet
	r3 := bytes.NewReader(page3)
	hdr3, _ := parseOggPageHeader(r3)
	packets3, partial3, _ := readOggPageBody(r3, hdr3)

	if len(packets3) != 1 {
		t.Errorf("page3: expected 1 complete packet, got %d", len(packets3))
	}
	if partial3 != nil {
		t.Errorf("page3: expected no partial, got %d bytes", len(partial3))
	}

	// Join all parts
	partial1 = append(partial1, packets3[0]...)
	if len(partial1) != 1120 {
		t.Errorf("expected final packet 1120 bytes, got %d", len(partial1))
	}

	// Verify data integrity
	for i := range 510 {
		if partial1[i] != 0x01 {
			t.Errorf("byte %d: expected 0x01, got 0x%02x", i, partial1[i])
			break
		}
	}
	for i := 510; i < 1020; i++ {
		if partial1[i] != 0x02 {
			t.Errorf("byte %d: expected 0x02, got 0x%02x", i, partial1[i])
			break
		}
	}
	for i := 1020; i < 1120; i++ {
		if partial1[i] != 0x03 {
			t.Errorf("byte %d: expected 0x03, got 0x%02x", i, partial1[i])
			break
		}
	}
}

// buildOggPage creates an Ogg page with the given segment table and dummy body data.
//
//nolint:unparam // serialNumber parameter exists for completeness even though tests only use value 0
func buildOggPage(t *testing.T, serialNumber, sequenceNum uint32, segments []uint8) []byte {
	t.Helper()
	var bodySize int
	for _, seg := range segments {
		bodySize += int(seg)
	}
	body := make([]byte, bodySize)
	// Fill with incrementing pattern for debugging
	for i := range body {
		body[i] = byte(i & 0xFF)
	}
	return buildOggPageWithData(t, serialNumber, sequenceNum, segments, body)
}

// buildOggPageWithData creates an Ogg page with specific body data.
func buildOggPageWithData(t *testing.T, serialNumber, sequenceNum uint32, segments []uint8, body []byte) []byte {
	t.Helper()

	// Verify body size matches segment table
	var expectedSize int
	for _, seg := range segments {
		expectedSize += int(seg)
	}
	if len(body) != expectedSize {
		t.Fatalf("body size %d doesn't match segment table total %d", len(body), expectedSize)
	}

	// Build header (27 bytes + segment table)
	headerSize := 27 + len(segments)
	page := make([]byte, headerSize+len(body))

	// Capture pattern "OggS"
	copy(page[0:4], "OggS")
	// Version (0)
	page[4] = 0
	// Header type flags (0 = normal page)
	page[5] = 0
	// Granule position (8 bytes, little-endian) - use 0
	binary.LittleEndian.PutUint64(page[6:14], 0)
	// Serial number (4 bytes)
	binary.LittleEndian.PutUint32(page[14:18], serialNumber)
	// Page sequence number (4 bytes)
	binary.LittleEndian.PutUint32(page[18:22], sequenceNum)
	// CRC checksum (4 bytes) - we skip validation so use 0
	binary.LittleEndian.PutUint32(page[22:26], 0)
	// Number of segments (test data always has < 255 segments)
	page[26] = uint8(len(segments)) //nolint:gosec // test data uses small segment counts
	// Segment table
	copy(page[27:27+len(segments)], segments)
	// Body
	copy(page[headerSize:], body)

	return page
}
