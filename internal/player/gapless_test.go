package player

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockStreamer produces a fixed number of samples then returns ok=false.
type mockStreamer struct {
	samples   int
	sampleVal float64
	produced  int
}

func (m *mockStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	remaining := m.samples - m.produced
	if remaining <= 0 {
		return 0, false
	}
	toWrite := min(len(samples), remaining)
	for i := range toWrite {
		samples[i] = [2]float64{m.sampleVal, m.sampleVal}
	}
	m.produced += toWrite
	return toWrite, true
}

func (m *mockStreamer) Err() error { return nil }

func TestGaplessStreamer_BasicTransition(t *testing.T) {
	current := &mockStreamer{samples: 10, sampleVal: 1.0}
	next := &mockStreamer{samples: 10, sampleVal: 2.0}

	transitioned := false
	g := &gaplessStreamer{
		current:  current,
		onSwitch: func() { transitioned = true },
	}
	g.SetNext(next)

	// Read all samples in one go (20 total)
	buf := make([][2]float64, 25)
	n, ok := g.Stream(buf)

	assert.True(t, ok)
	assert.Equal(t, 20, n)
	assert.True(t, transitioned)

	// First 10 should be 1.0, next 10 should be 2.0
	for i := range 10 {
		assert.Equal(t, 1.0, buf[i][0], "sample %d should be from current", i)
	}
	for i := 10; i < 20; i++ {
		assert.Equal(t, 2.0, buf[i][0], "sample %d should be from next", i)
	}
}

func TestGaplessStreamer_NoNext(t *testing.T) {
	current := &mockStreamer{samples: 5, sampleVal: 1.0}

	g := &gaplessStreamer{current: current}

	buf := make([][2]float64, 10)
	n, ok := g.Stream(buf)

	assert.False(t, ok)
	assert.Equal(t, 5, n)
}

func TestGaplessStreamer_SetNextDuringPlayback(t *testing.T) {
	current := &mockStreamer{samples: 20, sampleVal: 1.0}
	next := &mockStreamer{samples: 10, sampleVal: 2.0}

	g := &gaplessStreamer{current: current}

	// Read some samples before setting next
	buf := make([][2]float64, 10)
	n, ok := g.Stream(buf)
	assert.True(t, ok)
	assert.Equal(t, 10, n)

	// Now set next
	g.SetNext(next)

	// Read remaining - should transition
	buf2 := make([][2]float64, 25)
	n, ok = g.Stream(buf2)
	assert.True(t, ok)
	assert.Equal(t, 20, n) // 10 from current + 10 from next
}
