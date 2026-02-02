package albumart

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// Kitty graphics protocol escape sequences
const (
	escStart = "\x1b_G"
	escEnd   = "\x1b\\"
)

// KittyImage holds a transmitted image reference.
type KittyImage struct {
	ID     uint32
	Width  int // in cells
	Height int // in cells
}

// TransmitImage sends an image to the terminal using Kitty protocol.
// Returns the image ID for later placement.
// The image is transmitted but not displayed (a=t).
func TransmitImage(img image.Image, id uint32) (string, error) {
	// Encode image as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}

	return TransmitImageFromPNG(buf.Bytes(), id)
}

// TransmitImageFromPNG sends pre-encoded PNG data to the terminal using Kitty protocol.
func TransmitImageFromPNG(pngData []byte, id uint32) (string, error) {
	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(pngData)

	// Build transmission command
	// a=t: transmit only (don't display)
	// f=100: PNG format
	// i=ID: image ID for later reference
	// q=2: quiet mode (suppress responses)
	var sb strings.Builder

	// Kitty protocol requires chunked transmission for large images
	// Each chunk max 4096 bytes
	const chunkSize = 4096

	for i := 0; i < len(encoded); i += chunkSize {
		end := min(i+chunkSize, len(encoded))
		chunk := encoded[i:end]
		isLast := end >= len(encoded)

		sb.WriteString(escStart)
		if i == 0 {
			// First chunk: include all parameters, m=1 if more chunks follow
			moreChunks := 0
			if !isLast {
				moreChunks = 1
			}
			fmt.Fprintf(&sb, "a=t,f=100,i=%d,q=2,m=%d;", id, moreChunks)
		} else {
			// Subsequent chunks: just indicate if more follow
			moreChunks := 0
			if !isLast {
				moreChunks = 1
			}
			fmt.Fprintf(&sb, "m=%d;", moreChunks)
		}
		sb.WriteString(chunk)
		sb.WriteString(escEnd)
	}

	return sb.String(), nil
}

// PlaceImage returns escape sequence to display a previously transmitted image.
// row and col are 1-based terminal coordinates.
// width and height are in cells.
// Uses a fixed placement ID (1) so that repositioning automatically replaces
// the previous placement without leaving ghost images.
func PlaceImage(id uint32, row, col, width, height int) string {
	// a=p: place image
	// i=ID: image ID
	// p=1: fixed placement ID (replaces existing placement with same ID)
	// c=cols, r=rows: size in cells
	// C=1: don't move cursor after placing
	// We use cursor positioning to place the image
	var sb strings.Builder

	// Save cursor, move to position, place image, restore cursor
	fmt.Fprintf(&sb, "\x1b[s\x1b[%d;%dH", row, col)
	fmt.Fprintf(&sb, "%sa=p,i=%d,p=1,c=%d,r=%d,C=1,q=2;%s", escStart, id, width, height, escEnd)
	sb.WriteString("\x1b[u")

	return sb.String()
}

// DeleteImage returns escape sequence to delete a transmitted image and clear its placements.
func DeleteImage(id uint32) string {
	// a=d: delete
	// d=i: delete by image ID and clear all placements of this image
	// i=ID: the image ID
	return fmt.Sprintf("%sa=d,d=i,i=%d,q=2;%s", escStart, id, escEnd)
}

// BlankPlaceholder returns a string of spaces for the image area.
// This is used in the layout so lipgloss doesn't try to measure image escapes.
func BlankPlaceholder(width, height int) string {
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
