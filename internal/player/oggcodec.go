package player

import (
	"encoding/binary"
	"errors"
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

	return &opusCodec{
		channels:   int(packet[9]),
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
// TODO: Implement actual decoding in a later task.
func (c *opusCodec) Decode(_ []byte, _ []float32) (samplesPerChannel int, err error) {
	return 0, nil
}

// Reset resets decoder state.
// TODO: Implement actual reset in a later task.
func (c *opusCodec) Reset() error {
	return nil
}

// vorbisCodec implements OggCodec for Vorbis streams.
type vorbisCodec struct {
	channels   int
	sampleRate int
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

	return &vorbisCodec{
		channels:   int(packet[11]),
		sampleRate: int(binary.LittleEndian.Uint32(packet[12:16])),
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

// Decode decodes a Vorbis packet into PCM samples.
// TODO: Implement actual decoding in a later task.
func (c *vorbisCodec) Decode(_ []byte, _ []float32) (samplesPerChannel int, err error) {
	return 0, nil
}

// Reset resets decoder state.
// TODO: Implement actual reset in a later task.
func (c *vorbisCodec) Reset() error {
	return nil
}
