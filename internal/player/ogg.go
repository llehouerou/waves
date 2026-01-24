package player

import (
	"errors"
	"io"

	"github.com/gopxl/beep/v2"
)

// decodeOgg decodes an Ogg stream (Opus or Vorbis) into a beep streamer.
func decodeOgg(rc io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error) {
	// Read first page to get the identification packet
	hdr, err := parseOggPageHeader(rc)
	if err != nil {
		return nil, beep.Format{}, err
	}
	packets, partial, err := readOggPageBody(rc, hdr)
	if err != nil {
		return nil, beep.Format{}, err
	}
	if len(packets) == 0 {
		return nil, beep.Format{}, errors.New("ogg: no packets in first page")
	}

	// Detect codec from first packet
	codec, err := detectOggCodec(packets[0])
	if err != nil {
		return nil, beep.Format{}, err
	}

	// Feed header packets until codec is ready
	// Track partial packets that span pages
	for {
		complete, err := codec.AddHeaderPacket(nil) // Check if already complete
		if err != nil {
			return nil, beep.Format{}, err
		}
		if complete {
			break
		}

		// Read more pages for headers
		hdr, err := parseOggPageHeader(rc)
		if err != nil {
			return nil, beep.Format{}, err
		}
		pagePackets, newPartial, err := readOggPageBody(rc, hdr)
		if err != nil {
			return nil, beep.Format{}, err
		}

		// Join partial from previous page with first packet/partial of this page
		if len(partial) > 0 {
			if len(pagePackets) > 0 {
				// Previous partial + first complete packet = one header
				pagePackets[0] = append(partial, pagePackets[0]...)
			} else if newPartial != nil {
				// Previous partial + new partial (still spanning)
				newPartial = append(partial, newPartial...)
			}
		}

		// Feed complete packets to codec
		for _, pkt := range pagePackets {
			complete, err = codec.AddHeaderPacket(pkt)
			if err != nil {
				return nil, beep.Format{}, err
			}
			if complete {
				break
			}
		}

		// Track new partial for next iteration
		partial = newPartial
	}

	// Record where audio data starts (after headers)
	dataStart, err := rc.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, beep.Format{}, err
	}

	// Create OggReader
	ogg, err := NewOggReader(rc, codec.SampleRate(), codec.PreSkip())
	if err != nil {
		return nil, beep.Format{}, err
	}
	ogg.SetDataStart(dataStart)
	if err := ogg.ScanLastGranule(); err != nil {
		return nil, beep.Format{}, err
	}
	// Seek back to audio start
	if _, err := rc.Seek(dataStart, io.SeekStart); err != nil {
		return nil, beep.Format{}, err
	}

	format := beep.Format{
		SampleRate:  beep.SampleRate(codec.SampleRate()),
		NumChannels: codec.Channels(),
		Precision:   2,
	}

	decoder := &oggDecoder{
		ogg:       ogg,
		codec:     codec,
		closer:    rc,
		pcmBuffer: make([]float32, 8192*codec.Channels()),
		totalLen:  ogg.Duration(),
	}
	decoder.pcmPos = len(decoder.pcmBuffer) // empty buffer triggers refill

	return decoder, format, nil
}

// oggDecoder implements beep.StreamSeekCloser for Ogg streams.
type oggDecoder struct {
	ogg    *OggReader
	codec  OggCodec
	closer io.Closer

	currentPage *OggPage
	packetIdx   int
	pcmBuffer   []float32
	pcmPos      int
	granulePos  int64
	totalLen    int64
	err         error
}

// Stream reads audio samples into the provided buffer.
func (d *oggDecoder) Stream(samples [][2]float64) (n int, ok bool) {
	if d.err != nil {
		return 0, false
	}

	channels := d.codec.Channels()

	for n < len(samples) {
		// Use buffered PCM
		if d.pcmPos < len(d.pcmBuffer) {
			for n < len(samples) && d.pcmPos < len(d.pcmBuffer) {
				if channels == 2 {
					samples[n][0] = float64(d.pcmBuffer[d.pcmPos])
					samples[n][1] = float64(d.pcmBuffer[d.pcmPos+1])
					d.pcmPos += 2
				} else {
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

			samplesPerChannel, err := d.codec.Decode(packet, d.pcmBuffer[:cap(d.pcmBuffer)])
			if err != nil {
				continue // skip invalid packets
			}
			d.pcmBuffer = d.pcmBuffer[:samplesPerChannel*channels]
			d.pcmPos = 0
		}
	}

	return n, true
}

// Err returns any error that occurred during streaming.
func (d *oggDecoder) Err() error { return d.err }

// Len returns the total number of samples.
func (d *oggDecoder) Len() int { return int(d.totalLen) }

// Position returns the current sample position.
func (d *oggDecoder) Position() int { return int(d.granulePos) }

// Seek seeks to the given sample position.
func (d *oggDecoder) Seek(p int) error {
	if p < 0 {
		p = 0
	}
	if p > d.Len() {
		p = d.Len()
	}

	if err := d.ogg.SeekToGranule(int64(p)); err != nil {
		return err
	}

	d.currentPage = nil
	d.packetIdx = 0
	d.pcmBuffer = d.pcmBuffer[:cap(d.pcmBuffer)]
	d.pcmPos = len(d.pcmBuffer)
	d.granulePos = int64(p)
	d.err = nil

	return d.codec.Reset()
}

// Close closes the decoder and underlying file.
func (d *oggDecoder) Close() error {
	return d.closer.Close()
}
