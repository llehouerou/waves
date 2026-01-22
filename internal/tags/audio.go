package tags

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	goflac "github.com/go-flac/go-flac"
	"github.com/gopxl/beep/v2/flac"
	"github.com/llehouerou/go-m4a"
	"github.com/llehouerou/go-mp3"
)

// ReadAudioInfo reads audio stream properties (duration, format, sample rate).
// This uses lighter-weight methods than full decoding where possible.
func ReadAudioInfo(path string) (*AudioInfo, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ExtMP3 && ext != ExtFLAC && ext != ExtOPUS && ext != ExtOGG && ext != ExtM4A && ext != ExtMP4 {
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	switch ext {
	case ExtMP3:
		return readMP3AudioInfo(f)
	case ExtFLAC:
		return readFLACStreamInfo(path)
	case ExtOPUS, ExtOGG:
		return readOpusAudioInfo(f)
	case ExtM4A, ExtMP4:
		return readM4AAudioInfo(f)
	}

	return nil, fmt.Errorf("unsupported format: %s", ext)
}

// readMP3AudioInfo extracts audio info from an MP3 file.
func readMP3AudioInfo(f *os.File) (*AudioInfo, error) {
	decoder, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, err
	}

	sampleRate := decoder.SampleRate()
	if sampleRate == 0 {
		return nil, errors.New("mp3: invalid sample rate")
	}

	sampleCount := max(decoder.SampleCount(), 0)

	duration := time.Duration(float64(sampleCount) / float64(sampleRate) * float64(time.Second))

	return &AudioInfo{
		Duration:   duration,
		Format:     "MP3",
		SampleRate: sampleRate,
		BitDepth:   16, // MP3 decodes to 16-bit
	}, nil
}

// readFLACStreamInfo extracts audio info from FLAC streaminfo metadata.
func readFLACStreamInfo(path string) (*AudioInfo, error) {
	// Parse FLAC file to get metadata
	flacFile, err := goflac.ParseFile(path)
	if err != nil {
		// Try with ID3v2 skip for files with prepended ID3 tags
		return readFLACWithBeep(path)
	}

	// Find StreamInfo block
	for _, meta := range flacFile.Meta {
		if meta.Type != goflac.StreamInfo || len(meta.Data) < 18 {
			continue
		}
		// Parse StreamInfo block
		// Bytes 10-13: sample rate (20 bits), channels (3 bits), bits per sample (5 bits)
		// Bytes 14-17: total samples (36 bits, but only lower 32 bits typically used)
		data := meta.Data

		// Sample rate is in bits 0-19 of bytes 10-12
		sampleRate := int(data[10])<<12 | int(data[11])<<4 | int(data[12])>>4
		// Bits per sample is in bits 4-8 of bytes 12-13 (add 1 to get actual value)
		bitsPerSample := (int(data[12])&0x01)<<4 | int(data[13])>>4 + 1

		// Total samples is in bytes 14-17 (plus 4 bits from byte 13)
		totalSamples := int64(data[13]&0x0F)<<32 | int64(data[14])<<24 | int64(data[15])<<16 | int64(data[16])<<8 | int64(data[17])

		duration := time.Duration(0)
		if sampleRate > 0 {
			duration = time.Duration(float64(totalSamples) / float64(sampleRate) * float64(time.Second))
		}

		return &AudioInfo{
			Duration:   duration,
			Format:     "FLAC",
			SampleRate: sampleRate,
			BitDepth:   bitsPerSample,
		}, nil
	}

	// Fallback to beep decoder
	return readFLACWithBeep(path)
}

// readFLACWithBeep uses beep's FLAC decoder as fallback.
func readFLACWithBeep(path string) (*AudioInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Skip ID3v2 if present
	if err := skipID3v2(f); err != nil {
		return nil, err
	}

	streamer, format, err := flac.Decode(f)
	if err != nil {
		return nil, err
	}
	defer streamer.Close()

	return &AudioInfo{
		Duration:   format.SampleRate.D(streamer.Len()),
		Format:     "FLAC",
		SampleRate: int(format.SampleRate),
		BitDepth:   format.Precision * 8,
	}, nil
}

// readOpusAudioInfo extracts audio info from an Opus/OGG file.
func readOpusAudioInfo(f *os.File) (*AudioInfo, error) {
	// Opus always decodes at 48kHz
	const opusSampleRate = 48000

	// Parse OGG to get duration from granule positions
	duration, err := getOggDuration(f)
	if err != nil {
		return nil, err
	}

	return &AudioInfo{
		Duration:   duration,
		Format:     "OPUS",
		SampleRate: opusSampleRate,
		BitDepth:   16,
	}, nil
}

// getOggDuration calculates duration from OGG granule position.
func getOggDuration(f *os.File) (time.Duration, error) {
	// Seek to end to find last page's granule position
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}

	// Read the last 64KB to find the last OGG page
	searchSize := min(int64(65536), fi.Size())

	if _, err := f.Seek(-searchSize, io.SeekEnd); err != nil {
		return 0, err
	}

	buf := make([]byte, searchSize)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	buf = buf[:n]

	// Search backwards for OggS magic
	var lastGranule int64
	for i := len(buf) - 27; i >= 0; i-- {
		if buf[i] == 'O' && buf[i+1] == 'g' && buf[i+2] == 'g' && buf[i+3] == 'S' {
			// Found an OGG page header
			// Granule position is at offset 6, 8 bytes little-endian
			if i+14 <= len(buf) {
				lastGranule = int64(buf[i+6]) | int64(buf[i+7])<<8 | int64(buf[i+8])<<16 | int64(buf[i+9])<<24 |
					int64(buf[i+10])<<32 | int64(buf[i+11])<<40 | int64(buf[i+12])<<48 | int64(buf[i+13])<<56
				break
			}
		}
	}

	// Opus uses 48000 Hz sample rate
	if lastGranule > 0 {
		return time.Duration(float64(lastGranule) / 48000.0 * float64(time.Second)), nil
	}

	return 0, errors.New("could not determine OGG duration")
}

// readM4AAudioInfo extracts audio info from an M4A/MP4 file.
func readM4AAudioInfo(f *os.File) (*AudioInfo, error) {
	container, err := m4a.Open(f)
	if err != nil {
		return nil, err
	}

	codecType := container.Codec()
	var format string
	switch codecType {
	case m4a.CodecAAC:
		format = "AAC"
	case m4a.CodecALAC:
		format = "ALAC"
	case m4a.CodecUnknown:
		format = "M4A"
	}

	bitDepth := 16
	if codecType == m4a.CodecALAC && container.SampleSize() == 24 {
		bitDepth = 24
	}

	return &AudioInfo{
		Duration:   container.Duration(),
		Format:     format,
		SampleRate: int(container.SampleRate()),
		BitDepth:   bitDepth,
	}, nil
}

// skipID3v2 skips an ID3v2 tag if present at the beginning of the file.
func skipID3v2(r io.ReadSeeker) error {
	header := make([]byte, 10)
	n, err := r.Read(header)
	if err != nil {
		return err
	}
	if n < 10 {
		_, err = r.Seek(0, io.SeekStart)
		return err
	}

	if string(header[0:3]) != id3Magic {
		_, err = r.Seek(0, io.SeekStart)
		return err
	}

	// ID3v2 size is stored as a syncsafe integer in bytes 6-9
	size := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
	_, err = r.Seek(10+size, io.SeekStart)
	return err
}
