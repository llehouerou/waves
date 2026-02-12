// Package albumart provides terminal-based album cover rendering using Kitty or Sixel graphics protocols.
package albumart

import (
	"bytes"
	"image"
	_ "image/jpeg" // JPEG decoder for album art
	"image/png"
	"sync"
	"sync/atomic"

	"github.com/nfnt/resize"

	"github.com/llehouerou/waves/internal/tags"
)

// Global image ID counter
var nextImageID uint32

func getNextImageID() uint32 {
	return atomic.AddUint32(&nextImageID, 1)
}

// Renderer handles album cover rendering with a terminal image protocol.
type Renderer struct {
	mu sync.RWMutex

	// Protocol used for image display
	protocol ImageProtocol

	// Current image state
	currentPath    string
	currentImageID uint32
	transmitted    bool

	// Cached transmission command (sent once per track)
	transmitCmd string

	// Image dimensions in cells
	width  int
	height int

	// Disk cache for resized images
	cache *Cache
}

// New creates a new album art renderer with the given protocol and disk caching.
func New(protocol ImageProtocol) *Renderer {
	cache, _ := NewCache("") // Ignore error, cache is optional
	return &Renderer{
		protocol: protocol,
		cache:    cache,
	}
}

// SetSize sets the display dimensions in terminal cells.
func (r *Renderer) SetSize(width, height int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.width != width || r.height != height {
		r.width = width
		r.height = height
		r.transmitted = false // Need to re-transmit at new size
	}
}

// PrepareTrack prepares album art for a track.
// Returns the transmission command that should be written to the terminal once.
// Returns empty string if already prepared or no cover art.
// Uses disk cache to avoid re-processing the same track.
func (r *Renderer) PrepareTrack(trackPath string) string {
	if trackPath == "" {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Already prepared for this track
	if r.currentPath == trackPath && r.transmitted {
		return ""
	}

	// New track - delete old image if any
	var deleteCmd string
	if r.currentImageID > 0 {
		deleteCmd = r.protocol.Delete(r.currentImageID)
	}

	// Compute pixel dimensions for resize and cache key
	pw, ph := r.protocol.TargetPixelSize(r.width, r.height)

	// Check disk cache first (keyed by pixel dimensions for protocol-specific sizes)
	if r.cache != nil {
		if cached := r.cache.Get(trackPath, pw, ph); cached != nil {
			return r.prepareFromPNG(trackPath, cached, deleteCmd)
		}
	}

	// Extract cover art
	data, _, err := tags.ExtractCoverArt(trackPath)
	if err != nil || data == nil {
		r.currentPath = trackPath
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		r.currentPath = trackPath
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
	}

	// Resize image to fit cell dimensions using protocol-specific pixel sizes
	pixelWidth := uint(max(pw, 1))  //nolint:gosec // dimensions are small, no overflow risk
	pixelHeight := uint(max(ph, 1)) //nolint:gosec // dimensions are small, no overflow risk

	// Resize maintaining aspect ratio
	resized := resize.Thumbnail(pixelWidth, pixelHeight, img, resize.Lanczos3)

	// Encode to PNG for caching and transmission
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		r.currentPath = trackPath
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
	}
	pngData := buf.Bytes()

	// Save to disk cache (keyed by pixel dimensions)
	if r.cache != nil {
		_ = r.cache.Put(trackPath, pw, ph, pngData) //nolint:errcheck // cache is optional
	}

	return r.prepareFromPNG(trackPath, pngData, deleteCmd)
}

// prepareFromPNG prepares PNG data via the protocol.
// Must be called with mutex held.
func (r *Renderer) prepareFromPNG(trackPath string, pngData []byte, deleteCmd string) string {
	r.currentImageID = getNextImageID()
	r.currentPath = trackPath

	prepareCmd, err := r.protocol.PrepareFromPNG(pngData, r.currentImageID)
	if err != nil {
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
	}

	r.transmitted = true
	r.transmitCmd = prepareCmd

	// Transmit new image before deleting the old one to avoid a brief
	// flash of no image (visible on Ghostty/Kitty).
	return prepareCmd + deleteCmd
}

// GetPlaceholder returns blank space for the layout.
func (r *Renderer) GetPlaceholder() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.protocol.Placeholder(r.width, r.height)
}

// GetPlacementCmd returns the command to place the image at given position.
// row and col are 1-based terminal coordinates.
// Returns empty string if no image is prepared.
func (r *Renderer) GetPlacementCmd(row, col int) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.currentImageID == 0 {
		return ""
	}

	return r.protocol.Place(r.currentImageID, row, col, r.width, r.height)
}

// HasImage returns true if there's a prepared image for the current track.
func (r *Renderer) HasImage() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.currentImageID > 0
}

// Clear removes the current image from terminal memory.
func (r *Renderer) Clear() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var cmd string
	if r.currentImageID > 0 {
		cmd = r.protocol.Delete(r.currentImageID)
	}

	r.currentPath = ""
	r.currentImageID = 0
	r.transmitted = false
	r.transmitCmd = ""

	return cmd
}

// CurrentPath returns the path of the currently prepared track.
func (r *Renderer) CurrentPath() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentPath
}

// InvalidateCache clears the cached path so the next PrepareTrack call
// will re-extract and re-transmit the album art, even for the same path.
func (r *Renderer) InvalidateCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentPath = ""
	r.transmitted = false
}

// PrepareFromBytes prepares album art from raw image bytes.
// Returns the transmission command that should be written to the terminal once.
// Returns empty string if already prepared or no cover art.
// The identifier is used to track if the image has changed.
func (r *Renderer) PrepareFromBytes(data []byte, identifier string) string {
	if len(data) == 0 {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Already prepared for this identifier
	if r.currentPath == identifier && r.transmitted {
		return ""
	}

	// New image - delete old image if any
	var deleteCmd string
	if r.currentImageID > 0 {
		deleteCmd = r.protocol.Delete(r.currentImageID)
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		r.currentPath = identifier
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
	}

	// Resize image to fit cell dimensions using protocol-specific pixel sizes
	pw, ph := r.protocol.TargetPixelSize(r.width, r.height)
	pixelWidth := uint(max(pw, 1))  //nolint:gosec // dimensions are small, no overflow risk
	pixelHeight := uint(max(ph, 1)) //nolint:gosec // dimensions are small, no overflow risk

	// Resize maintaining aspect ratio
	resized := resize.Thumbnail(pixelWidth, pixelHeight, img, resize.Lanczos3)

	// Get new image ID
	r.currentImageID = getNextImageID()
	r.currentPath = identifier

	// Generate prepare command
	prepareCmd, err := r.protocol.Prepare(resized, r.currentImageID)
	if err != nil {
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
	}

	r.transmitted = true
	r.transmitCmd = prepareCmd

	// Transmit new image before deleting the old one to avoid a brief
	// flash of no image (visible on Ghostty/Kitty).
	return prepareCmd + deleteCmd
}
