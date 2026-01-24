package player

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
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

func TestReadOggPageBody(t *testing.T) {
	// Segment table with 2 packets: 100 bytes and 50 bytes
	hdr := &oggPageHeader{
		NumSegments:  2,
		SegmentTable: []uint8{100, 50},
	}

	// Create body data
	body := make([]byte, 150)
	for i := range body {
		body[i] = byte(i % 256)
	}

	r := bytes.NewReader(body)
	packets, partial, err := readOggPageBody(r, hdr)
	if err != nil {
		t.Fatalf("readOggPageBody failed: %v", err)
	}

	if len(packets) != 2 {
		t.Fatalf("got %d packets, want 2", len(packets))
	}
	if partial != nil {
		t.Errorf("expected no partial, got %d bytes", len(partial))
	}
	if len(packets[0]) != 100 {
		t.Errorf("packet[0] len = %d, want 100", len(packets[0]))
	}
	if len(packets[1]) != 50 {
		t.Errorf("packet[1] len = %d, want 50", len(packets[1]))
	}
}

func TestReadOggPageBody_SpanningPacket(t *testing.T) {
	// A packet spanning multiple segments uses 255-byte segments
	// until a segment < 255 terminates it
	hdr := &oggPageHeader{
		NumSegments:  3,
		SegmentTable: []uint8{255, 255, 100}, // One packet of 610 bytes
	}

	body := make([]byte, 610)
	r := bytes.NewReader(body)
	packets, partial, err := readOggPageBody(r, hdr)
	if err != nil {
		t.Fatalf("readOggPageBody failed: %v", err)
	}

	if len(packets) != 1 {
		t.Fatalf("got %d packets, want 1", len(packets))
	}
	if len(packets[0]) != 610 {
		t.Errorf("packet len = %d, want 610", len(packets[0]))
	}
	if partial != nil {
		t.Errorf("expected no partial, got %d bytes", len(partial))
	}
}

// testOggFile holds test file data with metadata for OggReader setup.
type testOggFile struct {
	data       []byte
	dataStart  int64 // offset where audio pages begin
	sampleRate int
	preSkip    int
}

func createTestOpusFile(t *testing.T) testOggFile {
	t.Helper()
	var buf bytes.Buffer

	// Page 1: OpusHead
	opusHead := []byte{
		'O', 'p', 'u', 's', 'H', 'e', 'a', 'd',
		1, 2, 0x38, 0x01, 0x80, 0xBB, 0x00, 0x00, 0, 0, 0,
	}
	writeOggPage(&buf, 0, 0, 1, 0, [][]byte{opusHead})

	// Page 2: OpusTags (minimal)
	opusTags := []byte{
		'O', 'p', 'u', 's', 'T', 'a', 'g', 's',
		0, 0, 0, 0, // vendor string length = 0
		0, 0, 0, 0, // user comment list length = 0
	}
	writeOggPage(&buf, 0, 0, 1, 1, [][]byte{opusTags})

	// Record where audio pages start
	dataStart := int64(buf.Len())

	// Page 3: Audio data with granule position 48000 (1 second)
	audioData := make([]byte, 100)
	writeOggPage(&buf, 48000, 0, 1, 2, [][]byte{audioData})

	return testOggFile{
		data:       buf.Bytes(),
		dataStart:  dataStart,
		sampleRate: 48000,
		preSkip:    312,
	}
}

// newTestOggReader creates an OggReader from a testOggFile, properly initialized.
func newTestOggReader(t *testing.T, tf testOggFile) *OggReader {
	t.Helper()
	r := bytes.NewReader(tf.data)
	ogr, err := NewOggReader(r, tf.sampleRate, tf.preSkip)
	if err != nil {
		t.Fatalf("NewOggReader failed: %v", err)
	}
	ogr.SetDataStart(tf.dataStart)
	if err := ogr.ScanLastGranule(); err != nil {
		t.Fatalf("ScanLastGranule failed: %v", err)
	}
	// Seek to data start for reading
	if err := ogr.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	return ogr
}

// writeOggPage writes a minimal Ogg page to the buffer.
//
//nolint:unparam // serial parameter exists for correctness even though tests only use value 1
func writeOggPage(w *bytes.Buffer, granule int64, flags byte, serial, sequence uint32, packets [][]byte) {
	// Calculate total size for preallocation
	var totalSize int
	for _, pkt := range packets {
		totalSize += len(pkt)
	}

	// Calculate segment table
	var segments []byte
	bodyData := make([]byte, 0, totalSize)
	for _, pkt := range packets {
		remaining := len(pkt)
		for remaining >= 255 {
			segments = append(segments, 255)
			remaining -= 255
		}
		segments = append(segments, byte(remaining))
		bodyData = append(bodyData, pkt...)
	}

	// Write header
	w.WriteString("OggS")
	w.WriteByte(0) // version
	w.WriteByte(flags)
	_ = binary.Write(w, binary.LittleEndian, granule)
	_ = binary.Write(w, binary.LittleEndian, serial)
	_ = binary.Write(w, binary.LittleEndian, sequence)
	_ = binary.Write(w, binary.LittleEndian, uint32(0)) // checksum placeholder
	w.WriteByte(byte(len(segments)))
	w.Write(segments)
	w.Write(bodyData)
}

func TestNewOggReader(t *testing.T) {
	tf := createTestOpusFile(t)
	ogr := newTestOggReader(t, tf)

	if ogr.SampleRate() != 48000 {
		t.Errorf("SampleRate = %d, want 48000", ogr.SampleRate())
	}
	if ogr.PreSkip() != 312 {
		t.Errorf("PreSkip = %d, want 312", ogr.PreSkip())
	}
}

func TestOggReader_ReadPage(t *testing.T) {
	tf := createTestOpusFile(t)
	ogr := newTestOggReader(t, tf)

	// Read first audio page
	page, err := ogr.ReadPage()
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	if page.GranulePos != 48000 {
		t.Errorf("GranulePos = %d, want 48000", page.GranulePos)
	}
	if len(page.Packets) != 1 {
		t.Errorf("Packets count = %d, want 1", len(page.Packets))
	}
}

func TestOggReader_ReadPage_EOF(t *testing.T) {
	tf := createTestOpusFile(t)
	ogr := newTestOggReader(t, tf)

	// Read first page
	_, err := ogr.ReadPage()
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	// Second read should return EOF
	_, err = ogr.ReadPage()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

//nolint:unparam // preSkip parameter allows testing different Opus configurations
func createTestOpusFileWithDuration(t *testing.T, totalSamples int64, preSkip int) testOggFile {
	t.Helper()
	var buf bytes.Buffer

	// Page 1: OpusHead
	opusHead := []byte{
		'O', 'p', 'u', 's', 'H', 'e', 'a', 'd',
		1, 2, 0x38, 0x01, 0x80, 0xBB, 0x00, 0x00, 0, 0, 0,
	}
	writeOggPage(&buf, 0, 0x02, 1, 0, [][]byte{opusHead}) // BOS flag

	// Page 2: OpusTags
	opusTags := []byte{
		'O', 'p', 'u', 's', 'T', 'a', 'g', 's',
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	writeOggPage(&buf, 0, 0, 1, 1, [][]byte{opusTags})

	// Record where audio pages start
	dataStart := int64(buf.Len())

	// Page 3: Audio data with final granule position
	audioData := make([]byte, 100)
	writeOggPage(&buf, totalSamples, 0x04, 1, 2, [][]byte{audioData}) // EOS flag

	return testOggFile{
		data:       buf.Bytes(),
		dataStart:  dataStart,
		sampleRate: 48000,
		preSkip:    preSkip,
	}
}

func TestOggReader_Duration(t *testing.T) {
	// Create file with 5 seconds of audio (240000 samples at 48kHz)
	tf := createTestOpusFileWithDuration(t, 240000+312, 312) // +312 for pre-skip
	ogr := newTestOggReader(t, tf)

	duration := ogr.Duration()
	// Duration should be total samples minus pre-skip
	expected := int64(240000)
	if duration != expected {
		t.Errorf("Duration = %d, want %d", duration, expected)
	}
}

func createTestOpusFileMultiPage(t *testing.T) testOggFile {
	t.Helper()
	var buf bytes.Buffer

	// Page 1: OpusHead
	opusHead := []byte{
		'O', 'p', 'u', 's', 'H', 'e', 'a', 'd',
		1, 2, 0, 0, 0x80, 0xBB, 0x00, 0x00, 0, 0, 0, // pre-skip = 0 for simplicity
	}
	writeOggPage(&buf, 0, 0x02, 1, 0, [][]byte{opusHead})

	// Page 2: OpusTags
	opusTags := []byte{'O', 'p', 'u', 's', 'T', 'a', 'g', 's', 0, 0, 0, 0, 0, 0, 0, 0}
	writeOggPage(&buf, 0, 0, 1, 1, [][]byte{opusTags})

	// Record where audio pages start
	dataStart := int64(buf.Len())

	// Pages 3-7: Audio data at 1-second intervals
	for i := int64(1); i <= 5; i++ {
		audioData := make([]byte, 500) // Larger data to ensure pages are spread out
		granule := i * 48000
		flags := byte(0)
		if i == 5 {
			flags = 0x04 // EOS
		}
		writeOggPage(&buf, granule, flags, 1, uint32(i+1), [][]byte{audioData}) //nolint:gosec // test data uses small values
	}

	return testOggFile{
		data:       buf.Bytes(),
		dataStart:  dataStart,
		sampleRate: 48000,
		preSkip:    0, // pre-skip = 0 for simplicity
	}
}

func TestOggReader_SeekToGranule(t *testing.T) {
	tf := createTestOpusFileMultiPage(t)
	ogr := newTestOggReader(t, tf)

	tests := []struct {
		name        string
		target      int64
		wantGranule int64 // Expected granule of page we land on (≤ target)
	}{
		{"seek to start", 0, 48000},            // First audio page
		{"seek to 2 seconds", 96000, 96000},    // Exactly on page boundary
		{"seek to 2.5 seconds", 120000, 96000}, // Between pages, land on earlier
		{"seek to end", 240000, 240000},        // Last page
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ogr.SeekToGranule(tt.target); err != nil {
				t.Fatalf("SeekToGranule(%d) failed: %v", tt.target, err)
			}

			page, err := ogr.ReadPage()
			if err != nil {
				t.Fatalf("ReadPage after seek failed: %v", err)
			}

			if page.GranulePos > tt.target && tt.target > 0 {
				t.Errorf("Landed on page with granule %d, which is after target %d",
					page.GranulePos, tt.target)
			}
		})
	}
}

func TestOggReader_SeekToGranule_Reset(t *testing.T) {
	tf := createTestOpusFileMultiPage(t)
	ogr := newTestOggReader(t, tf)

	// Seek to middle
	if err := ogr.SeekToGranule(144000); err != nil {
		t.Fatalf("SeekToGranule failed: %v", err)
	}

	// Seek to start
	if err := ogr.SeekToGranule(0); err != nil {
		t.Fatalf("SeekToGranule(0) failed: %v", err)
	}

	page, err := ogr.ReadPage()
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	// Should be first audio page
	if page.GranulePos != 48000 {
		t.Errorf("After seek to 0, got granule %d, want 48000", page.GranulePos)
	}
}

func TestIsOpusCodec_Opus(t *testing.T) {
	// Create a temp file with valid Opus data
	tf := createTestOpusFile(t)
	tmpfile, err := os.CreateTemp("", "test*.opus")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(tf.data); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpfile.Close()

	if !IsOpusCodec(tmpfile.Name()) {
		t.Error("IsOpusCodec returned false for valid Opus file")
	}
}

func TestIsOpusCodec_Vorbis(t *testing.T) {
	// Create a temp file with Vorbis identification header
	var buf bytes.Buffer

	// Vorbis identification packet: \x01vorbis + version + channels + sample rate + etc
	vorbisIdent := []byte{
		0x01, 'v', 'o', 'r', 'b', 'i', 's', // packet type + "vorbis"
		0, 0, 0, 0, // version (0)
		2,                // channels
		0x80, 0xBB, 0, 0, // sample rate 48000
		0, 0, 0, 0, // bitrate max
		0, 0, 0, 0, // bitrate nominal
		0, 0, 0, 0, // bitrate min
		0xB8, // blocksize
		1,    // framing
	}
	writeOggPage(&buf, 0, 0x02, 1, 0, [][]byte{vorbisIdent}) // BOS flag

	tmpfile, err := os.CreateTemp("", "test*.ogg")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(buf.Bytes()); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpfile.Close()

	if IsOpusCodec(tmpfile.Name()) {
		t.Error("IsOpusCodec returned true for Vorbis file")
	}
}

func TestIsOpusCodec_InvalidFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test*.ogg")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString("not an ogg file"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpfile.Close()

	if IsOpusCodec(tmpfile.Name()) {
		t.Error("IsOpusCodec returned true for invalid file")
	}
}

func TestIsOpusCodec_NonExistent(t *testing.T) {
	if IsOpusCodec("/nonexistent/path/file.ogg") {
		t.Error("IsOpusCodec returned true for non-existent file")
	}
}
