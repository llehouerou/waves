package albumart

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestTransmitImageFromPNG_SmallImage(t *testing.T) {
	// Create a small PNG
	pngData := createTestPNG(t, 10, 10)

	cmd, err := TransmitImageFromPNG(pngData, 1)
	if err != nil {
		t.Fatalf("TransmitImageFromPNG() error: %v", err)
	}

	// Verify escape sequence structure
	if !strings.HasPrefix(cmd, escStart) {
		t.Errorf("command should start with escStart")
	}
	if !strings.HasSuffix(cmd, escEnd) {
		t.Errorf("command should end with escEnd")
	}

	// Verify parameters in first chunk
	if !strings.Contains(cmd, "a=t") {
		t.Error("command should contain a=t (transmit action)")
	}
	if !strings.Contains(cmd, "f=100") {
		t.Error("command should contain f=100 (PNG format)")
	}
	if !strings.Contains(cmd, "i=1") {
		t.Error("command should contain i=1 (image ID)")
	}
	if !strings.Contains(cmd, "q=2") {
		t.Error("command should contain q=2 (quiet mode)")
	}
}

func TestTransmitImageFromPNG_LargeData_Chunked(t *testing.T) {
	// Create data that will exceed 4096 bytes when base64 encoded
	// 4096 base64 chars = ~3072 bytes raw data
	// Use fake "PNG" data large enough to trigger chunking
	pngData := make([]byte, 4000) // Will produce >5300 base64 chars
	for i := range pngData {
		pngData[i] = byte(i % 256)
	}

	cmd, err := TransmitImageFromPNG(pngData, 42)
	if err != nil {
		t.Fatalf("TransmitImageFromPNG() error: %v", err)
	}

	// Count escape sequences - should have multiple chunks
	chunkCount := strings.Count(cmd, escStart)
	if chunkCount < 2 {
		t.Errorf("expected multiple chunks for large data, got %d", chunkCount)
	}

	// First chunk should have m=1 (more chunks follow)
	if !strings.Contains(cmd, "m=1") {
		t.Error("first chunk should have m=1 for continuation")
	}

	// Last chunk should have m=0
	lastChunkIdx := strings.LastIndex(cmd, escStart)
	lastChunk := cmd[lastChunkIdx:]
	if !strings.Contains(lastChunk, "m=0") {
		t.Error("last chunk should have m=0")
	}

	// Verify image ID is in first chunk only
	firstChunk, rest, found := strings.Cut(cmd, escEnd)
	if !found {
		t.Fatal("could not find escEnd in command")
	}
	if !strings.Contains(firstChunk, "i=42") {
		t.Error("first chunk should contain image ID")
	}

	// Subsequent chunks should not have image ID
	secondChunkStart := strings.Index(rest, escStart)
	if secondChunkStart != -1 {
		secondChunkEnd := strings.Index(rest[secondChunkStart:], escEnd)
		if secondChunkEnd != -1 {
			secondChunk := rest[secondChunkStart : secondChunkStart+secondChunkEnd]
			if strings.Contains(secondChunk, "i=") {
				t.Error("subsequent chunks should not contain image ID")
			}
		}
	}
}

func TestTransmitImageFromPNG_DifferentIDs(t *testing.T) {
	pngData := createTestPNG(t, 10, 10)

	tests := []uint32{1, 42, 100, 65535}
	for _, id := range tests {
		cmd, err := TransmitImageFromPNG(pngData, id)
		if err != nil {
			t.Fatalf("TransmitImageFromPNG(id=%d) error: %v", id, err)
		}

		expectedID := "i=" + itoa(id)
		if !strings.Contains(cmd, expectedID) {
			t.Errorf("command should contain %s for id=%d", expectedID, id)
		}
	}
}

func TestTransmitImageFromPNG_Base64Encoded(t *testing.T) {
	pngData := createTestPNG(t, 10, 10)

	cmd, err := TransmitImageFromPNG(pngData, 1)
	if err != nil {
		t.Fatalf("TransmitImageFromPNG() error: %v", err)
	}

	// Extract payload (between ; and escEnd)
	payload := extractPayload(cmd)

	// Verify it's valid base64
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("payload is not valid base64: %v", err)
	}

	// Verify decoded data matches original
	if !bytes.Equal(decoded, pngData) {
		t.Error("decoded payload doesn't match original PNG data")
	}
}

func TestTransmitImage_FromImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	cmd, err := TransmitImage(img, 5)
	if err != nil {
		t.Fatalf("TransmitImage() error: %v", err)
	}

	// Verify basic structure
	if !strings.HasPrefix(cmd, escStart) {
		t.Error("command should start with escStart")
	}
	if !strings.Contains(cmd, "i=5") {
		t.Error("command should contain image ID")
	}
}

func TestPlaceImage(t *testing.T) {
	cmd := PlaceImage(42, 5, 10, 8, 4)

	// Verify escape sequence
	if !strings.Contains(cmd, escStart) {
		t.Error("command should contain escStart")
	}
	if !strings.Contains(cmd, escEnd) {
		t.Error("command should contain escEnd")
	}

	// Verify cursor save/restore
	if !strings.Contains(cmd, "\x1b[s") {
		t.Error("command should save cursor")
	}
	if !strings.Contains(cmd, "\x1b[u") {
		t.Error("command should restore cursor")
	}

	// Verify cursor positioning (row 5, col 10)
	if !strings.Contains(cmd, "\x1b[5;10H") {
		t.Error("command should position cursor at row 5, col 10")
	}

	// Verify placement parameters
	if !strings.Contains(cmd, "a=p") {
		t.Error("command should contain a=p (place action)")
	}
	if !strings.Contains(cmd, "i=42") {
		t.Error("command should contain i=42 (image ID)")
	}
	if !strings.Contains(cmd, "p=1") {
		t.Error("command should contain p=1 (placement ID)")
	}
	if !strings.Contains(cmd, "c=8") {
		t.Error("command should contain c=8 (width in cells)")
	}
	if !strings.Contains(cmd, "r=4") {
		t.Error("command should contain r=4 (height in cells)")
	}
	if !strings.Contains(cmd, "C=1") {
		t.Error("command should contain C=1 (don't move cursor)")
	}
}

func TestPlaceImage_DifferentPositions(t *testing.T) {
	tests := []struct {
		row, col int
	}{
		{1, 1},
		{10, 20},
		{100, 50},
	}

	for _, tt := range tests {
		cmd := PlaceImage(1, tt.row, tt.col, 8, 4)
		expected := fmt.Sprintf("\x1b[%d;%dH", tt.row, tt.col)
		if !strings.Contains(cmd, expected) {
			t.Errorf("PlaceImage(%d, %d) should position cursor at %s", tt.row, tt.col, expected)
		}
	}
}

func TestDeleteImage(t *testing.T) {
	cmd := DeleteImage(42)

	// Verify escape sequence structure
	if !strings.HasPrefix(cmd, escStart) {
		t.Error("command should start with escStart")
	}
	if !strings.HasSuffix(cmd, escEnd) {
		t.Error("command should end with escEnd")
	}

	// Verify delete parameters
	if !strings.Contains(cmd, "a=d") {
		t.Error("command should contain a=d (delete action)")
	}
	if !strings.Contains(cmd, "d=i") {
		t.Error("command should contain d=i (delete by image ID)")
	}
	if !strings.Contains(cmd, "i=42") {
		t.Error("command should contain i=42 (image ID)")
	}
	if !strings.Contains(cmd, "q=2") {
		t.Error("command should contain q=2 (quiet mode)")
	}
}

func TestDeleteImage_DifferentIDs(t *testing.T) {
	tests := []uint32{1, 100, 65535}
	for _, id := range tests {
		cmd := DeleteImage(id)
		expectedID := "i=" + itoa(id)
		if !strings.Contains(cmd, expectedID) {
			t.Errorf("DeleteImage(%d) should contain %s", id, expectedID)
		}
	}
}

func TestBlankPlaceholder(t *testing.T) {
	tests := []struct {
		width, height int
		wantLines     int
		wantWidth     int
	}{
		{8, 4, 4, 8},
		{10, 2, 2, 10},
		{1, 1, 1, 1},
		{20, 10, 10, 20},
	}

	for _, tt := range tests {
		placeholder := BlankPlaceholder(tt.width, tt.height)
		lines := strings.Split(placeholder, "\n")

		if len(lines) != tt.wantLines {
			t.Errorf("BlankPlaceholder(%d, %d) got %d lines, want %d",
				tt.width, tt.height, len(lines), tt.wantLines)
		}

		for i, line := range lines {
			if len(line) != tt.wantWidth {
				t.Errorf("BlankPlaceholder(%d, %d) line %d has width %d, want %d",
					tt.width, tt.height, i, len(line), tt.wantWidth)
			}
			// All characters should be spaces
			if strings.TrimLeft(line, " ") != "" {
				t.Errorf("BlankPlaceholder(%d, %d) line %d contains non-space characters",
					tt.width, tt.height, i)
			}
		}
	}
}

func TestBlankPlaceholder_ZeroDimensions(t *testing.T) {
	tests := []struct {
		width, height int
	}{
		{0, 4},
		{8, 0},
		{0, 0},
		{-1, 4},
		{8, -1},
	}

	for _, tt := range tests {
		placeholder := BlankPlaceholder(tt.width, tt.height)
		if placeholder != "" {
			t.Errorf("BlankPlaceholder(%d, %d) = %q, want empty string",
				tt.width, tt.height, placeholder)
		}
	}
}

// Helper functions

func createTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with a color
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	data, err := encodePNG(img)
	if err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return data
}

func extractPayload(cmd string) string {
	// Find content between first ; and first escEnd
	start := strings.Index(cmd, ";")
	end := strings.Index(cmd, escEnd)
	if start == -1 || end == -1 || start >= end {
		return ""
	}
	return cmd[start+1 : end]
}

// itoa converts uint32 to string (simple helper to avoid fmt.Sprintf)
func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestEncodePNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	data, err := encodePNG(img)
	if err != nil {
		t.Fatalf("encodePNG() error: %v", err)
	}

	// Verify PNG signature
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(data) < 8 {
		t.Fatal("PNG data too short")
	}
	for i, b := range pngSignature {
		if data[i] != b {
			t.Errorf("PNG signature byte %d = %02x, want %02x", i, data[i], b)
		}
	}

	// Verify it can be decoded back
	_, err = png.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Errorf("encoded PNG cannot be decoded: %v", err)
	}
}
