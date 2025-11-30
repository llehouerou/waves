// Package kittyimg provides Kitty terminal graphics protocol support.
package kittyimg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg" // Register JPEG decoder for image.Decode
	"image/png"
	"strings"
)

const (
	chunkSize = 4096 // Max bytes per escape sequence chunk
)

// Encode converts image data to a Kitty graphics protocol escape sequence.
// The image will be displayed at the specified column and row dimensions.
// Returns empty string if data is nil or empty.
func Encode(data []byte, cols, rows int) string {
	if len(data) == 0 {
		return ""
	}

	// Decode the image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}

	// Re-encode as PNG for Kitty
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}

	pngData := buf.Bytes()
	b64Data := base64.StdEncoding.EncodeToString(pngData)

	// Build the escape sequence(s)
	// Format: ESC ] G <params> ; <payload> ESC \
	// We use: a=T (transmit+display), f=100 (PNG), c=cols, r=rows
	var sb strings.Builder

	// Split into chunks if needed
	for i := 0; i < len(b64Data); i += chunkSize {
		end := min(i+chunkSize, len(b64Data))
		chunk := b64Data[i:end]

		// m=1 means more chunks follow, m=0 means last chunk
		more := 0
		if end < len(b64Data) {
			more = 1
		}

		if i == 0 {
			// First chunk includes all parameters
			sb.WriteString(fmt.Sprintf("\x1b_Ga=T,f=100,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, chunk))
		} else {
			// Subsequent chunks only have m parameter
			sb.WriteString(fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, chunk))
		}
	}

	return sb.String()
}

// Placeholder returns an ASCII art placeholder for missing cover art.
func Placeholder(cols, rows int) string {
	if cols < 4 || rows < 2 {
		return ""
	}

	var lines []string

	// Top border
	lines = append(lines, "┌"+strings.Repeat("─", cols-2)+"┐")

	// Middle rows with music note
	for i := 1; i < rows-1; i++ {
		if i == rows/2 && cols >= 5 {
			// Center a music note
			padding := (cols - 3) / 2
			line := "│" + strings.Repeat(" ", padding) + "♪" + strings.Repeat(" ", cols-3-padding) + "│"
			lines = append(lines, line)
		} else {
			lines = append(lines, "│"+strings.Repeat(" ", cols-2)+"│")
		}
	}

	// Bottom border
	lines = append(lines, "└"+strings.Repeat("─", cols-2)+"┘")

	return strings.Join(lines, "\n")
}
