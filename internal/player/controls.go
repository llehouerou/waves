package player

import (
	"time"

	"github.com/gopxl/beep/v2/speaker"
)

// Stop stops playback and releases resources.
func (p *Player) Stop() {
	if p.state == Stopped {
		return
	}

	speaker.Clear()

	if p.streamer != nil {
		p.streamer.Close()
		p.streamer = nil
	}
	if p.file != nil {
		p.file.Close()
		p.file = nil
	}

	p.ctrl = nil
	p.trackInfo = nil
	p.state = Stopped

	// Close done channel to unblock any waiters (safe to close already-closed channel
	// is NOT safe in Go, so we use a select to check if it's already closed)
	select {
	case <-p.done:
		// Already closed
	default:
		close(p.done)
	}
}

// Pause pauses playback.
func (p *Player) Pause() {
	if p.state != Playing || p.ctrl == nil {
		return
	}
	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
	p.state = Paused
}

// Resume resumes paused playback.
func (p *Player) Resume() {
	if p.state != Paused || p.ctrl == nil {
		return
	}
	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()
	p.state = Playing
}

// Toggle toggles between playing and paused states.
func (p *Player) Toggle() {
	switch p.state {
	case Playing:
		p.Pause()
	case Paused:
		p.Resume()
	case Stopped:
		// Nothing to toggle when stopped
	}
}

// Position returns the current playback position.
func (p *Player) Position() time.Duration {
	if p.streamer == nil {
		return 0
	}
	// Read position without lock - may be slightly stale but avoids deadlocks.
	// The streamer.Position() is typically safe for concurrent read.
	return p.format.SampleRate.D(p.streamer.Position())
}

// Seek moves the playback position by the given delta.
// Non-blocking: sends to a channel, dropping old requests if one is pending.
func (p *Player) Seek(delta time.Duration) {
	if p.streamer == nil || p.state == Stopped {
		return
	}

	// Non-blocking send - drop if channel full (previous seek pending)
	select {
	case p.seekChan <- delta:
	default:
		// Channel full, drain and send new value
		select {
		case <-p.seekChan:
		default:
		}
		select {
		case p.seekChan <- delta:
		default:
		}
	}
}

// seekLoop processes seek requests sequentially.
// Only the most recent seek is processed, older ones are dropped.
func (p *Player) seekLoop() {
	for delta := range p.seekChan {
		p.doSeek(delta)
	}
}

// doSeek performs the actual seek operation.
func (p *Player) doSeek(delta time.Duration) {
	// Quick check without lock - if already stopped, skip entirely
	if p.streamer == nil || p.state == Stopped || p.volume == nil {
		return
	}

	// Check position without holding the lock to avoid deadlocks
	streamer := p.streamer
	if streamer == nil {
		return
	}
	currentPos := streamer.Position()
	maxPos := streamer.Len()
	newPos := currentPos + p.format.SampleRate.N(delta)

	// If seeking past the end, signal track finished
	if newPos >= maxPos {
		// Signal finish via the channel (non-blocking)
		select {
		case p.finishedCh <- struct{}{}:
		default:
		}
		return
	}

	// Now acquire lock for the actual seek
	speaker.Lock()
	// Re-check under lock in case Stop() was called
	if p.streamer == nil || p.state == Stopped || p.volume == nil {
		speaker.Unlock()
		return
	}

	// Clamp to valid range
	newPos = max(newPos, 0)

	// Mute, seek, then unmute to avoid audio artifacts
	p.volume.Silent = true
	_ = p.streamer.Seek(newPos)
	speaker.Unlock()

	// Brief pause to let buffer clear before unmuting
	time.Sleep(100 * time.Millisecond)

	// Re-check state after sleep - track may have stopped or changed
	if p.volume == nil || p.state == Stopped {
		return
	}

	speaker.Lock()
	if p.volume != nil {
		p.volume.Silent = false
	}
	speaker.Unlock()
}
