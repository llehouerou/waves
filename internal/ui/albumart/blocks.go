package albumart

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// upperHalfBlock fills the top half of a cell, so its foreground paints the
// upper image pixel and its background the lower one.
const upperHalfBlock = "▀"

// BlockProtocol renders cover art with unicode half blocks. It is the fallback
// for terminals that support neither Kitty nor Sixel graphics (Alacritty, plain
// xterm, Terminal.app), where the other protocols emit escapes the terminal
// silently drops (issue #30).
//
// Unlike Kitty and Sixel, the art is plain text: it goes into the view through
// Placeholder instead of being written over it with cursor moves, because
// Bubble Tea truncates rendered lines to the terminal width and would cut it.
type BlockProtocol struct {
	profile colorprofile.Profile

	mu      sync.RWMutex
	current uint32   // ID of the image being displayed
	lines   []string // its rendered rows, one per terminal cell row
}

// NewBlockProtocol creates a block renderer using the terminal's color profile,
// so 256-color and 16-color terminals get downsampled colors instead of
// truecolor escapes they cannot honor.
func NewBlockProtocol() *BlockProtocol {
	return &BlockProtocol{profile: colorprofile.Detect(os.Stdout, os.Environ())}
}

func (b *BlockProtocol) Prepare(img image.Image, id uint32) (string, error) {
	lines := b.render(img)

	b.mu.Lock()
	b.current, b.lines = id, lines
	b.mu.Unlock()

	return "", nil
}

func (b *BlockProtocol) PrepareFromPNG(pngData []byte, id uint32) (string, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return "", fmt.Errorf("decode png: %w", err)
	}
	return b.Prepare(img, id)
}

// render turns the image into one string per terminal cell row, two image pixel
// rows per line.
func (b *BlockProtocol) render(img image.Image) []string {
	bounds := img.Bounds()
	if bounds.Empty() {
		return nil
	}

	var lines []string
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		var sb strings.Builder
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			upper := img.At(x, y)
			lower := color.Color(color.Black)
			if y+1 < bounds.Max.Y {
				lower = img.At(x, y+1)
			}
			style := ansi.Style{}.
				ForegroundColor(b.profile.Convert(upper)).
				BackgroundColor(b.profile.Convert(lower))
			sb.WriteString(style.String())
			sb.WriteString(upperHalfBlock)
		}
		sb.WriteString(ansi.ResetStyle)
		lines = append(lines, sb.String())
	}
	return lines
}

// Place emits nothing: the art is already in the view via Placeholder.
func (b *BlockProtocol) Place(_ uint32, _, _, _, _ int) string {
	return ""
}

// Delete forgets the image. Nothing to erase in the terminal: the art is text
// that the next frame overwrites. A stale ID is ignored, so dropping the
// previous track's image never wipes the one that just replaced it.
func (b *BlockProtocol) Delete(id uint32) string {
	b.mu.Lock()
	if b.current == id {
		b.current, b.lines = 0, nil
	}
	b.mu.Unlock()

	return ""
}

// Placeholder returns the art itself, padded to exactly width x height cells so
// the surrounding layout never shifts. Blank when no image is prepared.
func (b *BlockProtocol) Placeholder(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	b.mu.RLock()
	art := b.lines
	b.mu.RUnlock()

	blank := strings.Repeat(" ", width)
	lines := make([]string, height)
	for i := range lines {
		if i >= len(art) {
			lines[i] = blank
			continue
		}
		lines[i] = fitLine(art[i], width)
	}
	return strings.Join(lines, "\n")
}

// fitLine pads or truncates a rendered row to exactly width cells, counting
// printable cells rather than bytes.
func fitLine(line string, width int) string {
	if w := lipgloss.Width(line); w < width {
		return line + strings.Repeat(" ", width-w)
	} else if w > width {
		return ansi.Truncate(line, width, "")
	}
	return line
}

// TargetPixelSize maps one cell to one pixel wide and two pixels tall.
func (b *BlockProtocol) TargetPixelSize(widthCells, heightCells int) (pixelWidth, pixelHeight int) {
	return widthCells, heightCells * 2
}
