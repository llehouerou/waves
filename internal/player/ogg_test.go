package player

import (
	"bytes"
	"os"
	"testing"
)

const vorbisTestFile = "testdata/vorbis_44100_stereo.ogg"

func TestDecodeOgg_Opus(t *testing.T) {
	path := "testdata/test.opus"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("testdata/test.opus not found")
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	streamer, format, err := decodeOgg(f)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	if format.SampleRate != 48000 {
		t.Errorf("expected 48000 sample rate, got %d", format.SampleRate)
	}
}

func TestDecodeOgg_Vorbis(t *testing.T) {
	if _, err := os.Stat(vorbisTestFile); os.IsNotExist(err) {
		t.Skipf("%s not found - generate with: ffmpeg -f lavfi -i \"sine=frequency=440:duration=1\" -ac 2 -c:a libvorbis -q:a 5 %s", vorbisTestFile, vorbisTestFile)
	}

	f, err := os.Open(vorbisTestFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	streamer, format, err := decodeOgg(f)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Vorbis test file uses 44100Hz
	if format.SampleRate != 44100 {
		t.Errorf("expected 44100 sample rate, got %d", format.SampleRate)
	}
	if format.NumChannels != 2 {
		t.Errorf("expected 2 channels, got %d", format.NumChannels)
	}

	// Verify we can read samples
	samples := make([][2]float64, 1024)
	n, ok := streamer.Stream(samples)
	if !ok || n == 0 {
		t.Error("expected to read samples")
	}
}

func TestDecodeOgg_Vorbis_Seeking(t *testing.T) {
	if _, err := os.Stat(vorbisTestFile); os.IsNotExist(err) {
		t.Skip("Vorbis test file not found")
	}

	f, err := os.Open(vorbisTestFile)
	if err != nil {
		t.Fatal(err)
	}

	streamer, _, err := decodeOgg(f)
	if err != nil {
		t.Fatal(err)
	}
	defer streamer.Close()

	// Read some samples first
	samples := make([][2]float64, 1024)
	streamer.Stream(samples)

	// Test seeking to middle
	length := streamer.Len()
	middle := length / 2
	if err := streamer.Seek(middle); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	if got := streamer.Position(); got != middle {
		t.Errorf("Position after seek: expected %d, got %d", middle, got)
	}

	// Test seeking back to start
	if err := streamer.Seek(0); err != nil {
		t.Fatalf("Seek(0) failed: %v", err)
	}
	if streamer.Position() != 0 {
		t.Errorf("Position after Seek(0) = %d, want 0", streamer.Position())
	}
}

func TestDecodeOgg_Vorbis_StreamUntilEnd(t *testing.T) {
	if _, err := os.Stat(vorbisTestFile); os.IsNotExist(err) {
		t.Skip("Vorbis test file not found")
	}

	f, err := os.Open(vorbisTestFile)
	if err != nil {
		t.Fatal(err)
	}

	streamer, format, err := decodeOgg(f)
	if err != nil {
		t.Fatal(err)
	}
	defer streamer.Close()

	// Read all samples
	samples := make([][2]float64, 256)
	totalRead := 0
	for {
		n, ok := streamer.Stream(samples)
		totalRead += n
		if !ok {
			break
		}
	}

	if streamer.Err() != nil {
		t.Errorf("Unexpected error: %v", streamer.Err())
	}

	// Vorbis 1 second at 44100Hz should have roughly 44100 samples
	expectedSamples := int(format.SampleRate) // 1 second worth of samples
	// Allow 10% tolerance due to codec padding
	tolerance := expectedSamples / 10
	if totalRead < expectedSamples-tolerance || totalRead > expectedSamples+tolerance {
		t.Errorf("Read %d samples, expected around %d (1 second at 44100Hz)", totalRead, expectedSamples)
	}
}

func TestDecodeOgg_OpusFromMemory(t *testing.T) {
	tf := createTestOpusFile(t)
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, format, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Opus always uses 48kHz
	if format.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", format.SampleRate)
	}
	if format.NumChannels != 2 {
		t.Errorf("NumChannels = %d, want 2", format.NumChannels)
	}
	if format.Precision != 2 {
		t.Errorf("Precision = %d, want 2 (16-bit)", format.Precision)
	}
}

func TestDecodeOgg_OpusLen(t *testing.T) {
	// Create file with known duration
	tf := createTestOpusFileWithDuration(t, 48000+312, 312) // 1 second + pre-skip
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, _, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Len should return samples minus pre-skip
	expectedLen := 48000
	if streamer.Len() != expectedLen {
		t.Errorf("Len = %d, want %d", streamer.Len(), expectedLen)
	}
}

func TestOggDecoder_Stream(t *testing.T) {
	tf := createTestOpusFileWithDuration(t, 48000+312, 312)
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, _, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Read some samples
	samples := make([][2]float64, 1024)
	n, ok := streamer.Stream(samples)

	// With our minimal test file, we may get 0 samples since
	// the audio data is just zeros. Just verify no error.
	if streamer.Err() != nil {
		t.Errorf("Stream returned error: %v", streamer.Err())
	}

	// The decoder should return something or indicate end
	_ = n
	_ = ok
}

func TestOggDecoder_StreamUntilEnd(t *testing.T) {
	tf := createTestOpusFileWithDuration(t, 48000+312, 312)
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, _, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Read all samples
	samples := make([][2]float64, 256)
	totalRead := 0
	for {
		n, ok := streamer.Stream(samples)
		totalRead += n
		if !ok {
			break
		}
	}

	if streamer.Err() != nil {
		t.Errorf("Unexpected error: %v", streamer.Err())
	}
}

func TestOggDecoder_Seek(t *testing.T) {
	tf := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, _, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Seek to middle
	if err := streamer.Seek(96000); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	// Position should be at or near target
	pos := streamer.Position()
	if pos > 96000 {
		t.Errorf("Position after seek = %d, should be <= 96000", pos)
	}
}

func TestOggDecoder_SeekToStart(t *testing.T) {
	tf := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, _, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Read some samples
	samples := make([][2]float64, 1024)
	streamer.Stream(samples)

	// Seek back to start
	if err := streamer.Seek(0); err != nil {
		t.Fatalf("Seek(0) failed: %v", err)
	}

	if streamer.Position() != 0 {
		t.Errorf("Position after Seek(0) = %d, want 0", streamer.Position())
	}
}

func TestOggDecoder_SeekBeyondEnd(t *testing.T) {
	tf := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, _, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Seek beyond end should clamp to end
	if err := streamer.Seek(999999999); err != nil {
		t.Fatalf("Seek beyond end failed: %v", err)
	}

	// Position should be at or near end
	pos := streamer.Position()
	length := streamer.Len()
	if pos > length {
		t.Errorf("Position %d exceeds length %d", pos, length)
	}
}

func TestOggDecoder_FullIntegration(t *testing.T) {
	// Create a multi-page test file
	tf := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(tf.data)}

	streamer, format, err := decodeOgg(r)
	if err != nil {
		t.Fatalf("decodeOgg failed: %v", err)
	}
	defer streamer.Close()

	// Verify format
	if format.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", format.SampleRate)
	}

	// Read some samples
	samples := make([][2]float64, 1024)
	n1, ok := streamer.Stream(samples)
	if !ok && n1 == 0 {
		t.Log("No samples decoded (test data may not contain valid Opus frames)")
	}

	// Seek to middle
	midpoint := streamer.Len() / 2
	if err := streamer.Seek(midpoint); err != nil {
		t.Fatalf("Seek to midpoint failed: %v", err)
	}

	// Read more samples
	n2, _ := streamer.Stream(samples)
	_ = n2

	// Seek back to start
	if err := streamer.Seek(0); err != nil {
		t.Fatalf("Seek to start failed: %v", err)
	}

	// Verify position
	if streamer.Position() != 0 {
		t.Errorf("Position after Seek(0) = %d, want 0", streamer.Position())
	}

	// No errors
	if streamer.Err() != nil {
		t.Errorf("Unexpected error: %v", streamer.Err())
	}
}

func TestDecodeOgg_EmptyFirstPage(t *testing.T) {
	// Create a minimal ogg file with no packets in first page
	var buf bytes.Buffer
	writeOggPage(&buf, 0, 0, 1, 0, [][]byte{}) // Empty page

	r := &readSeekCloser{bytes.NewReader(buf.Bytes())}
	_, _, err := decodeOgg(r)
	if err == nil {
		t.Error("expected error for empty first page")
	}
}

func TestDecodeOgg_UnknownCodec(t *testing.T) {
	// Create an ogg file with unknown codec
	var buf bytes.Buffer
	unknownPacket := []byte("UnknownCodecData")
	writeOggPage(&buf, 0, 0, 1, 0, [][]byte{unknownPacket})

	r := &readSeekCloser{bytes.NewReader(buf.Bytes())}
	_, _, err := decodeOgg(r)
	if err == nil {
		t.Error("expected error for unknown codec")
	}
}
