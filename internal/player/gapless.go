package player

import (
	"sync"

	"github.com/gopxl/beep/v2"
)

var _ beep.Streamer = (*gaplessStreamer)(nil)

// gaplessStreamer wraps a streamer and allows seamless transition to a next streamer.
type gaplessStreamer struct {
	mu       sync.Mutex
	current  beep.Streamer
	next     beep.Streamer
	onSwitch func() // Called when transitioning to next
}

// Stream implements beep.Streamer.
func (g *gaplessStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	n, ok = g.current.Stream(samples)

	// If current didn't fill the buffer, check if it's exhausted
	if n < len(samples) && ok {
		// Try to get more from current to see if it's truly exhausted
		n2, ok2 := g.current.Stream(samples[n:])
		n += n2
		ok = ok2
	}

	// If current is exhausted and we have a next, switch to it
	if !ok && g.next != nil {
		if g.onSwitch != nil {
			g.onSwitch()
		}
		g.current = g.next
		g.next = nil

		// Fill remaining buffer from next track
		if n < len(samples) {
			n2, ok2 := g.current.Stream(samples[n:])
			n += n2
			ok = ok2
		} else {
			ok = true
		}
	}

	return n, ok
}

// Err implements beep.Streamer.
func (g *gaplessStreamer) Err() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.current != nil {
		return g.current.Err()
	}
	return nil
}

// SetNext sets the next streamer to transition to.
func (g *gaplessStreamer) SetNext(s beep.Streamer) {
	g.mu.Lock()
	g.next = s
	g.mu.Unlock()
}

// ClearNext removes the queued next streamer.
func (g *gaplessStreamer) ClearNext() {
	g.mu.Lock()
	g.next = nil
	g.mu.Unlock()
}

// HasNext returns true if a next streamer is queued.
func (g *gaplessStreamer) HasNext() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.next != nil
}
