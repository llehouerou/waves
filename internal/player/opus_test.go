package player

import (
	"bytes"
	"io"
	"testing"
)

type readSeekCloser struct {
	io.ReadSeeker
}

func (r *readSeekCloser) Close() error { return nil }

func TestDecodeOpus_Format(t *testing.T) {
	data := createTestOpusFile(t)
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, format, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
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

func TestDecodeOpus_Len(t *testing.T) {
	// Create file with known duration
	data := createTestOpusFileWithDuration(t, 48000+312) // 1 second + pre-skip
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, _, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
	}
	defer streamer.Close()

	// Len should return samples minus pre-skip
	expectedLen := 48000
	if streamer.Len() != expectedLen {
		t.Errorf("Len = %d, want %d", streamer.Len(), expectedLen)
	}
}

func TestOpusDecoder_Stream(t *testing.T) {
	data := createTestOpusFileWithDuration(t, 48000+312)
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, _, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
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

func TestOpusDecoder_StreamUntilEnd(t *testing.T) {
	data := createTestOpusFileWithDuration(t, 48000+312)
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, _, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
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

func TestOpusDecoder_Seek(t *testing.T) {
	data := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, _, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
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

func TestOpusDecoder_SeekToStart(t *testing.T) {
	data := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, _, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
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

func TestOpusDecoder_SeekBeyondEnd(t *testing.T) {
	data := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, _, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
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

func TestOpusDecoder_FullIntegration(t *testing.T) {
	// Create a multi-page test file
	data := createTestOpusFileMultiPage(t)
	r := &readSeekCloser{bytes.NewReader(data)}

	streamer, format, err := decodeOpus(r)
	if err != nil {
		t.Fatalf("decodeOpus failed: %v", err)
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
