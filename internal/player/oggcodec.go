package player

import (
	"encoding/binary"
	"errors"

	"github.com/jfreymuth/vorbis"
	"github.com/jj11hh/opus"
)

var (
	errUnknownOggCodec     = errors.New("ogg: unknown codec (not Opus or Vorbis)")
	errInvalidVorbisHeader = errors.New("vorbis: invalid identification header")
)

// OggCodec handles codec-specific initialization and decoding for Ogg streams.
type OggCodec interface {
	// SampleRate returns the audio sample rate.
	SampleRate() int

	// Channels returns the number of audio channels.
	Channels() int

	// PreSkip returns samples to skip at stream start (0 for Vorbis).
	PreSkip() int

	// GranuleToSamples converts granule position to sample count.
	GranuleToSamples(granule int64) int64

	// AddHeaderPacket adds a header packet for codecs that need multiple headers.
	// Returns true when all headers are received.
	// For Opus, this is a no-op (single header). For Vorbis, collects 3 headers.
	AddHeaderPacket(packet []byte) (complete bool, err error)

	// Decode decodes a packet into PCM samples.
	// Returns the number of samples per channel decoded.
	Decode(packet []byte, pcm []float32) (samplesPerChannel int, err error)

	// Reset resets decoder state (needed after seeking).
	Reset() error
}

// detectOggCodec detects the codec from the first Ogg packet and returns
// an initialized codec ready to receive further header packets.
func detectOggCodec(firstPacket []byte) (OggCodec, error) {
	// Check for Opus: starts with "OpusHead"
	if len(firstPacket) >= 8 && string(firstPacket[:8]) == "OpusHead" {
		return newOpusCodec(firstPacket)
	}

	// Check for Vorbis: starts with 0x01 + "vorbis"
	if len(firstPacket) >= 7 && firstPacket[0] == 0x01 && string(firstPacket[1:7]) == "vorbis" {
		return newVorbisCodec(firstPacket)
	}

	return nil, errUnknownOggCodec
}

// opusCodec implements OggCodec for Opus streams.
type opusCodec struct {
	decoder    *opus.Decoder
	channels   int
	preSkip    int
	sampleRate int // Original sample rate from header (informational only)
}

// newOpusCodec creates an opusCodec from an OpusHead packet.
func newOpusCodec(packet []byte) (*opusCodec, error) {
	if len(packet) < 19 {
		return nil, errInvalidOpusHead
	}

	// Check version (must be 1)
	if packet[8] != 1 {
		return nil, errUnsupportedOpus
	}

	channels := int(packet[9])

	decoder, err := opus.NewDecoder(opusSampleRate, channels)
	if err != nil {
		return nil, err
	}

	return &opusCodec{
		decoder:    decoder,
		channels:   channels,
		preSkip:    int(binary.LittleEndian.Uint16(packet[10:12])),
		sampleRate: int(binary.LittleEndian.Uint32(packet[12:16])),
	}, nil
}

// SampleRate returns 48000 (Opus always decodes to 48kHz).
func (c *opusCodec) SampleRate() int {
	return opusSampleRate // 48000
}

// Channels returns the number of audio channels.
func (c *opusCodec) Channels() int {
	return c.channels
}

// PreSkip returns samples to skip at stream start.
func (c *opusCodec) PreSkip() int {
	return c.preSkip
}

// GranuleToSamples converts granule position to sample count (subtracts pre-skip).
func (c *opusCodec) GranuleToSamples(granule int64) int64 {
	return granule - int64(c.preSkip)
}

// Decode decodes an Opus packet into PCM samples.
func (c *opusCodec) Decode(packet []byte, pcm []float32) (samplesPerChannel int, err error) {
	return c.decoder.DecodeFloat32(packet, pcm)
}

// Reset resets decoder state.
// Opus decoder recovers from packet loss automatically, so this is a no-op.
func (c *opusCodec) Reset() error {
	return nil
}

// AddHeaderPacket is a no-op for Opus (single header already parsed).
func (c *opusCodec) AddHeaderPacket(_ []byte) (bool, error) {
	return true, nil
}

// vorbisCodec implements OggCodec for Vorbis streams.
type vorbisCodec struct {
	decoder       *vorbis.Decoder
	channels      int
	sampleRate    int
	headerPackets [][]byte // collect headers before initializing decoder
}

// newVorbisCodec creates a vorbisCodec from a Vorbis identification header.
func newVorbisCodec(packet []byte) (*vorbisCodec, error) {
	// Vorbis identification header format:
	// [0]      = packet type (0x01)
	// [1:7]    = "vorbis"
	// [7:11]   = version (must be 0)
	// [11]     = channels
	// [12:16]  = sample rate (little-endian)
	if len(packet) < 16 {
		return nil, errInvalidVorbisHeader
	}

	// Check version (must be 0)
	version := binary.LittleEndian.Uint32(packet[7:11])
	if version != 0 {
		return nil, errInvalidVorbisHeader
	}

	// Store a copy of the identification header
	identHeader := make([]byte, len(packet))
	copy(identHeader, packet)

	return &vorbisCodec{
		channels:      int(packet[11]),
		sampleRate:    int(binary.LittleEndian.Uint32(packet[12:16])),
		headerPackets: [][]byte{identHeader},
	}, nil
}

// SampleRate returns the audio sample rate from the Vorbis header.
func (c *vorbisCodec) SampleRate() int {
	return c.sampleRate
}

// Channels returns the number of audio channels.
func (c *vorbisCodec) Channels() int {
	return c.channels
}

// PreSkip returns 0 (Vorbis has no pre-skip).
func (c *vorbisCodec) PreSkip() int {
	return 0
}

// GranuleToSamples converts granule position to sample count (direct mapping for Vorbis).
func (c *vorbisCodec) GranuleToSamples(granule int64) int64 {
	return granule
}

var (
	errVorbisDecoderNotInitialized = errors.New("vorbis: decoder not initialized (headers incomplete)")
	errVorbisBufferTooSmall        = errors.New("vorbis: output buffer too small")
)

// AddHeaderPacket adds a header packet for Vorbis.
// Vorbis requires 3 header packets: identification, comment, setup.
// Returns true when all headers are received and the decoder is initialized.
func (c *vorbisCodec) AddHeaderPacket(packet []byte) (bool, error) {
	// If decoder is already initialized, we're done
	if c.decoder != nil {
		return true, nil
	}

	// Store a copy of the header packet
	headerCopy := make([]byte, len(packet))
	copy(headerCopy, packet)
	c.headerPackets = append(c.headerPackets, headerCopy)

	// Vorbis has 3 header packets: identification, comment, setup
	// Once we have all 3, initialize the decoder
	if len(c.headerPackets) >= 3 {
		decoder := &vorbis.Decoder{}
		for _, hdr := range c.headerPackets {
			if err := decoder.ReadHeader(hdr); err != nil {
				return false, err
			}
		}
		c.decoder = decoder
		c.headerPackets = nil // free memory
		return true, nil
	}

	return false, nil
}

// Decode decodes a Vorbis packet into PCM samples.
func (c *vorbisCodec) Decode(packet []byte, pcm []float32) (samplesPerChannel int, err error) {
	if c.decoder == nil {
		return 0, errVorbisDecoderNotInitialized
	}
	// jfreymuth/vorbis Decode returns []float32 samples (interleaved)
	samples, err := c.decoder.Decode(packet)
	if err != nil {
		return 0, err
	}
	// Ensure output buffer is large enough
	if len(pcm) < len(samples) {
		return 0, errVorbisBufferTooSmall
	}
	// Copy to output buffer
	n := copy(pcm, samples)
	return n / c.channels, nil // return samples per channel
}

// Reset resets decoder state.
// For Vorbis, we need to clear any internal decoder state after seeking.
func (c *vorbisCodec) Reset() error {
	if c.decoder != nil {
		c.decoder.Clear()
	}
	return nil
}
