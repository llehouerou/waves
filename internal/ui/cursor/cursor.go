// Package cursor provides a reusable cursor component for scrollable lists.
package cursor

// Cursor manages cursor position and scroll offset for a scrollable list.
// The list length and viewport height are passed to methods rather than stored,
// since they can change dynamically.
type Cursor struct {
	pos    int // Current cursor position (0-indexed)
	offset int // Scroll offset (first visible item index)
	margin int // Scroll margin (items to keep visible above/below cursor)
}

// New creates a new Cursor with the specified scroll margin.
func New(margin int) Cursor {
	return Cursor{
		pos:    0,
		offset: 0,
		margin: margin,
	}
}

// Pos returns the current cursor position.
func (c Cursor) Pos() int {
	return c.pos
}

// Offset returns the current scroll offset.
func (c Cursor) Offset() int {
	return c.offset
}

// Margin returns the current scroll margin.
func (c Cursor) Margin() int {
	return c.margin
}

// SetMargin updates the scroll margin.
func (c *Cursor) SetMargin(margin int) {
	c.margin = margin
}

// Move moves the cursor by delta positions within a list of given length.
// It clamps the cursor to valid bounds and adjusts the offset for visibility.
// If listLen is 0, this is a no-op.
func (c *Cursor) Move(delta, listLen, height int) {
	if listLen == 0 {
		return
	}
	c.pos = clamp(c.pos+delta, listLen-1)
	c.ensureVisible(listLen, height)
}

// Jump sets the cursor to an absolute position within a list of given length.
// It clamps the cursor to valid bounds and adjusts the offset for visibility.
// If listLen is 0, this is a no-op.
func (c *Cursor) Jump(pos, listLen, height int) {
	if listLen == 0 {
		return
	}
	c.pos = clamp(pos, listLen-1)
	c.ensureVisible(listLen, height)
}

// JumpStart moves cursor to position 0 and resets offset.
func (c *Cursor) JumpStart() {
	c.pos = 0
	c.offset = 0
}

// JumpEnd moves cursor to the last position and adjusts offset.
func (c *Cursor) JumpEnd(listLen, height int) {
	if listLen == 0 {
		return
	}
	c.pos = listLen - 1
	c.ensureVisible(listLen, height)
}

// EnsureVisible adjusts the scroll offset to keep the cursor visible.
// This should be called after external cursor position changes.
func (c *Cursor) EnsureVisible(listLen, height int) {
	c.ensureVisible(listLen, height)
}

// ensureVisible is the internal implementation.
func (c *Cursor) ensureVisible(listLen, height int) {
	if height <= 0 || listLen == 0 {
		return
	}

	// Scroll up: cursor too close to top
	if c.pos < c.offset+c.margin {
		c.offset = max(c.pos-c.margin, 0)
	}

	// Scroll down: cursor too close to bottom
	if c.pos >= c.offset+height-c.margin {
		c.offset = c.pos - height + c.margin + 1
	}

	// Clamp offset to valid range
	maxOffset := max(listLen-height, 0)
	c.offset = clamp(c.offset, maxOffset)
}

// Center centers the cursor in the viewport.
func (c *Cursor) Center(listLen, height int) {
	if height <= 0 || listLen == 0 {
		return
	}

	c.offset = max(c.pos-height/2, 0)
	maxOffset := max(listLen-height, 0)
	c.offset = min(c.offset, maxOffset)
}

// ClampToBounds ensures the cursor is within valid bounds for the given length.
// Useful when the list length decreases (items deleted).
// Returns true if the cursor was adjusted.
func (c *Cursor) ClampToBounds(listLen int) bool {
	if listLen == 0 {
		changed := c.pos != 0 || c.offset != 0
		c.pos = 0
		c.offset = 0
		return changed
	}

	oldPos := c.pos
	c.pos = clamp(c.pos, listLen-1)
	return c.pos != oldPos
}

// VisibleRange returns the range of visible indices [start, end).
// The end index is exclusive.
func (c Cursor) VisibleRange(listLen, height int) (start, end int) {
	if listLen == 0 || height <= 0 {
		return 0, 0
	}
	start = c.offset
	end = min(c.offset+height, listLen)
	return start, end
}

// Reset resets the cursor to position 0 and offset 0.
func (c *Cursor) Reset() {
	c.pos = 0
	c.offset = 0
}

// SetPos sets the cursor position directly without bounds checking.
// Use with caution - prefer Jump() for safe position changes.
// This is useful when restoring state from persistence.
func (c *Cursor) SetPos(pos int) {
	c.pos = pos
}

// SetOffset sets the scroll offset directly without bounds checking.
// Use with caution - prefer EnsureVisible() for safe offset changes.
// This is useful when restoring state from persistence.
func (c *Cursor) SetOffset(offset int) {
	c.offset = offset
}

// HandleKey handles common list navigation keys and returns true if the key was handled.
// Supported keys: j/down, k/up, g/home, G/end, ctrl+d (half page down), ctrl+u (half page up).
// The calling code should check the return value to determine if post-movement actions
// (like updating previews or emitting navigation changed events) are needed.
func (c *Cursor) HandleKey(key string, listLen, height int) bool {
	switch key {
	case "j", "down":
		c.Move(1, listLen, height)
		return true
	case "k", "up":
		c.Move(-1, listLen, height)
		return true
	case "g", "home":
		c.JumpStart()
		return true
	case "G", "end":
		c.JumpEnd(listLen, height)
		return true
	case "ctrl+d":
		c.Move(height/2, listLen, height)
		return true
	case "ctrl+u":
		c.Move(-height/2, listLen, height)
		return true
	}
	return false
}

func clamp(v, maxVal int) int {
	if v < 0 {
		return 0
	}
	if v > maxVal {
		return maxVal
	}
	return v
}
