// Package albumart provides terminal-based album cover rendering using Kitty graphics protocol.
package albumart

import (
	"bytes"
	"image"
	_ "image/jpeg" // JPEG decoder for album art
	"image/png"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nfnt/resize"

	"github.com/llehouerou/waves/internal/tags"
)

// encodePNG encodes an image as PNG bytes.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsKittySupported checks if the terminal supports Kitty graphics protocol.
func IsKittySupported() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("TERM") == "xterm-kitty" {
		return true
	}
	if os.Getenv("TERM_PROGRAM") == "WezTerm" {
		return true
	}
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}
	if version := os.Getenv("KONSOLE_VERSION"); version != "" {
		if len(version) >= 4 && version[:4] >= "2204" {
			return true
		}
	}
	return strings.Contains(os.Getenv("TERM"), "kitty")
}

// Global image ID counter
var nextImageID uint32

func getNextImageID() uint32 {
	return atomic.AddUint32(&nextImageID, 1)
}

// Renderer handles album cover rendering with Kitty graphics protocol.
type Renderer struct {
	mu sync.RWMutex

	// Current image state
	currentPath    string // Track path for the current image
	currentImageID uint32 // Kitty image ID (0 = no image)

	// Display dimensions in terminal cells
	width  int
	height int

	// Disk cache for resized images
	cache *Cache
}

// New creates a new album art renderer with disk caching.
func New() *Renderer {
	cache, _ := NewCache("") // Ignore error, cache is optional
	return &Renderer{
		cache: cache,
	}
}

// SetSize sets the display dimensions in terminal cells.
func (r *Renderer) SetSize(width, height int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.width = width
	r.height = height
}

// ProcessedImage holds the result of async image processing.
type ProcessedImage struct {
	Data []byte // Resized PNG data, nil/empty if no image
}

// ProcessTrackAsync extracts and processes album art without modifying renderer state.
// This is safe to call from a goroutine.
// Uses disk cache to avoid reprocessing the same track.
func (r *Renderer) ProcessTrackAsync(trackPath string) *ProcessedImage {
	if trackPath == "" {
		return nil
	}

	r.mu.RLock()
	width := r.width
	height := r.height
	cache := r.cache
	r.mu.RUnlock()

	// Check disk cache first
	if cache != nil {
		if cached := cache.Get(trackPath, width, height); cached != nil {
			return &ProcessedImage{Data: cached}
		}
	}

	// Extract cover art
	data, _, err := tags.ExtractCoverArt(trackPath)
	if err != nil || data == nil {
		return &ProcessedImage{} // Empty = no cover art
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return &ProcessedImage{}
	}

	// Resize image to fit cell dimensions
	// Terminal cells are ~8x16 pixels, so for WxH cells we need W*8 x H*16 pixels
	pixelWidth := uint(max(width*8, 64))    //nolint:gosec // dimensions are small
	pixelHeight := uint(max(height*16, 64)) //nolint:gosec // dimensions are small
	resized := resize.Thumbnail(pixelWidth, pixelHeight, img, resize.Lanczos3)

	// Encode as PNG
	pngData, err := encodePNG(resized)
	if err != nil {
		return &ProcessedImage{}
	}

	// Store in disk cache (fire and forget)
	if cache != nil {
		go cache.Put(trackPath, width, height, pngData) //nolint:errcheck // best-effort
	}

	return &ProcessedImage{Data: pngData}
}

// Apply atomically updates the renderer state with processed image data.
// Returns the Kitty protocol commands to send to the terminal.
// Must be called from the main thread (not async).
func (r *Renderer) Apply(trackPath string, processed *ProcessedImage) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete old image if any
	var deleteCmd string
	if r.currentImageID > 0 {
		deleteCmd = DeleteImage(r.currentImageID)
	}

	r.currentPath = trackPath

	// No image data - just clear
	if processed == nil || len(processed.Data) == 0 {
		r.currentImageID = 0
		return deleteCmd
	}

	// Transmit new image
	r.currentImageID = getNextImageID()
	transmitCmd, err := TransmitImageFromPNG(processed.Data, r.currentImageID)
	if err != nil {
		r.currentImageID = 0
		return deleteCmd
	}

	return deleteCmd + transmitCmd
}

// Clear removes the current image and returns the delete command.
func (r *Renderer) Clear() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var cmd string
	if r.currentImageID > 0 {
		cmd = DeleteImage(r.currentImageID)
	}

	r.currentPath = ""
	r.currentImageID = 0
	return cmd
}

// GetPlacementCmd returns the command to place the image at the given position.
// expectedPath must match the current image's track path, otherwise returns empty
// to prevent displaying stale art.
func (r *Renderer) GetPlacementCmd(row, col int, expectedPath string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.currentImageID == 0 || r.currentPath != expectedPath {
		return ""
	}

	return PlaceImage(r.currentImageID, row, col, r.width, r.height)
}

// HasImage returns true if there's a prepared image.
func (r *Renderer) HasImage() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentImageID > 0
}

// CurrentPath returns the track path of the current image.
func (r *Renderer) CurrentPath() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentPath
}

// GetPlaceholder returns blank space for layout measurement.
func (r *Renderer) GetPlaceholder() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return BlankPlaceholder(r.width, r.height)
}
