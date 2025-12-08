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
	speaker.Lock()
	pos := p.format.SampleRate.D(p.streamer.Position())
	speaker.Unlock()
	return pos
}

// Seek moves the playback position by the given delta.
// If seeking past the end, the player stops.
func (p *Player) Seek(delta time.Duration) {
	if p.streamer == nil || p.state == Stopped || p.volume == nil {
		return
	}

	speaker.Lock()
	currentPos := p.streamer.Position()
	newPos := currentPos + p.format.SampleRate.N(delta)
	maxPos := p.streamer.Len()

	// Stop if seeking past the end
	if newPos >= maxPos {
		speaker.Unlock()
		p.Stop()
		if p.onFinished != nil {
			go p.onFinished()
		}
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

	speaker.Lock()
	p.volume.Silent = false
	speaker.Unlock()
}
