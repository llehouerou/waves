package albumart

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// Kitty graphics protocol escape sequences.
const (
	escStart = "\x1b_G"
	escEnd   = "\x1b\\"
)

// KittyProtocol implements ImageProtocol using the Kitty graphics protocol.
type KittyProtocol struct{}

func (k *KittyProtocol) Prepare(img image.Image, id uint32) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}
	return k.PrepareFromPNG(buf.Bytes(), id)
}

func (k *KittyProtocol) PrepareFromPNG(pngData []byte, id uint32) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(pngData)

	var sb strings.Builder

	const chunkSize = 4096

	for i := 0; i < len(encoded); i += chunkSize {
		end := min(i+chunkSize, len(encoded))
		chunk := encoded[i:end]
		isLast := end >= len(encoded)

		sb.WriteString(escStart)
		if i == 0 {
			moreChunks := 0
			if !isLast {
				moreChunks = 1
			}
			fmt.Fprintf(&sb, "a=t,f=100,i=%d,q=2,m=%d;", id, moreChunks)
		} else {
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

func (k *KittyProtocol) Place(id uint32, row, col, width, height int) string {
	var sb strings.Builder

	// Save cursor, move to position, place image, restore cursor
	fmt.Fprintf(&sb, "\x1b[s\x1b[%d;%dH", row, col)
	fmt.Fprintf(&sb, "%sa=p,i=%d,p=1,c=%d,r=%d,C=1,q=2;%s", escStart, id, width, height, escEnd)
	sb.WriteString("\x1b[u")

	return sb.String()
}

func (k *KittyProtocol) Delete(id uint32) string {
	return fmt.Sprintf("%sa=d,d=i,i=%d,q=2;%s", escStart, id, escEnd)
}

func (k *KittyProtocol) TargetPixelSize(widthCells, heightCells int) (pixelWidth, pixelHeight int) {
	return widthCells * 8, heightCells * 16
}

func (k *KittyProtocol) Placeholder(width, height int) string {
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
