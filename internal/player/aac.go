package player

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/llehouerou/go-faad2"
)

// aacDecoder wraps go-faad2 M4AReader to implement beep.StreamSeekCloser.
type aacDecoder struct {
	reader   *faad2.M4AReader
	closer   io.Closer
	format   beep.Format
	err      error
	readBuf  []int16 // reusable buffer for reading
	totalLen int     // total samples (cached)
}

// decodeAAC decodes an M4A/MP4 file using the go-faad2 library.
// Returns a beep.StreamSeekCloser and the audio format.
func decodeAAC(rc io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error) {
	reader, err := faad2.OpenM4A(context.Background(), rc)
	if err != nil {
		return nil, beep.Format{}, err
	}

	sampleRate := reader.SampleRate()

	format := beep.Format{
		SampleRate:  beep.SampleRate(sampleRate),
		NumChannels: 2, // Always output stereo (we'll duplicate mono if needed)
		Precision:   2, // 16-bit
	}

	// Calculate total samples from duration
	duration := reader.Duration()
	totalLen := int(duration.Seconds() * float64(sampleRate))

	d := &aacDecoder{
		reader:   reader,
		closer:   rc,
		format:   format,
		readBuf:  make([]int16, 8192),
		totalLen: totalLen,
	}

	return d, format, nil
}

// Stream reads audio samples into the provided buffer.
func (d *aacDecoder) Stream(samples [][2]float64) (n int, ok bool) {
	if d.err != nil {
		return 0, false
	}

	channels := int(d.reader.Channels())

	// Calculate how many int16 samples we need
	// For stereo: len(samples) * 2
	// For mono: len(samples) (we'll duplicate to stereo)
	samplesNeeded := len(samples) * channels
	if len(d.readBuf) < samplesNeeded {
		d.readBuf = make([]int16, samplesNeeded)
	}

	// Read from M4AReader
	samplesRead, err := d.reader.Read(context.Background(), d.readBuf[:samplesNeeded])
	if err != nil && !errors.Is(err, io.EOF) {
		d.err = err
		return 0, false
	}

	if samplesRead == 0 {
		return 0, false
	}

	// Convert to float64 stereo samples
	if channels == 2 {
		// Stereo input
		framesRead := samplesRead / 2
		for i := 0; i < framesRead && i < len(samples); i++ {
			left := d.readBuf[i*2]
			right := d.readBuf[i*2+1]
			samples[i][0] = float64(left) / 32768.0
			samples[i][1] = float64(right) / 32768.0
			n++
		}
	} else {
		// Mono input: duplicate to both channels
		for i := 0; i < samplesRead && i < len(samples); i++ {
			sample := float64(d.readBuf[i]) / 32768.0
			samples[i][0] = sample
			samples[i][1] = sample
			n++
		}
	}

	return n, true
}

// Err returns any error that occurred during streaming.
func (d *aacDecoder) Err() error {
	return d.err
}

// Len returns the total number of samples (frames).
func (d *aacDecoder) Len() int {
	return d.totalLen
}

// Position returns the current sample position.
func (d *aacDecoder) Position() int {
	pos := d.reader.Position()
	sampleRate := d.reader.SampleRate()
	return int(pos.Seconds() * float64(sampleRate))
}

// Seek seeks to the given sample position.
func (d *aacDecoder) Seek(p int) error {
	if p < 0 {
		p = 0
	}
	length := d.Len()
	if p > length {
		p = length
	}

	// Convert sample position to time.Duration
	sampleRate := d.reader.SampleRate()
	pos := time.Duration(float64(p) / float64(sampleRate) * float64(time.Second))

	err := d.reader.Seek(pos)
	if err != nil {
		return err
	}
	d.err = nil
	return nil
}

// Close closes the decoder and underlying file.
func (d *aacDecoder) Close() error {
	if err := d.reader.Close(context.Background()); err != nil {
		return err
	}
	return d.closer.Close()
}
