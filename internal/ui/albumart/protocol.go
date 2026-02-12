package albumart

import "image"

// ImageProtocol abstracts the terminal image display protocol (Kitty or Sixel).
type ImageProtocol interface {
	// Prepare encodes the image and returns any one-time terminal command.
	// Kitty: transmits to terminal memory, returns escape sequences.
	// Sixel: encodes and caches internally, returns empty string.
	Prepare(img image.Image, id uint32) (string, error)

	// PrepareFromPNG same but from pre-encoded PNG data.
	PrepareFromPNG(pngData []byte, id uint32) (string, error)

	// Place returns the escape sequence to display the image at (row, col).
	// Kitty: references by ID (lightweight).
	// Sixel: emits full image data with cursor positioning.
	Place(id uint32, row, col, width, height int) string

	// Delete returns the escape sequence to remove the image.
	// Sixel: no-op (returns "").
	Delete(id uint32) string

	// Placeholder returns blank space string for lipgloss layout measurement.
	Placeholder(width, height int) string

	// TargetPixelSize returns the pixel dimensions to use when resizing an
	// image that will be displayed in the given number of terminal cells.
	// Kitty: uses standard 8x16 cell assumptions.
	// Sixel: queries actual cell pixel size and leaves 1 row of vertical
	// margin to prevent terminal scroll when the image is near the bottom.
	TargetPixelSize(widthCells, heightCells int) (pixelWidth, pixelHeight int)
}
