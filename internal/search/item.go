package search

// Item represents a searchable item.
type Item interface {
	// FilterValue returns the string to match against.
	FilterValue() string
	// DisplayText returns the string to display in results.
	DisplayText() string
}

// TwoColumnItem is an optional interface for items that want two-column display.
type TwoColumnItem interface {
	Item
	// LeftColumn returns the left column text (e.g., playlist name).
	LeftColumn() string
	// RightColumn returns the right column text (e.g., folder path).
	RightColumn() string
}
