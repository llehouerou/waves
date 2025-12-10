package player

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/gopxl/beep/v2"
	"github.com/llehouerou/go-mp3"
)

// goMP3Decoder wraps llehouerou/go-mp3 to implement beep.StreamSeekCloser.
type goMP3Decoder struct {
	decoder *mp3.Decoder
	closer  io.Closer
	format  beep.Format
	err     error
	readBuf []byte // reusable buffer for reading
}

// decodeGoMP3 decodes an MP3 file using the llehouerou/go-mp3 library.
// Returns a beep.StreamSeekCloser and the audio format.
func decodeGoMP3(rc io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
	decoder, err := mp3.NewDecoder(rc)
	if err != nil {
		return nil, beep.Format{}, err
	}

	sampleRate := decoder.SampleRate()
	if sampleRate == 0 {
		return nil, beep.Format{}, errors.New("mp3: invalid sample rate")
	}

	format := beep.Format{
		SampleRate:  beep.SampleRate(sampleRate),
		NumChannels: 2, // go-mp3 always outputs stereo
		Precision:   2, // 16-bit
	}

	d := &goMP3Decoder{
		decoder: decoder,
		closer:  rc,
		format:  format,
		readBuf: make([]byte, 8192),
	}

	return d, format, nil
}

// Stream reads audio samples into the provided buffer.
func (d *goMP3Decoder) Stream(samples [][2]float64) (n int, ok bool) {
	if d.err != nil {
		return 0, false
	}

	// 4 bytes per sample (stereo 16-bit)
	bytesNeeded := len(samples) * 4
	if len(d.readBuf) < bytesNeeded {
		d.readBuf = make([]byte, bytesNeeded)
	}

	// Read from decoder
	bytesRead, err := io.ReadFull(d.decoder, d.readBuf[:bytesNeeded])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		d.err = err
		return 0, false
	}

	// Calculate samples read (4 bytes per sample)
	samplesRead := bytesRead / 4
	if samplesRead == 0 {
		return 0, false
	}

	// Convert to float64 stereo samples
	for i := 0; i < samplesRead && i < len(samples); i++ {
		offset := i * 4
		if offset+4 <= bytesRead {
			left := int16(binary.LittleEndian.Uint16(d.readBuf[offset:]))    //nolint:gosec // audio samples
			right := int16(binary.LittleEndian.Uint16(d.readBuf[offset+2:])) //nolint:gosec // audio samples
			samples[i][0] = float64(left) / 32768.0
			samples[i][1] = float64(right) / 32768.0
		}
		n++
	}

	return n, true
}

// Err returns any error that occurred during streaming.
func (d *goMP3Decoder) Err() error {
	return d.err
}

// Len returns the total number of samples.
func (d *goMP3Decoder) Len() int {
	count := d.decoder.SampleCount()
	if count < 0 {
		return 0
	}
	return int(count)
}

// Position returns the current sample position.
func (d *goMP3Decoder) Position() int {
	return int(d.decoder.SamplePosition())
}

// Seek seeks to the given sample position.
func (d *goMP3Decoder) Seek(p int) error {
	if p < 0 {
		p = 0
	}
	length := d.Len()
	if p > length {
		p = length
	}

	err := d.decoder.SeekToSample(int64(p))
	if err != nil {
		return err
	}
	d.err = nil
	return nil
}

// Close closes the decoder and underlying file.
func (d *goMP3Decoder) Close() error {
	return d.closer.Close()
}
