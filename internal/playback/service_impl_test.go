// internal/playback/service_impl_test.go
package playback

import (
	"errors"
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

	tracks := svc.Queue()

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

	if svc.QueueIndex() != -1 {
		t.Errorf("QueueIndex() = %d, want -1 (no current)", svc.QueueIndex())
	}

	q.JumpTo(1)
	if svc.QueueIndex() != 1 {
		t.Errorf("QueueIndex() = %d, want 1", svc.QueueIndex())
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
	if svc.QueueIndex() != 1 {
		t.Errorf("QueueIndex() = %d, want 1", svc.QueueIndex())
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
	if svc.QueueIndex() != 1 {
		t.Errorf("QueueIndex() = %d, want 1", svc.QueueIndex())
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
	if svc.QueueIndex() != 0 {
		t.Errorf("QueueIndex() = %d, want 0", svc.QueueIndex())
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
	if svc.QueueIndex() != 0 {
		t.Errorf("QueueIndex() = %d, want 0 (unchanged)", svc.QueueIndex())
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
	if svc.QueueIndex() != 2 {
		t.Errorf("QueueIndex() = %d, want 2", svc.QueueIndex())
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
	if svc.QueueIndex() != 0 {
		t.Errorf("QueueIndex() = %d, want 0 (unchanged)", svc.QueueIndex())
	}
}
