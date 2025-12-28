package playback

import (
	"testing"
	"time"
)

func TestNewSubscription_ChannelsReadable(t *testing.T) {
	sub := newSubscription()

	// Send events
	sub.sendState(StateChange{Previous: StateStopped, Current: StatePlaying})
	sub.sendTrack(TrackChange{Index: 1})
	sub.sendPosition(30 * time.Second)
	sub.sendQueue(QueueChange{Index: 2, Tracks: []Track{{Path: "/test/queue.mp3"}}})
	sub.sendMode(ModeChange{RepeatMode: RepeatAll, Shuffle: true})

	// Receive events
	select {
	case e := <-sub.StateChanged:
		if e.Current != StatePlaying {
			t.Errorf("StateChanged.Current = %v, want Playing", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged")
	}

	select {
	case e := <-sub.TrackChanged:
		if e.Index != 1 {
			t.Errorf("TrackChanged.Index = %d, want 1", e.Index)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for TrackChanged")
	}

	select {
	case e := <-sub.PositionChanged:
		if e.Position != 30*time.Second {
			t.Errorf("PositionChanged.Position = %v, want 30s", e.Position)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for PositionChanged")
	}

	select {
	case e := <-sub.QueueChanged:
		if e.Index != 2 {
			t.Errorf("QueueChanged.Index = %d, want 2", e.Index)
		}
		if len(e.Tracks) != 1 || e.Tracks[0].Path != "/test/queue.mp3" {
			t.Errorf("QueueChanged.Tracks = %v, want [{Path: /test/queue.mp3}]", e.Tracks)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for QueueChanged")
	}

	select {
	case e := <-sub.ModeChanged:
		if e.RepeatMode != RepeatAll {
			t.Errorf("ModeChanged.RepeatMode = %v, want RepeatAll", e.RepeatMode)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ModeChanged")
	}
}

func TestSubscription_Close_SignalsDone(t *testing.T) {
	sub := newSubscription()
	sub.close()

	select {
	case <-sub.Done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for Done")
	}
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
