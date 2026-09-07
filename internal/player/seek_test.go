package player

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// seekTargetFor drains what Seek accumulated, without running the seek loop.
func seekTargetFor(t *testing.T, p *Player) time.Duration {
	t.Helper()
	select {
	case <-p.seekWake:
	default:
		t.Fatal("Seek did not wake the seek loop")
	}
	p.seekMu.Lock()
	defer p.seekMu.Unlock()
	return p.seekTarget
}

// seekPlayer is a player on a 5-minute track, positioned at `at`.
func seekPlayer(at time.Duration) (*Player, *seekableMock) {
	streamer := &seekableMock{mockStreamer{samples: 44100 * 300}}
	p := playingPlayer(streamer)
	streamer.produced = int(at.Seconds() * 44100)
	return p, streamer
}

// A burst of seeks must accumulate onto one target, not overwrite each other.
func TestSeekBurstAccumulates(t *testing.T) {
	p, _ := seekPlayer(time.Minute)

	p.Seek(5 * time.Second)
	p.Seek(5 * time.Second)
	p.Seek(5 * time.Second)

	assert.Equal(t, 75*time.Second, seekTargetFor(t, p),
		"three +5s presses should target one +15s jump")
}

// +5 then -5 must be exactly neutral, whatever playback did in between.
func TestSeekRoundTripIsNeutral(t *testing.T) {
	p, streamer := seekPlayer(time.Minute)

	p.Seek(5 * time.Second)
	streamer.produced += 44100 / 2 // half a second played meanwhile
	p.Seek(-5 * time.Second)

	assert.Equal(t, time.Minute, seekTargetFor(t, p),
		"+5s/-5s should land back on the position the burst started from")
}

// An isolated seek, long after the previous one, starts from where playback is.
func TestSeekAfterIdleUsesLivePosition(t *testing.T) {
	p, streamer := seekPlayer(time.Minute)

	p.Seek(5 * time.Second)
	<-p.seekWake
	p.seekMu.Lock()
	p.lastSeek = time.Now().Add(-10 * time.Second) // the burst is long over
	p.seekMu.Unlock()

	streamer.produced = int(90 * 44100) // playback moved on to 1:30
	p.Seek(5 * time.Second)

	assert.Equal(t, 95*time.Second, seekTargetFor(t, p))
}

func TestSeekTargetIsClamped(t *testing.T) {
	p, _ := seekPlayer(2 * time.Second)

	for range 5 {
		p.Seek(-5 * time.Second)
	}
	assert.Equal(t, time.Duration(0), seekTargetFor(t, p),
		"repeated backward seeks must not accumulate below zero")
}

// A burst must not carry a target across a track change.
func TestSeekTargetResetsOnTrackChange(t *testing.T) {
	p, _ := seekPlayer(time.Minute)

	p.Seek(5 * time.Second)
	<-p.seekWake

	// gapless transition: a new track takes over, playback at 0
	next := &seekableMock{mockStreamer{samples: 44100 * 300}}
	p.current = &trackState{streamer: next, resampled: next, format: p.current.format}

	p.Seek(5 * time.Second)
	assert.Equal(t, 5*time.Second, seekTargetFor(t, p),
		"the new track's seek must start from its own position")
}
