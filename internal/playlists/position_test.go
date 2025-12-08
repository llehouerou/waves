package playlists

import (
	"reflect"
	"testing"
)

func TestPositionCalculator_canMove(t *testing.T) {
	tests := []struct {
		name      string
		positions []int
		count     int
		delta     int
		want      bool
	}{
		{
			name:      "empty positions",
			positions: []int{},
			count:     5,
			delta:     1,
			want:      false,
		},
		{
			name:      "zero delta",
			positions: []int{1, 2},
			count:     5,
			delta:     0,
			want:      false,
		},
		{
			name:      "move up valid",
			positions: []int{2, 3},
			count:     5,
			delta:     -1,
			want:      true,
		},
		{
			name:      "move up at boundary",
			positions: []int{0, 1},
			count:     5,
			delta:     -1,
			want:      false,
		},
		{
			name:      "move down valid",
			positions: []int{1, 2},
			count:     5,
			delta:     1,
			want:      true,
		},
		{
			name:      "move down at boundary",
			positions: []int{3, 4},
			count:     5,
			delta:     1,
			want:      false,
		},
		{
			name:      "move up unsorted positions",
			positions: []int{3, 1, 2},
			count:     5,
			delta:     -1,
			want:      true,
		},
		{
			name:      "single position move up",
			positions: []int{2},
			count:     5,
			delta:     -2,
			want:      true,
		},
		{
			name:      "single position move down",
			positions: []int{2},
			count:     5,
			delta:     2,
			want:      true,
		},
		{
			name:      "single position at start cannot move up",
			positions: []int{0},
			count:     5,
			delta:     -1,
			want:      false,
		},
		{
			name:      "single position at end cannot move down",
			positions: []int{4},
			count:     5,
			delta:     1,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := newPositionCalculator(tt.positions, tt.count, tt.delta)
			if got := calc.canMove(); got != tt.want {
				t.Errorf("canMove() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPositionCalculator_newPositions(t *testing.T) {
	tests := []struct {
		name      string
		positions []int
		count     int
		delta     int
		want      []int
	}{
		{
			name:      "move up by 1",
			positions: []int{2, 3, 4},
			count:     5,
			delta:     -1,
			want:      []int{1, 2, 3},
		},
		{
			name:      "move down by 1",
			positions: []int{0, 1, 2},
			count:     5,
			delta:     1,
			want:      []int{1, 2, 3},
		},
		{
			name:      "preserves original order",
			positions: []int{4, 2, 3},
			count:     5,
			delta:     -1,
			want:      []int{3, 1, 2},
		},
		{
			name:      "move by 2",
			positions: []int{3, 4},
			count:     7,
			delta:     -2,
			want:      []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := newPositionCalculator(tt.positions, tt.count, tt.delta)
			got := calc.newPositions(tt.positions)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newPositions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPositionCalculator_sortedPositions(t *testing.T) {
	tests := []struct {
		name      string
		positions []int
		want      []int
	}{
		{
			name:      "already sorted",
			positions: []int{1, 2, 3},
			want:      []int{1, 2, 3},
		},
		{
			name:      "unsorted",
			positions: []int{3, 1, 2},
			want:      []int{1, 2, 3},
		},
		{
			name:      "reverse order",
			positions: []int{5, 4, 3, 2, 1},
			want:      []int{1, 2, 3, 4, 5},
		},
		{
			name:      "single element",
			positions: []int{3},
			want:      []int{3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := newPositionCalculator(tt.positions, 10, 1)
			got := calc.sortedPositions()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortedPositions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPositionCalculator_shiftRanges(t *testing.T) {
	tests := []struct {
		name      string
		positions []int
		count     int
		delta     int
		want      []shiftRange
	}{
		{
			name:      "cannot move returns nil",
			positions: []int{0},
			count:     5,
			delta:     -1,
			want:      nil,
		},
		{
			name:      "move up single position",
			positions: []int{2},
			count:     5,
			delta:     -1,
			want:      []shiftRange{{start: 1, end: 2, delta: 1}},
		},
		{
			name:      "move up multiple positions",
			positions: []int{2, 3},
			count:     5,
			delta:     -1,
			want: []shiftRange{
				{start: 1, end: 2, delta: 1},
				{start: 2, end: 3, delta: 1},
			},
		},
		{
			name:      "move down single position",
			positions: []int{2},
			count:     5,
			delta:     1,
			want:      []shiftRange{{start: 3, end: 4, delta: -1}},
		},
		{
			name:      "move down multiple positions",
			positions: []int{1, 2},
			count:     5,
			delta:     1,
			want: []shiftRange{
				{start: 3, end: 4, delta: -1},
				{start: 2, end: 3, delta: -1},
			},
		},
		{
			name:      "move up by 2",
			positions: []int{3},
			count:     5,
			delta:     -2,
			want:      []shiftRange{{start: 1, end: 3, delta: 1}},
		},
		{
			name:      "move down by 2",
			positions: []int{1},
			count:     5,
			delta:     2,
			want:      []shiftRange{{start: 2, end: 4, delta: -1}},
		},
		{
			name:      "move up unsorted positions",
			positions: []int{4, 2},
			count:     6,
			delta:     -1,
			want: []shiftRange{
				{start: 1, end: 2, delta: 1},
				{start: 3, end: 4, delta: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := newPositionCalculator(tt.positions, tt.count, tt.delta)
			got := calc.shiftRanges()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shiftRanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPositionCalculator_doesNotMutateInput(t *testing.T) {
	original := []int{3, 1, 2}
	positions := make([]int, len(original))
	copy(positions, original)

	calc := newPositionCalculator(positions, 5, -1)
	calc.sortedPositions()

	if !reflect.DeepEqual(positions, original) {
		t.Errorf("input positions were mutated: got %v, want %v", positions, original)
	}
}
