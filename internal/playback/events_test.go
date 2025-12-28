package playback

import (
	"testing"
	"time"
)

func TestStateChange_Fields(t *testing.T) {
	sc := StateChange{
		Previous: StateStopped,
		Current:  StatePlaying,
	}
	if sc.Previous != StateStopped {
		t.Errorf("Previous = %v, want Stopped", sc.Previous)
	}
	if sc.Current != StatePlaying {
		t.Errorf("Current = %v, want Playing", sc.Current)
	}
}

func TestTrackChange_Fields(t *testing.T) {
	prev := &Track{Path: "/a.mp3"}
	curr := &Track{Path: "/b.mp3"}
	tc := TrackChange{
		Previous: prev,
		Current:  curr,
		Index:    1,
	}
	if tc.Previous.Path != "/a.mp3" {
		t.Errorf("Previous.Path = %q, want /a.mp3", tc.Previous.Path)
	}
	if tc.Current.Path != "/b.mp3" {
		t.Errorf("Current.Path = %q, want /b.mp3", tc.Current.Path)
	}
	if tc.Index != 1 {
		t.Errorf("Index = %d, want 1", tc.Index)
	}
}

func TestQueueChange_Fields(t *testing.T) {
	tracks := []Track{
		{Path: "/a.mp3", Title: "Track A"},
		{Path: "/b.mp3", Title: "Track B"},
	}
	qc := QueueChange{
		Tracks: tracks,
		Index:  1,
	}
	if len(qc.Tracks) != 2 {
		t.Errorf("len(Tracks) = %d, want 2", len(qc.Tracks))
	}
	if qc.Tracks[0].Path != "/a.mp3" {
		t.Errorf("Tracks[0].Path = %q, want /a.mp3", qc.Tracks[0].Path)
	}
	if qc.Index != 1 {
		t.Errorf("Index = %d, want 1", qc.Index)
	}
}

func TestModeChange_Fields(t *testing.T) {
	mc := ModeChange{
		RepeatMode: RepeatAll,
		Shuffle:    true,
	}
	if mc.RepeatMode != RepeatAll {
		t.Errorf("RepeatMode = %v, want RepeatAll", mc.RepeatMode)
	}
	if !mc.Shuffle {
		t.Error("Shuffle = false, want true")
	}
}

func TestPositionChange_Fields(t *testing.T) {
	pc := PositionChange{
		Position: 30 * time.Second,
	}
	if pc.Position != 30*time.Second {
		t.Errorf("Position = %v, want 30s", pc.Position)
	}
}

func TestTrack_Fields(t *testing.T) {
	track := Track{
		ID:          42,
		Path:        "/music/song.mp3",
		Title:       "My Song",
		Artist:      "Artist Name",
		Album:       "Album Name",
		TrackNumber: 5,
		Duration:    3*time.Minute + 30*time.Second,
	}
	if track.ID != 42 {
		t.Errorf("ID = %d, want 42", track.ID)
	}
	if track.Path != "/music/song.mp3" {
		t.Errorf("Path = %q, want /music/song.mp3", track.Path)
	}
	if track.Title != "My Song" {
		t.Errorf("Title = %q, want My Song", track.Title)
	}
	if track.Artist != "Artist Name" {
		t.Errorf("Artist = %q, want Artist Name", track.Artist)
	}
	if track.Album != "Album Name" {
		t.Errorf("Album = %q, want Album Name", track.Album)
	}
	if track.TrackNumber != 5 {
		t.Errorf("TrackNumber = %d, want 5", track.TrackNumber)
	}
	if track.Duration != 3*time.Minute+30*time.Second {
		t.Errorf("Duration = %v, want 3m30s", track.Duration)
	}
}
