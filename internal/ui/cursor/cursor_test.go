package cursor

import "testing"

func TestNew(t *testing.T) {
	c := New(5)
	if c.Pos() != 0 {
		t.Errorf("New() pos = %d, want 0", c.Pos())
	}
	if c.Offset() != 0 {
		t.Errorf("New() offset = %d, want 0", c.Offset())
	}
	if c.Margin() != 5 {
		t.Errorf("New() margin = %d, want 5", c.Margin())
	}
}

func TestMove(t *testing.T) {
	tests := []struct {
		name       string
		margin     int
		initial    int
		delta      int
		len        int
		height     int
		wantPos    int
		wantOffset int
	}{
		{
			name:       "move down within bounds no scroll",
			margin:     2,
			initial:    0,
			delta:      1,
			len:        10,
			height:     5,
			wantPos:    1,
			wantOffset: 0,
		},
		{
			name:       "move down triggers scroll with margin",
			margin:     2,
			initial:    0,
			delta:      3,
			len:        10,
			height:     5,
			wantPos:    3,
			wantOffset: 1,
		},
		{
			name:       "move up clamps to 0",
			margin:     2,
			initial:    2,
			delta:      -5,
			len:        10,
			height:     5,
			wantPos:    0,
			wantOffset: 0,
		},
		{
			name:       "move down clamps to len-1",
			margin:     2,
			initial:    5,
			delta:      15,
			len:        10,
			height:     5,
			wantPos:    9,
			wantOffset: 5,
		},
		{
			name:       "move triggers scroll down",
			margin:     2,
			initial:    2,
			delta:      3,
			len:        10,
			height:     5,
			wantPos:    5,
			wantOffset: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.margin)
			c.pos = tt.initial
			c.Move(tt.delta, tt.len, tt.height)
			if c.Pos() != tt.wantPos {
				t.Errorf("Move() pos = %d, want %d", c.Pos(), tt.wantPos)
			}
			if c.Offset() != tt.wantOffset {
				t.Errorf("Move() offset = %d, want %d", c.Offset(), tt.wantOffset)
			}
		})
	}
}

func TestMoveEmptyList(t *testing.T) {
	c := New(2)
	c.pos = 5 // Set to non-zero to verify no change
	c.Move(1, 0, 5)
	if c.Pos() != 5 {
		t.Errorf("Move() on empty list changed pos to %d", c.Pos())
	}
}

func TestJump(t *testing.T) {
	c := New(2)
	c.Jump(5, 10, 5)
	if c.Pos() != 5 {
		t.Errorf("Jump() pos = %d, want 5", c.Pos())
	}

	// Jump beyond bounds
	c.Jump(100, 10, 5)
	if c.Pos() != 9 {
		t.Errorf("Jump() pos = %d, want 9 (clamped)", c.Pos())
	}

	// Jump negative
	c.Jump(-5, 10, 5)
	if c.Pos() != 0 {
		t.Errorf("Jump() pos = %d, want 0 (clamped)", c.Pos())
	}
}

func TestJumpStart(t *testing.T) {
	c := New(2)
	c.pos = 5
	c.offset = 3
	c.JumpStart()
	if c.Pos() != 0 {
		t.Errorf("JumpStart() pos = %d, want 0", c.Pos())
	}
	if c.Offset() != 0 {
		t.Errorf("JumpStart() offset = %d, want 0", c.Offset())
	}
}

func TestJumpEnd(t *testing.T) {
	c := New(2)
	c.JumpEnd(10, 5)
	if c.Pos() != 9 {
		t.Errorf("JumpEnd() pos = %d, want 9", c.Pos())
	}

	// Empty list
	c2 := New(2)
	c2.JumpEnd(0, 5)
	if c2.Pos() != 0 {
		t.Errorf("JumpEnd() on empty list pos = %d, want 0", c2.Pos())
	}
}

func TestEnsureVisible(t *testing.T) {
	tests := []struct {
		name       string
		margin     int
		pos        int
		offset     int
		len        int
		height     int
		wantOffset int
	}{
		{
			name:       "cursor in view no change",
			margin:     2,
			pos:        5,
			offset:     3,
			len:        10,
			height:     5,
			wantOffset: 3,
		},
		{
			name:       "cursor above view scrolls up",
			margin:     2,
			pos:        1,
			offset:     5,
			len:        10,
			height:     5,
			wantOffset: 0,
		},
		{
			name:       "cursor below view scrolls down",
			margin:     2,
			pos:        8,
			offset:     0,
			len:        10,
			height:     5,
			wantOffset: 5,
		},
		{
			name:       "tight scrolling no margin",
			margin:     0,
			pos:        4,
			offset:     0,
			len:        10,
			height:     5,
			wantOffset: 0,
		},
		{
			name:       "tight scrolling needs scroll",
			margin:     0,
			pos:        5,
			offset:     0,
			len:        10,
			height:     5,
			wantOffset: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.margin)
			c.pos = tt.pos
			c.offset = tt.offset
			c.EnsureVisible(tt.len, tt.height)
			if c.Offset() != tt.wantOffset {
				t.Errorf("EnsureVisible() offset = %d, want %d", c.Offset(), tt.wantOffset)
			}
		})
	}
}

func TestCenter(t *testing.T) {
	tests := []struct {
		name       string
		pos        int
		len        int
		height     int
		wantOffset int
	}{
		{
			name:       "center in middle",
			pos:        5,
			len:        10,
			height:     5,
			wantOffset: 3,
		},
		{
			name:       "center near start",
			pos:        1,
			len:        10,
			height:     5,
			wantOffset: 0,
		},
		{
			name:       "center near end",
			pos:        9,
			len:        10,
			height:     5,
			wantOffset: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(2)
			c.pos = tt.pos
			c.Center(tt.len, tt.height)
			if c.Offset() != tt.wantOffset {
				t.Errorf("Center() offset = %d, want %d", c.Offset(), tt.wantOffset)
			}
		})
	}
}

func TestClampToBounds(t *testing.T) {
	tests := []struct {
		name        string
		pos         int
		offset      int
		len         int
		wantChanged bool
		wantPos     int
		wantOffset  int
	}{
		{
			name:        "in bounds no change",
			pos:         3,
			offset:      0,
			len:         10,
			wantChanged: false,
			wantPos:     3,
			wantOffset:  0,
		},
		{
			name:        "pos exceeds len",
			pos:         8,
			offset:      5,
			len:         5,
			wantChanged: true,
			wantPos:     4,
			wantOffset:  5,
		},
		{
			name:        "empty list",
			pos:         5,
			offset:      3,
			len:         0,
			wantChanged: true,
			wantPos:     0,
			wantOffset:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(2)
			c.pos = tt.pos
			c.offset = tt.offset
			changed := c.ClampToBounds(tt.len)
			if changed != tt.wantChanged {
				t.Errorf("ClampToBounds() changed = %v, want %v", changed, tt.wantChanged)
			}
			if c.Pos() != tt.wantPos {
				t.Errorf("ClampToBounds() pos = %d, want %d", c.Pos(), tt.wantPos)
			}
			if c.Offset() != tt.wantOffset {
				t.Errorf("ClampToBounds() offset = %d, want %d", c.Offset(), tt.wantOffset)
			}
		})
	}
}

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		name      string
		offset    int
		len       int
		height    int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "normal range",
			offset:    2,
			len:       10,
			height:    5,
			wantStart: 2,
			wantEnd:   7,
		},
		{
			name:      "at end of list",
			offset:    7,
			len:       10,
			height:    5,
			wantStart: 7,
			wantEnd:   10,
		},
		{
			name:      "empty list",
			offset:    0,
			len:       0,
			height:    5,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "zero height",
			offset:    0,
			len:       10,
			height:    0,
			wantStart: 0,
			wantEnd:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(2)
			c.offset = tt.offset
			start, end := c.VisibleRange(tt.len, tt.height)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("VisibleRange() = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestReset(t *testing.T) {
	c := New(2)
	c.pos = 5
	c.offset = 3
	c.Reset()
	if c.Pos() != 0 {
		t.Errorf("Reset() pos = %d, want 0", c.Pos())
	}
	if c.Offset() != 0 {
		t.Errorf("Reset() offset = %d, want 0", c.Offset())
	}
}

func TestSetPosOffset(t *testing.T) {
	c := New(2)
	c.SetPos(7)
	c.SetOffset(4)
	if c.Pos() != 7 {
		t.Errorf("SetPos() pos = %d, want 7", c.Pos())
	}
	if c.Offset() != 4 {
		t.Errorf("SetOffset() offset = %d, want 4", c.Offset())
	}
}

func TestSetMargin(t *testing.T) {
	c := New(2)
	c.SetMargin(5)
	if c.Margin() != 5 {
		t.Errorf("SetMargin() margin = %d, want 5", c.Margin())
	}
}
