package playlists

import "sort"

// positionCalculator calculates position shifts for moving tracks in a playlist.
// It separates the pure position calculation logic from database operations.
type positionCalculator struct {
	sorted []int // sorted positions to move
	count  int   // total track count
	delta  int   // movement amount (negative = up, positive = down)
}

// newPositionCalculator creates a calculator for moving positions by delta.
func newPositionCalculator(positions []int, count, delta int) *positionCalculator {
	sorted := make([]int, len(positions))
	copy(sorted, positions)
	sort.Ints(sorted)
	return &positionCalculator{sorted: sorted, count: count, delta: delta}
}

// canMove returns true if the move is valid (within bounds).
// Returns false if there are no positions to move, delta is zero,
// or the move would go out of bounds.
func (c *positionCalculator) canMove() bool {
	if len(c.sorted) == 0 || c.delta == 0 {
		return false
	}
	if c.delta < 0 {
		return c.sorted[0]+c.delta >= 0
	}
	return c.sorted[len(c.sorted)-1]+c.delta < c.count
}

// newPositions returns the new positions after the move.
// The input should be the original (unsorted) positions array.
func (c *positionCalculator) newPositions(originalPositions []int) []int {
	result := make([]int, len(originalPositions))
	for i, pos := range originalPositions {
		result[i] = pos + c.delta
	}
	return result
}

// shiftRange represents a range of positions to shift.
type shiftRange struct {
	start int // inclusive start position
	end   int // exclusive end position
	delta int // amount to shift (+1 or -1)
}

// shiftRanges returns the ranges that need to be shifted to make room for the moved tracks.
// Each range represents non-selected tracks that need their position adjusted.
// When moving up (delta < 0): ranges shift down by +1
// When moving down (delta > 0): ranges shift up by -1
func (c *positionCalculator) shiftRanges() []shiftRange {
	if !c.canMove() {
		return nil
	}

	var ranges []shiftRange
	if c.delta < 0 {
		// Moving up: shift tracks in [newPos, oldPos) down by +1
		for _, pos := range c.sorted {
			newPos := pos + c.delta
			ranges = append(ranges, shiftRange{start: newPos, end: pos, delta: 1})
		}
	} else {
		// Moving down: shift tracks in (oldPos, newPos] up by -1
		// Process in reverse order to maintain consistency
		for i := len(c.sorted) - 1; i >= 0; i-- {
			pos := c.sorted[i]
			newPos := pos + c.delta
			ranges = append(ranges, shiftRange{start: pos + 1, end: newPos + 1, delta: -1})
		}
	}
	return ranges
}

// sortedPositions returns the sorted positions for iteration.
func (c *positionCalculator) sortedPositions() []int {
	return c.sorted
}
