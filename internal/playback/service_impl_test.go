// internal/playback/service_impl_test.go
package playback

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

const (
	testSvcPathA     = "/a.mp3"
	testSvcPathB     = "/b.mp3"
	testSvcPathC     = "/c.mp3"
	testSvcMusicPath = "/music/song.mp3"
)

func TestNew_ReturnsService(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()

	svc := New(p, q)

	if svc == nil {
		t.Fatal("New() returned nil")
	}
}

func TestService_State_ReflectsPlayer(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	// Initially stopped
	if svc.State() != StateStopped {
		t.Errorf("State() = %v, want Stopped", svc.State())
	}

	// Set to playing
	p.SetState(player.Playing)
	if svc.State() != StatePlaying {
		t.Errorf("State() = %v, want Playing", svc.State())
	}

	// Set to paused
	p.SetState(player.Paused)
	if svc.State() != StatePaused {
		t.Errorf("State() = %v, want Paused", svc.State())
	}
}

func TestService_Position_ReflectsPlayer(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	p.SetPosition(30 * time.Second)

	if svc.Position() != 30*time.Second {
		t.Errorf("Position() = %v, want 30s", svc.Position())
	}
}

func TestService_Duration_ReflectsPlayer(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	p.SetDuration(3 * time.Minute)

	if svc.Duration() != 3*time.Minute {
		t.Errorf("Duration() = %v, want 3m", svc.Duration())
	}
}

func TestService_CurrentTrack_ReturnsNilWhenEmpty(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	if svc.CurrentTrack() != nil {
		t.Error("CurrentTrack() should be nil for empty queue")
	}
}

func TestService_CurrentTrack_ReturnsCopy(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{
		ID:    1,
		Path:  testSvcMusicPath,
		Title: "Test Song",
	})
	q.JumpTo(0)
	svc := New(p, q)

	track := svc.CurrentTrack()

	if track == nil {
		t.Fatal("CurrentTrack() returned nil")
	}
	if track.Path != testSvcMusicPath {
		t.Errorf("Path = %q, want %s", track.Path, testSvcMusicPath)
	}
	if track.Title != "Test Song" {
		t.Errorf("Title = %q, want Test Song", track.Title)
	}
}

func TestService_Queue_ReturnsCopy(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: testSvcPathA},
		playlist.Track{Path: testSvcPathB},
	)
	svc := New(p, q)

	tracks := svc.QueueTracks()

	if len(tracks) != 2 {
		t.Fatalf("len(Queue()) = %d, want 2", len(tracks))
	}
	if tracks[0].Path != testSvcPathA {
		t.Errorf("tracks[0].Path = %q, want /a.mp3", tracks[0].Path)
	}
	if tracks[1].Path != testSvcPathB {
		t.Errorf("tracks[1].Path = %q, want /b.mp3", tracks[1].Path)
	}
}

func TestService_QueueIndex_ReflectsQueue(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
	svc := New(p, q)

	if svc.QueueCurrentIndex() != -1 {
		t.Errorf("QueueIndex() = %d, want -1 (no current)", svc.QueueCurrentIndex())
	}

	q.JumpTo(1)
	if svc.QueueCurrentIndex() != 1 {
		t.Errorf("QueueIndex() = %d, want 1", svc.QueueCurrentIndex())
	}
}

func TestService_RepeatMode_ReflectsQueue(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	if svc.RepeatMode() != RepeatOff {
		t.Errorf("RepeatMode() = %v, want Off", svc.RepeatMode())
	}

	q.SetRepeatMode(playlist.RepeatAll)
	if svc.RepeatMode() != RepeatAll {
		t.Errorf("RepeatMode() = %v, want All", svc.RepeatMode())
	}
}

func TestService_Shuffle_ReflectsQueue(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	if svc.Shuffle() {
		t.Error("Shuffle() = true, want false")
	}

	q.SetShuffle(true)
	if !svc.Shuffle() {
		t.Error("Shuffle() = false, want true")
	}
}

func TestService_Subscribe_ReturnsSubscription(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	sub := svc.Subscribe()

	if sub == nil {
		t.Fatal("Subscribe() returned nil")
	}
	if sub.StateChanged == nil {
		t.Error("StateChanged channel is nil")
	}
}

func TestService_Close_SignalsSubscribers(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	err := svc.Close()

	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	select {
	case <-sub.Done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for Done")
	}
}

func TestService_Close_Idempotent(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	_ = svc.Close()
	err := svc.Close()

	if err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestService_Play_StartsPlayback(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcMusicPath})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	err := svc.Play()

	if err != nil {
		t.Fatalf("Play() error = %v", err)
	}
	if svc.State() != StatePlaying {
		t.Errorf("State() = %v, want Playing", svc.State())
	}
	if len(p.PlayCalls()) != 1 || p.PlayCalls()[0] != testSvcMusicPath {
		t.Errorf("PlayCalls() = %v, want [%s]", p.PlayCalls(), testSvcMusicPath)
	}

	// Verify StateChanged event was emitted
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StateStopped {
			t.Errorf("event.Previous = %v, want Stopped", e.Previous)
		}
		if e.Current != StatePlaying {
			t.Errorf("event.Current = %v, want Playing", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}
}

func TestService_Play_EmptyQueue_ReturnsError(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)

	err := svc.Play()

	if !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Play() error = %v, want ErrEmptyQueue", err)
	}
}

func TestService_Play_NoCurrentTrack_ReturnsError(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcMusicPath})
	// Don't call JumpTo, so current is nil
	svc := New(p, q)

	err := svc.Play()

	if !errors.Is(err, ErrNoCurrentTrack) {
		t.Errorf("Play() error = %v, want ErrNoCurrentTrack", err)
	}
}

func TestService_Pause_PausesPlayback(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcMusicPath})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing first
	_ = svc.Play()
	// Drain the Play event
	<-sub.StateChanged

	err := svc.Pause()

	if err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if svc.State() != StatePaused {
		t.Errorf("State() = %v, want Paused", svc.State())
	}

	// Verify StateChanged event was emitted
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StatePlaying {
			t.Errorf("event.Previous = %v, want Playing", e.Previous)
		}
		if e.Current != StatePaused {
			t.Errorf("event.Current = %v, want Paused", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}
}

func TestService_Pause_WhenStopped_NoOp(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	err := svc.Pause()

	if err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if svc.State() != StateStopped {
		t.Errorf("State() = %v, want Stopped", svc.State())
	}

	// Verify no StateChanged event was emitted
	select {
	case e := <-sub.StateChanged:
		t.Errorf("unexpected StateChanged event: %+v", e)
	case <-time.After(50 * time.Millisecond):
		// Expected - no event
	}
}

func TestService_Stop_StopsPlayback(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcMusicPath})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing first
	_ = svc.Play()
	// Drain the Play event
	<-sub.StateChanged

	err := svc.Stop()

	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if svc.State() != StateStopped {
		t.Errorf("State() = %v, want Stopped", svc.State())
	}

	// Verify StateChanged event was emitted
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StatePlaying {
			t.Errorf("event.Previous = %v, want Playing", e.Previous)
		}
		if e.Current != StateStopped {
			t.Errorf("event.Current = %v, want Stopped", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}
}

func TestService_Toggle_PlaysWhenStopped(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcMusicPath})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	err := svc.Toggle()

	if err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	if svc.State() != StatePlaying {
		t.Errorf("State() = %v, want Playing", svc.State())
	}

	// Verify StateChanged event was emitted
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StateStopped {
			t.Errorf("event.Previous = %v, want Stopped", e.Previous)
		}
		if e.Current != StatePlaying {
			t.Errorf("event.Current = %v, want Playing", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}
}

func TestService_Toggle_PausesWhenPlaying(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcMusicPath})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing first
	_ = svc.Play()
	// Drain the Play event
	<-sub.StateChanged

	err := svc.Toggle()

	if err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	if svc.State() != StatePaused {
		t.Errorf("State() = %v, want Paused", svc.State())
	}

	// Verify StateChanged event was emitted
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StatePlaying {
			t.Errorf("event.Previous = %v, want Playing", e.Previous)
		}
		if e.Current != StatePaused {
			t.Errorf("event.Current = %v, want Paused", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}
}

func TestService_Toggle_ResumesWhenPaused(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcMusicPath})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing and pause
	_ = svc.Play()
	<-sub.StateChanged
	_ = svc.Pause()
	<-sub.StateChanged

	err := svc.Toggle()

	if err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	if svc.State() != StatePlaying {
		t.Errorf("State() = %v, want Playing", svc.State())
	}

	// Verify StateChanged event was emitted
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StatePaused {
			t.Errorf("event.Previous = %v, want Paused", e.Previous)
		}
		if e.Current != StatePlaying {
			t.Errorf("event.Current = %v, want Playing", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}
}

func TestService_Next_AdvancesToNextTrack(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: testSvcPathA, Title: "Song A"},
		playlist.Track{Path: testSvcPathB, Title: "Song B"},
	)
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing first
	_ = svc.Play()
	<-sub.StateChanged

	err := svc.Next()

	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if svc.QueueCurrentIndex() != 1 {
		t.Errorf("QueueIndex() = %d, want 1", svc.QueueCurrentIndex())
	}

	// Verify TrackChanged event
	select {
	case e := <-sub.TrackChanged:
		if e.Previous == nil || e.Previous.Path != testSvcPathA {
			t.Errorf("event.Previous.Path = %v, want /a.mp3", e.Previous)
		}
		if e.Current == nil || e.Current.Path != testSvcPathB {
			t.Errorf("event.Current.Path = %v, want /b.mp3", e.Current)
		}
		if e.Index != 1 {
			t.Errorf("event.Index = %d, want 1", e.Index)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for TrackChanged event")
	}

	// Verify player.Play was called with new track
	calls := p.PlayCalls()
	if len(calls) != 2 || calls[1] != testSvcPathB {
		t.Errorf("PlayCalls() = %v, want [/a.mp3, /b.mp3]", calls)
	}
}

func TestService_Next_AtEnd_StopsPlayback(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing
	_ = svc.Play()
	<-sub.StateChanged

	err := svc.Next()

	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if svc.State() != StateStopped {
		t.Errorf("State() = %v, want Stopped", svc.State())
	}

	// Verify StateChanged event (from Playing to Stopped)
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StatePlaying {
			t.Errorf("event.Previous = %v, want Playing", e.Previous)
		}
		if e.Current != StateStopped {
			t.Errorf("event.Current = %v, want Stopped", e.Current)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}
}

func TestService_Next_WhenStopped_AdvancesWithoutPlaying(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: testSvcPathA},
		playlist.Track{Path: testSvcPathB},
	)
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Don't start playing - just call Next while stopped
	err := svc.Next()

	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if svc.QueueCurrentIndex() != 1 {
		t.Errorf("QueueIndex() = %d, want 1", svc.QueueCurrentIndex())
	}
	if svc.State() != StateStopped {
		t.Errorf("State() = %v, want Stopped", svc.State())
	}

	// Verify TrackChanged event was emitted
	select {
	case e := <-sub.TrackChanged:
		if e.Index != 1 {
			t.Errorf("event.Index = %d, want 1", e.Index)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for TrackChanged event")
	}

	// Verify player.Play was NOT called
	if len(p.PlayCalls()) != 0 {
		t.Errorf("PlayCalls() = %v, want empty", p.PlayCalls())
	}
}

func TestService_Previous_GoesToPreviousTrack(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: testSvcPathA},
		playlist.Track{Path: testSvcPathB},
	)
	q.JumpTo(1) // Start at second track
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing
	_ = svc.Play()
	<-sub.StateChanged

	err := svc.Previous()

	if err != nil {
		t.Fatalf("Previous() error = %v", err)
	}
	if svc.QueueCurrentIndex() != 0 {
		t.Errorf("QueueIndex() = %d, want 0", svc.QueueCurrentIndex())
	}

	// Verify TrackChanged event
	select {
	case e := <-sub.TrackChanged:
		if e.Previous == nil || e.Previous.Path != testSvcPathB {
			t.Errorf("event.Previous.Path = %v, want /b.mp3", e.Previous)
		}
		if e.Current == nil || e.Current.Path != testSvcPathA {
			t.Errorf("event.Current.Path = %v, want /a.mp3", e.Current)
		}
		if e.Index != 0 {
			t.Errorf("event.Index = %d, want 0", e.Index)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for TrackChanged event")
	}

	// Verify player.Play was called with new track
	calls := p.PlayCalls()
	if len(calls) != 2 || calls[1] != testSvcPathA {
		t.Errorf("PlayCalls() = %v, want [/b.mp3, /a.mp3]", calls)
	}
}

func TestService_Previous_AtStart_StaysAtStart(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA})
	q.JumpTo(0) // At start
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing
	_ = svc.Play()
	<-sub.StateChanged

	err := svc.Previous()

	if err != nil {
		t.Fatalf("Previous() error = %v", err)
	}
	if svc.QueueCurrentIndex() != 0 {
		t.Errorf("QueueIndex() = %d, want 0 (unchanged)", svc.QueueCurrentIndex())
	}

	// Verify no TrackChanged event (no-op)
	select {
	case e := <-sub.TrackChanged:
		t.Errorf("unexpected TrackChanged event: %+v", e)
	case <-time.After(50 * time.Millisecond):
		// Expected - no event
	}

	// Verify player.Play was NOT called again
	if len(p.PlayCalls()) != 1 {
		t.Errorf("PlayCalls() = %v, want single call", p.PlayCalls())
	}
}

func TestService_JumpTo_ChangesIndex(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: testSvcPathA},
		playlist.Track{Path: testSvcPathB},
		playlist.Track{Path: testSvcPathC},
	)
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing
	_ = svc.Play()
	<-sub.StateChanged

	err := svc.JumpTo(2)

	if err != nil {
		t.Fatalf("JumpTo() error = %v", err)
	}
	if svc.QueueCurrentIndex() != 2 {
		t.Errorf("QueueIndex() = %d, want 2", svc.QueueCurrentIndex())
	}

	// Verify TrackChanged event
	select {
	case e := <-sub.TrackChanged:
		if e.Previous == nil || e.Previous.Path != testSvcPathA {
			t.Errorf("event.Previous.Path = %v, want /a.mp3", e.Previous)
		}
		if e.Current == nil || e.Current.Path != testSvcPathC {
			t.Errorf("event.Current.Path = %v, want /c.mp3", e.Current)
		}
		if e.Index != 2 {
			t.Errorf("event.Index = %d, want 2", e.Index)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for TrackChanged event")
	}

	// Verify player.Play was called with new track
	calls := p.PlayCalls()
	if len(calls) != 2 || calls[1] != testSvcPathC {
		t.Errorf("PlayCalls() = %v, want [/a.mp3, /c.mp3]", calls)
	}
}

func TestService_JumpTo_InvalidIndex_ReturnsError(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA})
	q.JumpTo(0)
	svc := New(p, q)

	// Test negative index
	err := svc.JumpTo(-1)
	if !errors.Is(err, ErrInvalidIndex) {
		t.Errorf("JumpTo(-1) error = %v, want ErrInvalidIndex", err)
	}

	// Test index too large
	err = svc.JumpTo(5)
	if !errors.Is(err, ErrInvalidIndex) {
		t.Errorf("JumpTo(5) error = %v, want ErrInvalidIndex", err)
	}

	// Verify queue index unchanged
	if svc.QueueCurrentIndex() != 0 {
		t.Errorf("QueueIndex() = %d, want 0 (unchanged)", svc.QueueCurrentIndex())
	}
}

func TestService_Seek_SeeksPlayer(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	delta := 10 * time.Second
	err := svc.Seek(delta)

	if err != nil {
		t.Fatalf("Seek() error = %v", err)
	}

	// Verify player.Seek was called with the delta
	calls := p.SeekCalls()
	if len(calls) != 1 || calls[0] != delta {
		t.Errorf("SeekCalls() = %v, want [%v]", calls, delta)
	}

	// Verify PositionChanged event was emitted
	select {
	case e := <-sub.PositionChanged:
		// Position should match what the mock player returns
		if e.Position != p.Position() {
			t.Errorf("event.Position = %v, want %v", e.Position, p.Position())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for PositionChanged event")
	}
}

func TestService_SeekTo_SeeksToPosition(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	// Set current position to 30s
	p.SetPosition(30 * time.Second)

	// Seek to 60s (delta should be 30s)
	targetPosition := 60 * time.Second
	err := svc.SeekTo(targetPosition)

	if err != nil {
		t.Fatalf("SeekTo() error = %v", err)
	}

	// Verify player.Seek was called with calculated delta (60s - 30s = 30s)
	expectedDelta := 30 * time.Second
	calls := p.SeekCalls()
	if len(calls) != 1 || calls[0] != expectedDelta {
		t.Errorf("SeekCalls() = %v, want [%v]", calls, expectedDelta)
	}

	// Verify PositionChanged event was emitted
	select {
	case <-sub.PositionChanged:
		// Event received
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for PositionChanged event")
	}
}

func TestService_SetRepeatMode_ChangesMode(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	svc.SetRepeatMode(RepeatAll)

	if svc.RepeatMode() != RepeatAll {
		t.Errorf("RepeatMode() = %v, want RepeatAll", svc.RepeatMode())
	}

	// Verify ModeChanged event was emitted
	select {
	case e := <-sub.ModeChanged:
		if e.RepeatMode != RepeatAll {
			t.Errorf("event.RepeatMode = %v, want RepeatAll", e.RepeatMode)
		}
		if e.Shuffle != false {
			t.Errorf("event.Shuffle = %v, want false", e.Shuffle)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ModeChanged event")
	}
}

func TestService_CycleRepeatMode_CyclesThroughModes(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	// Verify initial mode is Off
	if svc.RepeatMode() != RepeatOff {
		t.Fatalf("initial RepeatMode() = %v, want RepeatOff", svc.RepeatMode())
	}

	// Cycle: Off -> All
	mode := svc.CycleRepeatMode()
	if mode != RepeatAll {
		t.Errorf("CycleRepeatMode() = %v, want RepeatAll", mode)
	}
	<-sub.ModeChanged

	// Cycle: All -> One
	mode = svc.CycleRepeatMode()
	if mode != RepeatOne {
		t.Errorf("CycleRepeatMode() = %v, want RepeatOne", mode)
	}
	<-sub.ModeChanged

	// Cycle: One -> Radio
	mode = svc.CycleRepeatMode()
	if mode != RepeatRadio {
		t.Errorf("CycleRepeatMode() = %v, want RepeatRadio", mode)
	}
	<-sub.ModeChanged

	// Cycle: Radio -> Off
	mode = svc.CycleRepeatMode()
	if mode != RepeatOff {
		t.Errorf("CycleRepeatMode() = %v, want RepeatOff", mode)
	}

	// Verify final ModeChanged event
	select {
	case e := <-sub.ModeChanged:
		if e.RepeatMode != RepeatOff {
			t.Errorf("event.RepeatMode = %v, want RepeatOff", e.RepeatMode)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ModeChanged event")
	}
}

func TestService_SetShuffle_ChangesShuffle(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	// Verify initial shuffle is off
	if svc.Shuffle() {
		t.Fatal("initial Shuffle() = true, want false")
	}

	svc.SetShuffle(true)

	if !svc.Shuffle() {
		t.Error("Shuffle() = false, want true")
	}

	// Verify ModeChanged event was emitted
	select {
	case e := <-sub.ModeChanged:
		if e.Shuffle != true {
			t.Errorf("event.Shuffle = %v, want true", e.Shuffle)
		}
		if e.RepeatMode != RepeatOff {
			t.Errorf("event.RepeatMode = %v, want RepeatOff", e.RepeatMode)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ModeChanged event")
	}
}

func TestService_ToggleShuffle_TogglesAndReturnsNewState(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	svc := New(p, q)
	sub := svc.Subscribe()

	// Verify initial shuffle is off
	if svc.Shuffle() {
		t.Fatal("initial Shuffle() = true, want false")
	}

	// Toggle: off -> on
	newState := svc.ToggleShuffle()
	if !newState {
		t.Error("ToggleShuffle() = false, want true")
	}
	if !svc.Shuffle() {
		t.Error("Shuffle() = false, want true")
	}

	// Verify ModeChanged event
	select {
	case e := <-sub.ModeChanged:
		if e.Shuffle != true {
			t.Errorf("event.Shuffle = %v, want true", e.Shuffle)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ModeChanged event")
	}

	// Toggle: on -> off
	newState = svc.ToggleShuffle()
	if newState {
		t.Error("ToggleShuffle() = true, want false")
	}
	if svc.Shuffle() {
		t.Error("Shuffle() = true, want false")
	}

	// Verify ModeChanged event
	select {
	case e := <-sub.ModeChanged:
		if e.Shuffle != false {
			t.Errorf("event.Shuffle = %v, want false", e.Shuffle)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ModeChanged event")
	}
}

func TestService_TrackFinished_AdvancesToNext(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: "/track1.mp3", Title: "Track 1"},
		playlist.Track{Path: "/track2.mp3", Title: "Track 2"},
	)
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing first track
	err := svc.Play()
	if err != nil {
		t.Fatalf("Play() error = %v", err)
	}

	// Drain initial StateChanged event
	select {
	case <-sub.StateChanged:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for initial StateChanged")
	}

	// Simulate track finishing
	p.SimulateFinished()

	// Expect TrackChanged event with Index=1 and Current.Path="/track2.mp3"
	select {
	case e := <-sub.TrackChanged:
		if e.Index != 1 {
			t.Errorf("event.Index = %d, want 1", e.Index)
		}
		if e.Current == nil || e.Current.Path != "/track2.mp3" {
			t.Errorf("event.Current.Path = %v, want /track2.mp3", e.Current)
		}
		if e.Previous == nil || e.Previous.Path != "/track1.mp3" {
			t.Errorf("event.Previous.Path = %v, want /track1.mp3", e.Previous)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for TrackChanged event")
	}

	// Verify player.Play was called with new track
	calls := p.PlayCalls()
	if len(calls) != 2 || calls[1] != "/track2.mp3" {
		t.Errorf("PlayCalls() = %v, want [/track1.mp3, /track2.mp3]", calls)
	}
}

func TestService_TrackFinished_AtEnd_Stops(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: "/track1.mp3", Title: "Track 1"})
	q.JumpTo(0)
	svc := New(p, q)
	sub := svc.Subscribe()

	// Start playing first track
	err := svc.Play()
	if err != nil {
		t.Fatalf("Play() error = %v", err)
	}

	// Drain initial StateChanged event
	select {
	case <-sub.StateChanged:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for initial StateChanged")
	}

	// Simulate track finishing (at end of queue)
	p.SimulateFinished()

	// Expect StateChanged event with Current=StateStopped
	select {
	case e := <-sub.StateChanged:
		if e.Previous != StatePlaying {
			t.Errorf("event.Previous = %v, want Playing", e.Previous)
		}
		if e.Current != StateStopped {
			t.Errorf("event.Current = %v, want Stopped", e.Current)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for StateChanged event")
	}

	// Verify service state is stopped
	if svc.State() != StateStopped {
		t.Errorf("State() = %v, want Stopped", svc.State())
	}
}

func TestService_ConcurrentAccess_NoRace(_ *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: "/a.mp3"},
		playlist.Track{Path: "/b.mp3"},
		playlist.Track{Path: "/c.mp3"},
	)
	q.JumpTo(0)
	svc := New(p, q)
	defer svc.Close()

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(6)

		go func() {
			defer wg.Done()
			_ = svc.Toggle()
		}()

		go func() {
			defer wg.Done()
			_ = svc.State()
		}()

		go func() {
			defer wg.Done()
			_ = svc.Position()
		}()

		go func() {
			defer wg.Done()
			_ = svc.QueueTracks()
		}()

		go func() {
			defer wg.Done()
			_ = svc.CurrentTrack()
		}()

		go func() {
			defer wg.Done()
			_ = svc.CycleRepeatMode()
		}()
	}

	wg.Wait()
}

func TestService_MultipleSubscribers_AllReceiveEvents(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: "/song.mp3"})
	q.JumpTo(0)
	svc := New(p, q)
	defer svc.Close()

	sub1 := svc.Subscribe()
	sub2 := svc.Subscribe()
	sub3 := svc.Subscribe()

	_ = svc.Play()

	// All subscribers should receive the state change
	for i, sub := range []*Subscription{sub1, sub2, sub3} {
		select {
		case e := <-sub.StateChanged:
			if e.Current != StatePlaying {
				t.Errorf("sub%d: Current = %v, want Playing", i+1, e.Current)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("sub%d: timeout waiting for StateChanged", i+1)
		}
	}
}
