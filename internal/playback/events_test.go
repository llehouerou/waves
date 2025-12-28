package playback

import (
	"testing"
	"time"
)

const (
	testPathA     = "/a.mp3"
	testPathB     = "/b.mp3"
	testMusicPath = "/music/song.mp3"
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
	prev := &Track{Path: testPathA}
	curr := &Track{Path: testPathB}
	tc := TrackChange{
		Previous: prev,
		Current:  curr,
		Index:    1,
	}
	if tc.Previous.Path != testPathA {
		t.Errorf("Previous.Path = %q, want %s", tc.Previous.Path, testPathA)
	}
	if tc.Current.Path != testPathB {
		t.Errorf("Current.Path = %q, want %s", tc.Current.Path, testPathB)
	}
	if tc.Index != 1 {
		t.Errorf("Index = %d, want 1", tc.Index)
	}
}

func TestQueueChange_Fields(t *testing.T) {
	tracks := []Track{
		{Path: testPathA, Title: "Track A"},
		{Path: testPathB, Title: "Track B"},
	}
	qc := QueueChange{
		Tracks: tracks,
		Index:  1,
	}
	if len(qc.Tracks) != 2 {
		t.Errorf("len(Tracks) = %d, want 2", len(qc.Tracks))
	}
	if qc.Tracks[0].Path != testPathA {
		t.Errorf("Tracks[0].Path = %q, want %s", qc.Tracks[0].Path, testPathA)
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
		Path:        testMusicPath,
		Title:       "My Song",
		Artist:      "Artist Name",
		Album:       "Album Name",
		TrackNumber: 5,
		Duration:    3*time.Minute + 30*time.Second,
	}
	if track.ID != 42 {
		t.Errorf("ID = %d, want 42", track.ID)
	}
	if track.Path != testMusicPath {
		t.Errorf("Path = %q, want %s", track.Path, testMusicPath)
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
