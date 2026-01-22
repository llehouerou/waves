package player

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/llehouerou/alac"
	"github.com/llehouerou/go-faad2"
	"github.com/llehouerou/go-m4a"
)

// m4aDecoder wraps go-m4a container reader with AAC or ALAC decoder.
type m4aDecoder struct {
	container  *m4a.Reader
	closer     io.Closer
	format     beep.Format
	codecType  m4a.CodecType
	err        error
	currentIdx int
	totalLen   int
	sampleSize int // bits per sample (16 or 24)
	channels   int

	// AAC decoder (when codec is AAC)
	aacDecoder *faad2.Decoder

	// ALAC decoder (when codec is ALAC)
	alacDecoder *alac.Alac

	// PCM buffer for partial reads (stored as float64 stereo frames)
	pcmBuffer [][2]float64
	pcmOffset int
}

// decodeM4A decodes an M4A/MP4 file, auto-detecting AAC or ALAC codec.
// Returns a beep.StreamSeekCloser, the audio format, and the codec type string.
func decodeM4A(rc io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, string, error) {
	container, err := m4a.Open(rc)
	if err != nil {
		return nil, beep.Format{}, "", err
	}

	codecType := container.Codec()
	sampleRate := container.SampleRate()
	channels := container.Channels()

	// Determine precision based on codec
	precision := 2 // 16-bit default
	if codecType == m4a.CodecALAC && container.SampleSize() == 24 {
		precision = 3 // 24-bit for ALAC
	}

	format := beep.Format{
		SampleRate:  beep.SampleRate(sampleRate),
		NumChannels: 2, // Always output stereo
		Precision:   precision,
	}

	d := &m4aDecoder{
		container:  container,
		closer:     rc,
		format:     format,
		codecType:  codecType,
		totalLen:   int(container.Duration().Seconds() * float64(sampleRate)),
		sampleSize: int(container.SampleSize()),
		channels:   int(channels),
	}

	// Initialize the appropriate decoder
	switch codecType {
	case m4a.CodecAAC:
		decoder, err := faad2.NewDecoder(context.Background())
		if err != nil {
			return nil, beep.Format{}, "", err
		}
		if err := decoder.Init(context.Background(), container.CodecConfig()); err != nil {
			decoder.Close(context.Background())
			return nil, beep.Format{}, "", err
		}
		d.aacDecoder = decoder

	case m4a.CodecALAC:
		cfg := alac.Config{
			SampleRate:  int(sampleRate),
			SampleSize:  int(container.SampleSize()),
			NumChannels: int(channels),
			FrameSize:   4096, // ALAC default
		}
		decoder, err := alac.NewWithConfig(cfg)
		if err != nil {
			return nil, beep.Format{}, "", err
		}
		d.alacDecoder = decoder

	case m4a.CodecUnknown:
		return nil, beep.Format{}, "", errors.New("unsupported codec in M4A container")
	}

	return d, format, codecType.String(), nil
}

// Stream reads audio samples into the provided buffer.
func (d *m4aDecoder) Stream(samples [][2]float64) (n int, ok bool) {
	if d.err != nil {
		return 0, false
	}

	for n < len(samples) {
		// First, drain any buffered samples
		if d.pcmOffset < len(d.pcmBuffer) {
			for n < len(samples) && d.pcmOffset < len(d.pcmBuffer) {
				samples[n] = d.pcmBuffer[d.pcmOffset]
				d.pcmOffset++
				n++
			}
			continue
		}

		// Check if we've read all samples
		if d.currentIdx >= d.container.SampleCount() {
			if n > 0 {
				return n, true
			}
			return 0, false
		}

		// Read next sample from container
		sampleData, err := d.container.ReadSample(d.currentIdx)
		if err != nil {
			d.err = err
			return n, n > 0
		}
		d.currentIdx++

		// Decode the sample and convert to float64 stereo frames
		switch d.codecType {
		case m4a.CodecAAC:
			pcm, err := d.aacDecoder.Decode(context.Background(), sampleData)
			if err != nil {
				d.err = err
				return n, n > 0
			}
			d.pcmBuffer = d.int16ToFloat64Stereo(pcm)

		case m4a.CodecALAC:
			rawPCM := d.alacDecoder.Decode(sampleData)
			d.pcmBuffer = d.alacBytesToFloat64Stereo(rawPCM)

		case m4a.CodecUnknown:
			d.err = errors.New("unsupported codec")
			return n, n > 0
		}

		d.pcmOffset = 0
	}

	return n, true
}

// int16ToFloat64Stereo converts int16 PCM samples to float64 stereo frames.
func (d *m4aDecoder) int16ToFloat64Stereo(pcm []int16) [][2]float64 {
	if d.channels == 2 {
		frames := make([][2]float64, len(pcm)/2)
		for i := range frames {
			frames[i][0] = float64(pcm[i*2]) / 32768.0
			frames[i][1] = float64(pcm[i*2+1]) / 32768.0
		}
		return frames
	}
	// Mono: duplicate to stereo
	frames := make([][2]float64, len(pcm))
	for i, sample := range pcm {
		v := float64(sample) / 32768.0
		frames[i][0] = v
		frames[i][1] = v
	}
	return frames
}

// alacBytesToFloat64Stereo converts ALAC raw PCM bytes to float64 stereo frames.
// Handles both 16-bit and 24-bit sample sizes.
func (d *m4aDecoder) alacBytesToFloat64Stereo(data []byte) [][2]float64 {
	if d.sampleSize == 24 {
		return d.alac24BitToFloat64Stereo(data)
	}
	return d.alac16BitToFloat64Stereo(data)
}

func (d *m4aDecoder) alac24BitToFloat64Stereo(data []byte) [][2]float64 {
	// 24-bit: 3 bytes per sample, 6 bytes per stereo frame
	bytesPerFrame := 3 * d.channels
	frameCount := len(data) / bytesPerFrame
	frames := make([][2]float64, frameCount)

	for i := range frameCount {
		offset := i * bytesPerFrame
		// 24-bit little-endian, sign-extend to int32
		left := int32(data[offset]) | int32(data[offset+1])<<8 | int32(data[offset+2])<<16
		if left&0x800000 != 0 {
			left |= ^0xFFFFFF // sign extend
		}

		right := left
		if d.channels == 2 {
			right = int32(data[offset+3]) | int32(data[offset+4])<<8 | int32(data[offset+5])<<16
			if right&0x800000 != 0 {
				right |= ^0xFFFFFF // sign extend
			}
		}

		frames[i][0] = float64(left) / 8388608.0 // 2^23
		frames[i][1] = float64(right) / 8388608.0
	}
	return frames
}

func (d *m4aDecoder) alac16BitToFloat64Stereo(data []byte) [][2]float64 {
	// 16-bit: 2 bytes per sample, 4 bytes per stereo frame
	bytesPerFrame := 2 * d.channels
	frameCount := len(data) / bytesPerFrame
	frames := make([][2]float64, frameCount)

	for i := range frameCount {
		offset := i * bytesPerFrame
		left := int16(data[offset]) | int16(data[offset+1])<<8

		right := left
		if d.channels == 2 {
			right = int16(data[offset+2]) | int16(data[offset+3])<<8
		}

		frames[i][0] = float64(left) / 32768.0
		frames[i][1] = float64(right) / 32768.0
	}
	return frames
}

// Err returns any error that occurred during streaming.
func (d *m4aDecoder) Err() error {
	return d.err
}

// Len returns the total number of samples (frames).
func (d *m4aDecoder) Len() int {
	return d.totalLen
}

// Position returns the current sample position.
func (d *m4aDecoder) Position() int {
	pos := d.container.SampleTime(d.currentIdx)
	sampleRate := d.container.SampleRate()
	return int(pos.Seconds() * float64(sampleRate))
}

// Seek seeks to the given sample position.
func (d *m4aDecoder) Seek(p int) error {
	if p < 0 {
		p = 0
	}
	if p > d.totalLen {
		p = d.totalLen
	}

	// Convert sample position to time
	sampleRate := d.container.SampleRate()
	pos := time.Duration(float64(p) / float64(sampleRate) * float64(time.Second))

	// Find the sample index for this position
	d.currentIdx = d.container.SeekToTime(pos)
	d.pcmBuffer = nil
	d.pcmOffset = 0
	d.err = nil

	return nil
}

// Close closes the decoder and underlying file.
func (d *m4aDecoder) Close() error {
	if d.aacDecoder != nil {
		d.aacDecoder.Close(context.Background())
	}
	// ALAC decoder doesn't need explicit close
	return d.closer.Close()
}
