package ui

// Base provides common UI component functionality for focus and size management.
// Embed this in component models to get standard methods automatically.
//
// Example:
//
//	type Model struct {
//	    ui.Base
//	    cursor cursor.Cursor
//	    items  []Item
//	}
type Base struct {
	width, height int
	focused       bool
}

// SetFocused sets whether the component is focused.
func (b *Base) SetFocused(focused bool) {
	b.focused = focused
}

// IsFocused returns whether the component is focused.
func (b Base) IsFocused() bool {
	return b.focused
}

// SetSize sets the component dimensions.
func (b *Base) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// Size returns the component dimensions.
func (b Base) Size() (width, height int) {
	return b.width, b.height
}

// Width returns the component width.
func (b Base) Width() int {
	return b.width
}

// Height returns the component height.
func (b Base) Height() int {
	return b.height
}

// ListHeight returns available height for list content after subtracting overhead.
func (b Base) ListHeight(overhead int) int {
	return b.height - overhead
}
