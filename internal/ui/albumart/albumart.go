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

// NeedsLoad reports whether art still has to be loaded for a track. Cheap, so
// the UI can ask before starting a load.
func (r *Renderer) NeedsLoad(trackPath string) bool {
	if trackPath == "" {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentPath != trackPath || !r.transmitted
}

// LoadTrack reads the cover out of a track and resizes it, returning PNG data
// for Commit. It reads the audio file — hundreds of KB, over the network for a
// remote library — and resizes it, so it must run off the UI goroutine: it holds
// no lock while working, only snapshotting the target size (issue #49).
//
// Returns nil when the track has no usable cover, which Commit still needs to
// know about so it can drop the previous image.
func (r *Renderer) LoadTrack(trackPath string) []byte {
	if trackPath == "" {
		return nil
	}

	r.mu.RLock()
	cache, width, height := r.cache, r.width, r.height
	r.mu.RUnlock()

	pw, ph := r.protocol.TargetPixelSize(width, height)

	if cache != nil {
		if cached := cache.Get(trackPath, pw, ph); cached != nil {
			return cached
		}
	}

	data, _, err := tags.ExtractCoverArt(trackPath)
	if err != nil || data == nil {
		return nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	//nolint:gosec // dimensions are small, no overflow risk
	resized := resize.Thumbnail(uint(max(pw, 1)), uint(max(ph, 1)), img, resize.Lanczos3)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return nil
	}
	pngData := buf.Bytes()

	if cache != nil {
		_ = cache.Put(trackPath, pw, ph, pngData) //nolint:errcheck // cache is optional
	}
	return pngData
}

// Commit installs art loaded by LoadTrack and returns the command to write to
// the terminal. pngData may be nil, meaning the track has no cover: the previous
// image is then dropped. Cheap, so it belongs in Update.
func (r *Renderer) Commit(trackPath string, pngData []byte) string {
	if trackPath == "" {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var deleteCmd string
	if r.currentImageID > 0 {
		deleteCmd = r.protocol.Delete(r.currentImageID)
	}

	if pngData == nil {
		r.currentPath = trackPath
		r.currentImageID = 0
		r.transmitted = true
		r.transmitCmd = ""
		return deleteCmd
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
