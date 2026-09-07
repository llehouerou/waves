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

	// Stop monitor loop
	if p.monitorDone != nil {
		close(p.monitorDone)
		p.monitorDone = nil
	}

	speaker.Clear()

	// Clean up current track
	if p.current != nil {
		p.current.Close()
		p.current = nil
	}

	// Clean up pre-loaded next track
	if p.next != nil {
		p.next.Close()
		p.next = nil
	}

	p.gapless = nil
	p.ctrl = nil
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
	if p.current == nil || p.current.streamer == nil {
		return 0
	}
	// Read position without lock - may be slightly stale but avoids deadlocks.
	// The streamer.Position() is typically safe for concurrent read.
	return p.current.format.SampleRate.D(p.current.streamer.Position())
}

// seekBurst is how long a seek keeps accumulating onto the previous target.
// Longer than seekTo's mute, so presses during a seek still accumulate.
const seekBurst = 700 * time.Millisecond

// Seek moves the playback position by the given delta.
// Non-blocking: deltas accumulate onto a target the seek loop picks up, so a
// burst of presses becomes one jump and +5s/-5s is neutral (issue #42).
func (p *Player) Seek(delta time.Duration) {
	if p.current == nil || p.current.streamer == nil || p.state == Stopped {
		return
	}

	now := time.Now()
	p.seekMu.Lock()
	if now.Sub(p.lastSeek) > seekBurst || p.seekTrack != p.current {
		// Burst is over, or the track changed under it: start from where
		// playback actually is.
		p.seekTarget = p.Position()
		p.seekTrack = p.current
	}
	p.lastSeek = now
	p.seekTarget = min(max(p.seekTarget+delta, 0), p.streamerLen())
	p.seekMu.Unlock()

	select {
	case p.seekWake <- struct{}{}:
	default: // already awake, it will read the latest target
	}
}

// streamerLen is the decoded length of the current track. Unlike Duration() it
// does not depend on tag metadata, which the seek clamp cannot trust.
func (p *Player) streamerLen() time.Duration {
	if p.current == nil || p.current.streamer == nil {
		return 0
	}
	return p.current.format.SampleRate.D(p.current.streamer.Len())
}

// pendingSeek returns the accumulated target, and whether it still applies. A
// target belongs to the track it was computed against: seeking past the end
// stops that track and the queue moves on, and applying the old target to the
// fresh track would seek past its end too and stop it dead.
func (p *Player) pendingSeek() (time.Duration, bool) {
	p.seekMu.Lock()
	defer p.seekMu.Unlock()
	if p.seekTrack == nil || p.seekTrack != p.current {
		return 0, false
	}
	return p.seekTarget, true
}

// seekLoop seeks to the accumulated target whenever one is pending.
func (p *Player) seekLoop() {
	for range p.seekWake {
		if target, ok := p.pendingSeek(); ok {
			p.seekTo(target)
		}
	}
}

// seekTo performs the actual seek to an absolute position.
func (p *Player) seekTo(target time.Duration) {
	// Quick check without lock - if already stopped, skip entirely
	if p.current == nil || p.current.streamer == nil || p.state == Stopped || p.volume == nil {
		return
	}

	// Check position without holding the lock to avoid deadlocks
	streamer := p.current.streamer
	if streamer == nil {
		return
	}
	maxPos := streamer.Len()
	newPos := p.current.format.SampleRate.N(target)

	// Seeking past the end ends the track. Stop first: a consumer that sees the
	// player still Playing takes it for a gapless transition the player already
	// made, and skips the next track (issue #38). This runs before the
	// speaker.Lock() below, so it cannot deadlock the way it did before 4c008e8.
	if newPos >= maxPos {
		p.Stop()
		select {
		case p.finishedCh <- struct{}{}:
		default:
		}
		return
	}

	// Now acquire lock for the actual seek
	speaker.Lock()
	// Re-check under lock in case Stop() was called
	if p.current == nil || p.current.streamer == nil || p.state == Stopped || p.volume == nil {
		speaker.Unlock()
		return
	}

	// Clamp to valid range
	newPos = max(newPos, 0)

	// Mute, seek, then unmute to avoid audio artifacts
	p.volume.Silent = true
	_ = p.current.streamer.Seek(newPos)
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
