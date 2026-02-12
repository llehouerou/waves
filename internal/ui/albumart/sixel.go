package albumart

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mattn/go-sixel"
)

// placeCounter is incremented on every Place call to ensure the output string
// is always unique. This prevents Bubble Tea's diff renderer from skipping
// the sixel data when only surrounding text changed (e.g. progress bar tick),
// which would leave the image partially erased.
var placeCounter uint64

// SixelProtocol implements ImageProtocol using the Sixel graphics protocol.
type SixelProtocol struct {
	mu     sync.RWMutex
	images map[uint32]string // cached Sixel-encoded data by image ID
	cellW  int               // cell width in pixels
	cellH  int               // cell height in pixels
}

// NewSixelProtocol creates a new SixelProtocol instance.
// It queries the terminal for actual cell pixel dimensions via TIOCGWINSZ.
func NewSixelProtocol() *SixelProtocol {
	cellW, cellH := getCellSize()
	return &SixelProtocol{
		images: make(map[uint32]string),
		cellW:  cellW,
		cellH:  cellH,
	}
}

func (s *SixelProtocol) Prepare(img image.Image, id uint32) (string, error) {
	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	enc.Dither = true

	if err := enc.Encode(img); err != nil {
		return "", fmt.Errorf("encode sixel: %w", err)
	}

	s.mu.Lock()
	s.images[id] = buf.String()
	s.mu.Unlock()

	return "", nil
}

func (s *SixelProtocol) PrepareFromPNG(pngData []byte, id uint32) (string, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return "", fmt.Errorf("decode png: %w", err)
	}
	return s.Prepare(img, id)
}

func (s *SixelProtocol) Place(id uint32, row, col, _, _ int) string {
	s.mu.RLock()
	data, ok := s.images[id]
	s.mu.RUnlock()

	if !ok {
		return ""
	}

	// Save cursor, move to position, emit sixel data, restore cursor.
	//
	// A monotonic counter is embedded in a no-op SGR sequence to ensure the
	// output string is unique on every call. Without this, Bubble Tea's diff
	// renderer would skip re-sending identical sixel data when only
	// surrounding text changed (progress bar tick, track change with same
	// art), leaving the image partially erased.
	seq := atomic.AddUint64(&placeCounter, 1)
	var sb strings.Builder
	fmt.Fprintf(&sb, "\x1b[s\x1b[%d;%dH", row, col)
	sb.WriteString(data)
	fmt.Fprintf(&sb, "\x1b[u\x1b[%dm\x1b[0m", seq%255+1)

	return sb.String()
}

func (s *SixelProtocol) Delete(id uint32) string {
	s.mu.Lock()
	delete(s.images, id)
	s.mu.Unlock()

	return ""
}

func (s *SixelProtocol) Placeholder(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	line := strings.Repeat(" ", width)
	lines := make([]string, height)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func (s *SixelProtocol) TargetPixelSize(widthCells, heightCells int) (pixelWidth, pixelHeight int) {
	// Use actual cell pixel dimensions from TIOCGWINSZ.
	return widthCells * s.cellW, heightCells * s.cellH
}
