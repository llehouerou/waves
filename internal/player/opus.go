package player

import (
	"errors"
	"io"

	"github.com/gopxl/beep/v2"
	"github.com/jj11hh/opus"
)

const opusSampleRate = 48000

// opusDecoder wraps jj11hh/opus to implement beep.StreamSeekCloser.
type opusDecoder struct {
	ogg     *OggReader
	decoder *opus.Decoder
	closer  io.Closer

	// Streaming state
	currentPage *OggPage
	packetIdx   int
	pcmBuffer   []float32 // decoded samples from current packet
	pcmPos      int

	// Position tracking
	granulePos int64
	totalLen   int64

	err error
}

// decodeOpus creates a decoder for an Ogg/Opus stream.
func decodeOpus(rc io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error) {
	ogg, err := NewOggReader(rc)
	if err != nil {
		return nil, beep.Format{}, err
	}

	channels := ogg.Channels()

	// jj11hh/opus decoder - supports all Opus modes (SILK, CELT, Hybrid)
	decoder, err := opus.NewDecoder(opusSampleRate, channels)
	if err != nil {
		return nil, beep.Format{}, err
	}

	format := beep.Format{
		SampleRate:  opusSampleRate,
		NumChannels: channels,
		Precision:   2, // 16-bit equivalent
	}

	d := &opusDecoder{
		ogg:      ogg,
		decoder:  decoder,
		closer:   rc,
		totalLen: ogg.Duration(),
	}
	// Initialize buffer with capacity but trigger refill on first Stream call
	d.pcmBuffer = make([]float32, 5760*channels) // Max Opus frame size
	d.pcmPos = len(d.pcmBuffer)                  // Empty: pos >= len triggers refill

	return d, format, nil
}

// Stream reads audio samples into the provided buffer.
func (d *opusDecoder) Stream(samples [][2]float64) (n int, ok bool) {
	if d.err != nil {
		return 0, false
	}

	channels := d.ogg.Channels()

	for n < len(samples) {
		// If we have buffered PCM data, use it
		if d.pcmPos < len(d.pcmBuffer) {
			for n < len(samples) && d.pcmPos < len(d.pcmBuffer) {
				if channels == 2 {
					samples[n][0] = float64(d.pcmBuffer[d.pcmPos])
					samples[n][1] = float64(d.pcmBuffer[d.pcmPos+1])
					d.pcmPos += 2
				} else {
					// Mono: duplicate to both channels
					samples[n][0] = float64(d.pcmBuffer[d.pcmPos])
					samples[n][1] = float64(d.pcmBuffer[d.pcmPos])
					d.pcmPos++
				}
				n++
				d.granulePos++
			}
			continue
		}

		// Need more packets
		if d.currentPage == nil || d.packetIdx >= len(d.currentPage.Packets) {
			page, err := d.ogg.ReadPage()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return n, n > 0
				}
				d.err = err
				return n, n > 0
			}
			d.currentPage = page
			d.packetIdx = 0
		}

		// Decode next packet
		if d.packetIdx < len(d.currentPage.Packets) {
			packet := d.currentPage.Packets[d.packetIdx]
			d.packetIdx++

			// Decode Opus packet - jj11hh/opus returns (samplesPerChannel, error)
			samplesPerChannel, err := d.decoder.DecodeFloat32(packet, d.pcmBuffer[:cap(d.pcmBuffer)])
			if err != nil {
				// Skip invalid packets
				continue
			}
			// Total samples = samplesPerChannel * channels
			d.pcmBuffer = d.pcmBuffer[:samplesPerChannel*channels]
			d.pcmPos = 0
		}
	}

	return n, true
}

// Err returns any error that occurred during streaming.
func (d *opusDecoder) Err() error {
	return d.err
}

// Len returns the total number of samples.
func (d *opusDecoder) Len() int {
	return int(d.totalLen)
}

// Position returns the current sample position.
func (d *opusDecoder) Position() int {
	return int(d.granulePos)
}

// Seek seeks to the given sample position.
func (d *opusDecoder) Seek(p int) error {
	if p < 0 {
		p = 0
	}
	length := d.Len()
	if p > length {
		p = length
	}

	// Calculate pre-roll position (80ms = 3840 samples at 48kHz)
	preroll := max(p-3840, 0)

	// Seek Ogg container
	if err := d.ogg.SeekToGranule(int64(preroll)); err != nil {
		return err
	}

	// Reset decoder state
	d.currentPage = nil
	d.packetIdx = 0
	d.pcmBuffer = d.pcmBuffer[:cap(d.pcmBuffer)]
	d.pcmPos = len(d.pcmBuffer) // Empty buffer (pos >= len triggers refill)
	d.granulePos = int64(preroll)
	d.err = nil

	// Decode and discard pre-roll samples
	if preroll < p {
		if err := d.discardSamples(p - preroll); err != nil {
			return err
		}
	}

	d.granulePos = int64(p)
	return nil
}

// discardSamples decodes and discards the specified number of samples.
func (d *opusDecoder) discardSamples(count int) error {
	discard := make([][2]float64, 256)
	remaining := count

	for remaining > 0 {
		toRead := min(remaining, len(discard))
		n, ok := d.Stream(discard[:toRead])
		if !ok && n == 0 {
			break
		}
		remaining -= n
	}

	return d.err
}

// Close closes the decoder and underlying file.
func (d *opusDecoder) Close() error {
	return d.closer.Close()
}
