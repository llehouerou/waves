package playback

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestNewSubscription_ChannelsReadable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sub := newSubscription()

		// Send events
		sub.sendState(StateChange{Previous: StateStopped, Current: StatePlaying})
		sub.sendTrack(TrackChange{Index: 1})
		sub.sendPosition(30 * time.Second)
		sub.sendQueue(QueueChange{Index: 2, Tracks: []Track{{Path: "/test/queue.mp3"}}})
		sub.sendMode(ModeChange{RepeatMode: RepeatAll, Shuffle: true})

		// Receive events - synctest détecte automatiquement les deadlocks
		e := <-sub.StateChanged
		if e.Current != StatePlaying {
			t.Errorf("StateChanged.Current = %v, want Playing", e.Current)
		}

		tr := <-sub.TrackChanged
		if tr.Index != 1 {
			t.Errorf("TrackChanged.Index = %d, want 1", tr.Index)
		}

		pos := <-sub.PositionChanged
		if pos.Position != 30*time.Second {
			t.Errorf("PositionChanged.Position = %v, want 30s", pos.Position)
		}

		q := <-sub.QueueChanged
		if q.Index != 2 {
			t.Errorf("QueueChanged.Index = %d, want 2", q.Index)
		}
		if len(q.Tracks) != 1 || q.Tracks[0].Path != "/test/queue.mp3" {
			t.Errorf("QueueChanged.Tracks = %v, want [{Path: /test/queue.mp3}]", q.Tracks)
		}

		m := <-sub.ModeChanged
		if m.RepeatMode != RepeatAll {
			t.Errorf("ModeChanged.RepeatMode = %v, want RepeatAll", m.RepeatMode)
		}
	})
}

func TestSubscription_Close_SignalsDone(t *testing.T) {
	synctest.Test(t, func(_ *testing.T) {
		sub := newSubscription()
		sub.close()
		<-sub.Done
	})
}

func TestSubscription_NonBlocking_DropsWhenFull(t *testing.T) {
	sub := newSubscription()

	// Fill buffer
	for range eventBufferSize + 5 {
		sub.sendState(StateChange{})
	}

	// Should not block or panic - count what we got
	count := 0
	for {
		select {
		case <-sub.StateChanged:
			count++
		default:
			goto done
		}
	}
done:
	if count != eventBufferSize {
		t.Errorf("received %d events, want %d (buffer size)", count, eventBufferSize)
	}
}
