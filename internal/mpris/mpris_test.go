//go:build linux

package mpris

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/quarckster/go-mpris-server/pkg/types"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
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

func (f *fakeService) PlayPath(string) error { return nil }

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

func (f *fakeService) State() playback.State { return playback.StateStopped }

func (f *fakeService) IsPlaying() bool { return false }

func (f *fakeService) IsStopped() bool { return true }

func (f *fakeService) IsPaused() bool { return false }

func (f *fakeService) Position() time.Duration { return 0 }

func (f *fakeService) TrackInfo() *tags.FileInfo { return nil }

func (f *fakeService) Player() player.Interface { return nil }

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
