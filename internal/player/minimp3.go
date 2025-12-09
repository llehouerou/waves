package player

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/gopxl/beep/v2"
	"github.com/tosone/minimp3"
)

// minimp3Decoder wraps tosone/minimp3 to implement beep.StreamSeekCloser.
type minimp3Decoder struct {
	decoder  *minimp3.Decoder
	file     io.ReadSeekCloser
	format   beep.Format
	position int
	length   int
	bitrate  int
	err      error
	readBuf  []byte // reusable buffer for reading
}

// decodeMiniMP3 decodes an MP3 file using the minimp3 library.
// Returns a beep.StreamSeekCloser and the audio format.
func decodeMiniMP3(file io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error) {
	// Get file size BEFORE creating decoder (decoder runs in background goroutines)
	fileSize, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, beep.Format{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, beep.Format{}, err
	}

	decoder, err := minimp3.NewDecoder(file)
	if err != nil {
		return nil, beep.Format{}, err
	}

	// Wait for decoder to parse header
	<-decoder.Started()

	sampleRate := decoder.SampleRate
	channels := decoder.Channels
	bitrate := decoder.Kbps

	if sampleRate == 0 || channels == 0 {
		decoder.Close()
		return nil, beep.Format{}, io.ErrUnexpectedEOF
	}

	if bitrate == 0 {
		bitrate = 128 // fallback
	}

	// Calculate duration and convert to samples
	durationSeconds := float64(fileSize) * 8 / float64(bitrate) / 1000
	length := int(durationSeconds * float64(sampleRate))

	format := beep.Format{
		SampleRate:  beep.SampleRate(sampleRate),
		NumChannels: 2, // We always output stereo
		Precision:   2, // 16-bit
	}

	d := &minimp3Decoder{
		decoder:  decoder,
		file:     file,
		format:   format,
		position: 0,
		length:   length,
		bitrate:  bitrate,
		readBuf:  make([]byte, 8192),
	}

	return d, format, nil
}

// Stream reads audio samples into the provided buffer.
func (d *minimp3Decoder) Stream(samples [][2]float64) (n int, ok bool) {
	if d.err != nil {
		return 0, false
	}

	channels := d.decoder.Channels
	if channels == 0 {
		channels = 2 // fallback
	}

	// Bytes per input sample
	bytesPerInputSample := 2 * channels

	// Ensure read buffer is large enough (account for mono->stereo conversion)
	inputBytesNeeded := len(samples) * bytesPerInputSample
	if len(d.readBuf) < inputBytesNeeded {
		d.readBuf = make([]byte, inputBytesNeeded)
	}

	// Read from decoder
	bytesRead, err := d.decoder.Read(d.readBuf[:inputBytesNeeded])
	if err != nil && !errors.Is(err, io.EOF) {
		d.err = err
		return 0, false
	}

	// Calculate samples read
	samplesRead := bytesRead / bytesPerInputSample
	if samplesRead == 0 {
		return 0, false
	}

	// Convert to float64 stereo samples
	for i := 0; i < samplesRead && i < len(samples); i++ {
		if channels == 2 {
			// Stereo: 4 bytes per sample (L16, R16)
			offset := i * 4
			if offset+4 <= bytesRead {
				left := int16(binary.LittleEndian.Uint16(d.readBuf[offset:]))    //nolint:gosec // audio samples
				right := int16(binary.LittleEndian.Uint16(d.readBuf[offset+2:])) //nolint:gosec // audio samples
				samples[i][0] = float64(left) / 32768.0
				samples[i][1] = float64(right) / 32768.0
			}
		} else {
			// Mono: 2 bytes per sample, duplicate to both channels
			offset := i * 2
			if offset+2 <= bytesRead {
				mono := int16(binary.LittleEndian.Uint16(d.readBuf[offset:])) //nolint:gosec // audio samples
				val := float64(mono) / 32768.0
				samples[i][0] = val
				samples[i][1] = val
			}
		}
		n++
	}

	d.position += n
	return n, true
}

// Err returns any error that occurred during streaming.
func (d *minimp3Decoder) Err() error {
	return d.err
}

// Len returns the total number of samples.
func (d *minimp3Decoder) Len() int {
	return d.length
}

// Position returns the current sample position.
func (d *minimp3Decoder) Position() int {
	return d.position
}

// Seek seeks to the given sample position.
// Since tosone/minimp3 doesn't support seeking directly, we recreate the decoder
// and skip to the desired position.
func (d *minimp3Decoder) Seek(p int) error {
	if p < 0 {
		p = 0
	}
	if p > d.length {
		p = d.length
	}

	// Calculate byte offset from sample position
	sampleRate := int(d.format.SampleRate)
	if sampleRate == 0 {
		return nil
	}

	// byte_offset = (p / sampleRate) * (bitrate * 1000 / 8)
	// = p * bitrate * 125 / sampleRate
	byteOffset := int64(p) * int64(d.bitrate) * 125 / int64(sampleRate)

	// Close old decoder
	d.decoder.Close()

	// Seek in the underlying file
	if _, err := d.file.Seek(byteOffset, io.SeekStart); err != nil {
		return err
	}

	// Create new decoder
	decoder, err := minimp3.NewDecoder(d.file)
	if err != nil {
		return err
	}

	// Wait for decoder to initialize
	<-decoder.Started()

	d.decoder = decoder
	d.position = p
	d.err = nil
	return nil
}

// Close closes the decoder and underlying file.
func (d *minimp3Decoder) Close() error {
	d.decoder.Close()
	return d.file.Close()
}
