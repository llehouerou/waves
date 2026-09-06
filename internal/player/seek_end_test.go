package player

import (
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/stretchr/testify/assert"
)

// seekableMock is a mockStreamer with a seekable position, standing in for a
// decoded track.
type seekableMock struct {
	mockStreamer
}

func (s *seekableMock) Len() int { return s.samples }

func (s *seekableMock) Position() int { return s.produced }

func (s *seekableMock) Seek(p int) error {
	s.produced = p
	return nil
}

func (s *seekableMock) Close() error { return nil }

func playingPlayer(streamer beep.StreamSeekCloser) *Player {
	p := New()
	p.current = &trackState{
		streamer:  streamer,
		resampled: streamer,
		format:    beep.Format{SampleRate: 44100, NumChannels: 2, Precision: 2},
	}
	p.volume = &effects.Volume{Streamer: streamer}
	p.state = Playing
	return p
}

// Seeking past the end must end the track once: one finished signal, and a
// player that is no longer playing the old track. Issue #38: it used to signal
// while leaving the track playing, so the consumer advanced the queue, skipped
// the next track, and advanced again at the real end.
func TestSeekPastEndEndsTheTrack(t *testing.T) {
	streamer := &seekableMock{mockStreamer{samples: 44100}} // 1 second
	p := playingPlayer(streamer)

	p.doSeek(10 * time.Second)

	select {
	case <-p.FinishedChan():
	default:
		t.Fatal("no finished signal after seeking past the end")
	}

	assert.NotEqual(t, Playing, p.State(),
		"player still reports Playing, so the consumer treats it as a gapless switch and skips the next track")

	// The old track must not produce audio any more, and must not fire a
	// second finished signal later.
	assert.Nil(t, p.current, "current track still attached after seeking past the end")

	select {
	case <-p.FinishedChan():
		t.Fatal("second finished signal: the queue would advance twice")
	default:
	}
}
