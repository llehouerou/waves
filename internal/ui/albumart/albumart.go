// Package albumart provides terminal-based album cover rendering using Kitty graphics protocol.
package albumart

import (
	"bytes"
	"image"
	_ "image/jpeg" // JPEG decoder for album art
	_ "image/png"  // PNG decoder for album art
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nfnt/resize"

	"github.com/llehouerou/waves/internal/player"
)

// IsKittySupported checks if the terminal supports Kitty graphics protocol.
// It checks for known environment variables set by terminals that support it.
func IsKittySupported() bool {
	// Kitty terminal sets KITTY_WINDOW_ID
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}

	// Kitty also sets TERM=xterm-kitty
	if os.Getenv("TERM") == "xterm-kitty" {
		return true
	}

	// WezTerm supports Kitty graphics protocol
	if os.Getenv("TERM_PROGRAM") == "WezTerm" {
		return true
	}

	// Ghostty supports Kitty graphics protocol
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}

	// Konsole 22.04+ supports Kitty graphics (check KONSOLE_VERSION)
	if version := os.Getenv("KONSOLE_VERSION"); version != "" {
		// KONSOLE_VERSION is like "220401" for 22.04.01
		// Kitty graphics supported from 22.04+
		if len(version) >= 4 && version[:4] >= "2204" {
			return true
		}
	}

	// Check TERM for other Kitty-compatible indicators
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
	currentPath    string
	currentImageID uint32
	transmitted    bool

	// Cached transmission command (sent once per track)
	transmitCmd string

	// Image dimensions in cells
	width  int
	height int
}

// New creates a new album art renderer.
func New() *Renderer {
	return &Renderer{}
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
		deleteCmd = DeleteImage(r.currentImageID)
	}

	// Extract cover art
	data, _, err := player.ExtractCoverArt(trackPath)
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

	// Resize image to fit cell dimensions
	// Assuming ~2:1 aspect ratio for terminal cells (cell is taller than wide)
	// For a square album art in WxH cells, we need W*charWidth x H*charHeight pixels
	// Typical cell is about 8x16 pixels, so ratio is 1:2
	// For 8 cells wide x 4 cells tall, target roughly square: 8*8=64 x 4*16=64
	pixelWidth := uint(max(r.width*8, 64))    //nolint:gosec // dimensions are small, no overflow risk
	pixelHeight := uint(max(r.height*16, 64)) //nolint:gosec // dimensions are small, no overflow risk

	// Resize maintaining aspect ratio
	resized := resize.Thumbnail(pixelWidth, pixelHeight, img, resize.Lanczos3)

	// Get new image ID
	r.currentImageID = getNextImageID()
	r.currentPath = trackPath

	// Generate transmission command
	transmitCmd, err := TransmitImage(resized, r.currentImageID)
	if err != nil {
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
	}

	r.transmitted = true
	r.transmitCmd = transmitCmd

	return deleteCmd + transmitCmd
}

// GetPlaceholder returns blank space for the layout.
func (r *Renderer) GetPlaceholder() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return BlankPlaceholder(r.width, r.height)
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

	return PlaceImage(r.currentImageID, row, col, r.width, r.height)
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
		cmd = DeleteImage(r.currentImageID)
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
