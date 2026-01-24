package player

import (
	"testing"
)

func TestDetectCodec_Opus(t *testing.T) {
	// OpusHead packet (minimal valid header - 19 bytes minimum)
	// Format: "OpusHead" (8) + version (1) + channels (1) + pre-skip (2) + sample rate (4) + gain (2) + mapping (1)
	packet := []byte{
		'O', 'p', 'u', 's', 'H', 'e', 'a', 'd', // magic
		0x01,       // version (must be 1)
		0x02,       // 2 channels
		0x00, 0x00, // pre-skip (little-endian)
		0x80, 0xBB, 0x00, 0x00, // sample rate 48000 (little-endian)
		0x00, 0x00, // output gain
		0x00, // channel mapping family
	}

	codec, err := detectOggCodec(packet)
	if err != nil {
		t.Fatalf("detectOggCodec failed: %v", err)
	}
	opusC, ok := codec.(*opusCodec)
	if !ok {
		t.Fatalf("expected *opusCodec, got %T", codec)
	}

	// Verify parsed values
	if opusC.Channels() != 2 {
		t.Errorf("expected 2 channels, got %d", opusC.Channels())
	}
	if opusC.PreSkip() != 0 {
		t.Errorf("expected pre-skip 0, got %d", opusC.PreSkip())
	}
}

func TestDetectCodec_Opus_InvalidVersion(t *testing.T) {
	// OpusHead packet with invalid version (must be 1, but we use 2)
	packet := []byte{
		'O', 'p', 'u', 's', 'H', 'e', 'a', 'd', // magic
		0x02,       // version 2 (invalid, must be 1)
		0x02,       // 2 channels
		0x00, 0x00, // pre-skip (little-endian)
		0x80, 0xBB, 0x00, 0x00, // sample rate 48000 (little-endian)
		0x00, 0x00, // output gain
		0x00, // channel mapping family
	}

	_, err := detectOggCodec(packet)
	if err == nil {
		t.Error("expected error for invalid Opus version")
	}
}

func TestDetectCodec_Opus_TruncatedHeader(t *testing.T) {
	// OpusHead packet truncated to 18 bytes (minimum is 19)
	packet := []byte{
		'O', 'p', 'u', 's', 'H', 'e', 'a', 'd', // magic (8 bytes)
		0x01,       // version (must be 1)
		0x02,       // 2 channels
		0x00, 0x00, // pre-skip (little-endian)
		0x80, 0xBB, 0x00, 0x00, // sample rate 48000 (little-endian)
		0x00, 0x00, // output gain (only 18 bytes total, missing mapping)
	}

	_, err := detectOggCodec(packet)
	if err == nil {
		t.Error("expected error for truncated Opus header")
	}
}

func TestDetectCodec_Vorbis(t *testing.T) {
	// Vorbis identification header
	packet := []byte{
		0x01,                         // packet type
		'v', 'o', 'r', 'b', 'i', 's', // "vorbis"
		0x00, 0x00, 0x00, 0x00, // version 0
		0x02,                   // 2 channels
		0x44, 0xAC, 0x00, 0x00, // 44100 Hz little-endian
	}

	codec, err := detectOggCodec(packet)
	if err != nil {
		t.Fatalf("detectOggCodec failed: %v", err)
	}
	vorbisC, ok := codec.(*vorbisCodec)
	if !ok {
		t.Fatalf("expected *vorbisCodec, got %T", codec)
	}

	// Verify parsed values
	if vorbisC.Channels() != 2 {
		t.Errorf("expected 2 channels, got %d", vorbisC.Channels())
	}
	if vorbisC.SampleRate() != 44100 {
		t.Errorf("expected sample rate 44100, got %d", vorbisC.SampleRate())
	}
}

func TestDetectCodec_Vorbis_InvalidVersion(t *testing.T) {
	// Vorbis identification header with invalid version (must be 0, but we use 1)
	packet := []byte{
		0x01,                         // packet type
		'v', 'o', 'r', 'b', 'i', 's', // "vorbis"
		0x01, 0x00, 0x00, 0x00, // version 1 (invalid, must be 0)
		0x02,                   // 2 channels
		0x44, 0xAC, 0x00, 0x00, // 44100 Hz little-endian
	}

	_, err := detectOggCodec(packet)
	if err == nil {
		t.Error("expected error for invalid Vorbis version")
	}
}

func TestDetectCodec_Vorbis_TruncatedHeader(t *testing.T) {
	// Vorbis identification header truncated to 15 bytes (minimum is 16)
	packet := []byte{
		0x01,                         // packet type
		'v', 'o', 'r', 'b', 'i', 's', // "vorbis"
		0x00, 0x00, 0x00, 0x00, // version 0
		0x02,             // 2 channels
		0x44, 0xAC, 0x00, // truncated sample rate (only 15 bytes total)
	}

	_, err := detectOggCodec(packet)
	if err == nil {
		t.Error("expected error for truncated Vorbis header")
	}
}

func TestDetectCodec_Unknown(t *testing.T) {
	packet := []byte("UnknownCodec")
	_, err := detectOggCodec(packet)
	if err == nil {
		t.Error("expected error for unknown codec")
	}
}
