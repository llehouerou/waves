//go:build linux || freebsd

package mpris

import (
	"errors"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/quarckster/go-mpris-server/pkg/types"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/tags"
)

// fakeService is a minimal playback.Service stub for testing.
type fakeService struct {
	track    *playback.Track
	duration time.Duration
}

func (f *fakeService) CurrentTrack() *playback.Track { return f.track }

func (f *fakeService) Duration() time.Duration { return f.duration }

// Unused interface methods — stubs.
func (f *fakeService) Play() error { return nil }

func (f *fakeService) Pause() error { return nil }

func (f *fakeService) Stop() error { return nil }

func (f *fakeService) Toggle() error { return nil }

func (f *fakeService) Next() error { return nil }

func (f *fakeService) Previous() error { return nil }

func (f *fakeService) Seek(time.Duration) error { return nil }

func (f *fakeService) SeekTo(time.Duration) error { return nil }

func (f *fakeService) JumpTo(int) error { return nil }

func (f *fakeService) QueueAdvance() *playback.Track { return nil }

func (f *fakeService) QueueMoveTo(int) *playback.Track { return nil }

func (f *fakeService) AddTracks(...playback.Track) {}

func (f *fakeService) ReplaceTracks(...playback.Track) *playback.Track { return nil }

func (f *fakeService) ClearQueue() {}

func (f *fakeService) RestoreQueue(playback.SavedQueue) {}

func (f *fakeService) RemoveTracks([]int) {}

func (f *fakeService) MoveTracks([]int, int) {}

func (f *fakeService) State() playback.State { return playback.StateStopped }

func (f *fakeService) IsPlaying() bool { return false }

func (f *fakeService) IsStopped() bool { return true }

func (f *fakeService) IsPaused() bool { return false }

func (f *fakeService) Position() time.Duration { return 0 }

func (f *fakeService) TrackInfo() *tags.FileInfo { return nil }

func (f *fakeService) QueueTracks() []playback.Track { return nil }

func (f *fakeService) QueueCurrentIndex() int { return 0 }

func (f *fakeService) QueueLen() int { return 0 }

func (f *fakeService) QueueIsEmpty() bool { return true }

func (f *fakeService) QueueHasNext() bool { return false }

func (f *fakeService) QueuePeekNext() *playback.Track { return nil }

func (f *fakeService) Undo() bool { return false }

func (f *fakeService) Redo() bool { return false }

func (f *fakeService) RepeatMode() playback.RepeatMode { return playback.RepeatOff }

func (f *fakeService) SetRepeatMode(playback.RepeatMode) {}

func (f *fakeService) CycleRepeatMode() playback.RepeatMode { return playback.RepeatOff }

func (f *fakeService) ResumeAfterRunOut() error { return nil }

func (f *fakeService) Shuffle() bool { return false }

func (f *fakeService) SetShuffle(bool) {}

func (f *fakeService) ToggleShuffle() bool { return false }

func (f *fakeService) Subscribe() *playback.Subscription { return nil }

func (f *fakeService) Close() error { return nil }

func TestMetadata_NoTrack_ValidObjectPath(t *testing.T) {
	adapter := &playerAdapter{service: &fakeService{}}

	meta, err := adapter.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}

	if !meta.TrackId.IsValid() {
		t.Errorf("TrackId = %q, want valid dbus ObjectPath", meta.TrackId)
	}

	want := dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack")
	if meta.TrackId != want {
		t.Errorf("TrackId = %q, want %q", meta.TrackId, want)
	}
}

func TestMetadata_WithTrack_SetsUrl(t *testing.T) {
	svc := &fakeService{
		track: &playback.Track{Path: "/music/artist/album/song.flac"},
	}
	adapter := &playerAdapter{service: svc}

	meta, err := adapter.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}

	want := "file:///music/artist/album/song.flac"
	if meta.Url != want {
		t.Errorf("Url = %q, want %q", meta.Url, want)
	}
}

func TestMetadata_WithTrack_SetsExtendedFields(t *testing.T) {
	svc := &fakeService{
		track: &playback.Track{
			Path:        "/music/song.flac",
			DiscNumber:  2,
			Genre:       "Rock",
			Year:        2024,
			TrackNumber: 5,
		},
	}
	adapter := &playerAdapter{service: svc}

	meta, err := adapter.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}

	if meta.DiscNumber != 2 {
		t.Errorf("DiscNumber = %d, want 2", meta.DiscNumber)
	}
	if len(meta.Genre) != 1 || meta.Genre[0] != "Rock" {
		t.Errorf("Genre = %v, want [Rock]", meta.Genre)
	}
	if meta.ContentCreated != "2024" {
		t.Errorf("ContentCreated = %q, want %q", meta.ContentCreated, "2024")
	}
}

func TestMetadata_WithTrack_UsesServiceDuration(t *testing.T) {
	svc := &fakeService{
		track:    &playback.Track{Path: "/music/song.flac"},
		duration: 3*time.Minute + 30*time.Second,
	}
	adapter := &playerAdapter{service: svc}

	meta, err := adapter.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}

	want := types.Microseconds((3*time.Minute + 30*time.Second).Microseconds())
	if meta.Length != want {
		t.Errorf("Length = %d, want %d", meta.Length, want)
	}
}

// signals stands in for the session bus: it keeps the names of the
// PropertiesChanged signals the adapter asks for.
type signals struct {
	mu   sync.Mutex
	sent []string
}

func (s *signals) record(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, name)
	return nil
}

func (s *signals) OnEnded() error { return s.record("ended") }

func (s *signals) OnVolume() error { return s.record("volume") }

func (s *signals) OnPlayback() error { return s.record("playback") }

func (s *signals) OnPlayPause() error { return s.record("playpause") }

func (s *signals) OnTitle() error { return s.record("title") }

func (s *signals) OnSeek(types.Microseconds) error { return s.record("seek") }

func (s *signals) OnOptions() error { return s.record("options") }

func (s *signals) OnAll() error { return s.record("all") }

// wait fails the test unless the signal name is asked for n times.
func (s *signals) wait(t *testing.T, name string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		got := 0
		for _, sent := range s.sent {
			if sent == name {
				got++
			}
		}
		all := slices.Clone(s.sent)
		s.mu.Unlock()
		if got >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("signals = %v, want %q %d times", all, name, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// No session bus here: the adapter listens to a real playback service, and
// what it would emit is kept.
func TestEventLoop_TellsTheBusWhatPlaybackDoes(t *testing.T) {
	svc := playback.New(player.NewMock(), playlist.NewQueue())
	t.Cleanup(func() { svc.Close() })
	bus := &signals{}
	a := &Adapter{player: bus, done: make(chan struct{})}
	go a.runEventLoop(svc.Subscribe())
	t.Cleanup(func() { close(a.done) })

	svc.AddTracks(playback.Track{Path: "/music/a.flac"}, playback.Track{Path: "/music/b.flac"})
	svc.QueueMoveTo(0)
	bus.wait(t, "options", 2) // what can be skipped to changed twice

	if err := svc.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	bus.wait(t, "playback", 1)

	if err := svc.Next(); err != nil {
		t.Fatalf("next: %v", err)
	}
	bus.wait(t, "title", 1)

	if err := svc.Seek(10 * time.Second); err != nil {
		t.Fatalf("seek: %v", err)
	}
	bus.wait(t, "seek", 1)

	svc.SetShuffle(true)
	bus.wait(t, "options", 3)

	if err := svc.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	bus.wait(t, "ended", 1)
}

// MPRIS Next onto a track that fails to open stops playback: the widgets are
// told it ended, not left showing it playing.
func TestEventLoop_TellsTheBusPlaybackEndedWhenNextFailsToOpen(t *testing.T) {
	mock := player.NewMock()
	svc := playback.New(mock, playlist.NewQueue())
	t.Cleanup(func() { svc.Close() })
	bus := &signals{}
	a := &Adapter{player: bus, done: make(chan struct{})}
	go a.runEventLoop(svc.Subscribe())
	t.Cleanup(func() { close(a.done) })

	svc.AddTracks(playback.Track{Path: "/music/a.flac"}, playback.Track{Path: "/music/b.flac"})
	svc.QueueMoveTo(0)
	if err := svc.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	bus.wait(t, "playback", 1)

	mock.SetPlayError(errors.New("unreadable"))
	adapter := &playerAdapter{service: svc, bus: bus}
	if err := adapter.Next(); err == nil {
		t.Fatal("next succeeded, want the open failure")
	}
	bus.wait(t, "ended", 1)
}

// fakeVolume stands in for the volume of the core.
type fakeVolume struct {
	level float64
	set   []float64
}

func (f *fakeVolume) Volume() float64 { return f.level }

func (f *fakeVolume) SetVolume(level float64) {
	f.level = level
	f.set = append(f.set, level)
}

func TestVolume_IsTheVolumeOfWaves(t *testing.T) {
	adapter := &playerAdapter{service: &fakeService{}, volume: &fakeVolume{level: 0.7}}

	if got, err := adapter.Volume(); err != nil || got != 0.7 {
		t.Errorf("Volume() = %v, %v, want 0.7", got, err)
	}
}

// The MPRIS spec: a negative volume is 0. Above 1 is louder than waves goes.
func TestSetVolume_SetsTheVolumeOfWavesWithinItsRange(t *testing.T) {
	for _, tc := range []struct{ asked, want float64 }{{0.4, 0.4}, {-1, 0}, {2, 1}} {
		vol := &fakeVolume{}
		bus := &signals{}
		adapter := &playerAdapter{service: &fakeService{}, volume: vol, bus: bus}

		if err := adapter.SetVolume(tc.asked); err != nil {
			t.Fatalf("SetVolume(%v): %v", tc.asked, err)
		}

		if !slices.Equal(vol.set, []float64{tc.want}) {
			t.Errorf("SetVolume(%v) set %v, want %v", tc.asked, vol.set, tc.want)
		}
		// Other widgets listen too.
		if !slices.Equal(bus.sent, []string{"volume"}) {
			t.Errorf("SetVolume(%v) sent %v, want one volume signal", tc.asked, bus.sent)
		}
	}
}

func TestSetVolume_RefusesWhatIsNotANumber(t *testing.T) {
	vol := &fakeVolume{level: 0.5}
	adapter := &playerAdapter{service: &fakeService{}, volume: vol, bus: &signals{}}

	if err := adapter.SetVolume(math.NaN()); err == nil {
		t.Error("SetVolume(NaN) succeeded")
	}
	if vol.set != nil {
		t.Errorf("SetVolume(NaN) set %v", vol.set)
	}
}

func TestVolumeChanged_TellsTheBus(t *testing.T) {
	bus := &signals{}
	a := &Adapter{player: bus}

	a.VolumeChanged()

	if !slices.Equal(bus.sent, []string{"volume"}) {
		t.Errorf("sent %v, want one volume signal", bus.sent)
	}
}
