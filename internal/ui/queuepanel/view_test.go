package queuepanel

import (
	"regexp"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/playlist"
)

// stripANSI removes ANSI escape codes from a string for easier testing.
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// newTestQueue creates a queue with test tracks.
func newTestQueue(tracks ...playlist.Track) *playlist.PlayingQueue {
	q := playlist.NewQueue()
	for _, t := range tracks {
		q.Add(t)
	}
	return q
}

// testTrack creates a track with the given title and artist.
func testTrack(title, artist string) playlist.Track {
	return playlist.Track{
		Title:  title,
		Artist: artist,
		Path:   "/test/" + title + ".mp3",
	}
}

func TestView_EmptyQueue(t *testing.T) {
	q := playlist.NewQueue()
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should show "Queue (0/0)" in header
	if !strings.Contains(stripped, "Queue (0/0)") {
		t.Errorf("empty queue should show 'Queue (0/0)', got: %s", stripped)
	}
}

func TestView_SingleTrack(t *testing.T) {
	q := newTestQueue(testTrack("Test Song", "Test Artist"))
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should contain the track info
	if !strings.Contains(stripped, "Test Song") {
		t.Errorf("should contain track title, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Test Artist") {
		t.Errorf("should contain track artist, got: %s", stripped)
	}
}

func TestView_MultipleTracksHeader(t *testing.T) {
	q := newTestQueue(
		testTrack("Song 1", "Artist 1"),
		testTrack("Song 2", "Artist 2"),
		testTrack("Song 3", "Artist 3"),
	)
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should show "Queue (0/3)" - no track playing yet
	if !strings.Contains(stripped, "Queue (0/3)") {
		t.Errorf("should show 'Queue (0/3)', got: %s", stripped)
	}
}

func TestView_CurrentTrackInHeader(t *testing.T) {
	q := newTestQueue(
		testTrack("Song 1", "Artist 1"),
		testTrack("Song 2", "Artist 2"),
		testTrack("Song 3", "Artist 3"),
	)
	q.JumpTo(1) // Second track is current
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should show "Queue (2/3)" - track 2 of 3
	if !strings.Contains(stripped, "Queue (2/3)") {
		t.Errorf("should show 'Queue (2/3)', got: %s", stripped)
	}
}

func TestView_PlayingIndicator(t *testing.T) {
	q := newTestQueue(
		testTrack("Song 1", "Artist 1"),
		testTrack("Song 2", "Artist 2"),
	)
	q.JumpTo(0) // First track is playing
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should contain playing symbol (▶)
	if !strings.Contains(stripped, "▶") {
		t.Errorf("should contain playing symbol, got: %s", stripped)
	}
}

func TestView_SelectionCount(t *testing.T) {
	q := newTestQueue(
		testTrack("Song 1", "Artist 1"),
		testTrack("Song 2", "Artist 2"),
		testTrack("Song 3", "Artist 3"),
	)
	m := New(q)
	m.SetSize(60, 10)
	m.SetFocused(true)

	// Select two items
	m.selected[0] = true
	m.selected[2] = true

	output := m.View()
	stripped := stripANSI(output)

	// Should show selection count in header
	if !strings.Contains(stripped, "[2 selected]") {
		t.Errorf("should show '[2 selected]', got: %s", stripped)
	}
}

func TestView_SelectionMarker(t *testing.T) {
	q := newTestQueue(
		testTrack("Song 1", "Artist 1"),
		testTrack("Song 2", "Artist 2"),
	)
	m := New(q)
	m.SetSize(60, 10)
	m.SetFocused(true)
	m.selected[0] = true

	output := m.View()
	stripped := stripANSI(output)

	// Should contain selection symbol (●)
	if !strings.Contains(stripped, "●") {
		t.Errorf("should contain selection symbol, got: %s", stripped)
	}
}

func TestView_ZeroSize(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	m := New(q)
	// Don't set size - should return empty

	output := m.View()
	if output != "" {
		t.Errorf("zero size should return empty string, got: %q", output)
	}
}

func TestView_ShuffleIcon(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	q.SetShuffle(true)
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should contain shuffle icon (default style uses [S])
	if !strings.Contains(stripped, "[S]") {
		t.Errorf("should contain shuffle icon when shuffle enabled, got: %s", stripped)
	}
}

func TestView_RepeatAllIcon(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	q.SetRepeatMode(playlist.RepeatAll)
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should contain repeat all icon (default style uses [R])
	if !strings.Contains(stripped, "[R]") {
		t.Errorf("should contain repeat all icon, got: %s", stripped)
	}
}

func TestView_RepeatOneIcon(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	q.SetRepeatMode(playlist.RepeatOne)
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should contain repeat one icon (default style uses [1])
	if !strings.Contains(stripped, "[1]") {
		t.Errorf("should contain repeat one icon, got: %s", stripped)
	}
}

func TestView_NoIconsWhenOff(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	q.SetShuffle(false)
	q.SetRepeatMode(playlist.RepeatOff)
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should not contain shuffle or repeat icons
	if strings.Contains(stripped, "[S]") {
		t.Errorf("should not contain shuffle icon when off, got: %s", stripped)
	}
	if strings.Contains(stripped, "[R]") || strings.Contains(stripped, "[1]") {
		t.Errorf("should not contain repeat icons when off, got: %s", stripped)
	}
}

func TestView_ContainsSeparator(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	m := New(q)
	m.SetSize(60, 10)

	output := m.View()
	stripped := stripANSI(output)

	// Should contain separator line
	if !strings.Contains(stripped, "─") {
		t.Errorf("should contain separator line, got: %s", stripped)
	}
}

func TestRenderTrackLine_BasicFormat(t *testing.T) {
	q := newTestQueue(testTrack("My Song", "My Artist"))
	m := New(q)
	m.SetSize(60, 10)

	line := m.renderTrackLine(q.Tracks()[0], 0, -1, 50)
	stripped := stripANSI(line)

	if !strings.Contains(stripped, "My Song") {
		t.Errorf("track line should contain title, got: %s", stripped)
	}
	if !strings.Contains(stripped, "My Artist") {
		t.Errorf("track line should contain artist, got: %s", stripped)
	}
}

func TestRenderTrackLine_PlayingPrefix(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	m := New(q)
	m.SetSize(60, 10)

	// Not playing
	line := m.renderTrackLine(q.Tracks()[0], 0, -1, 50)
	stripped := stripANSI(line)
	if strings.Contains(stripped, "▶") {
		t.Errorf("non-playing track should not have play symbol")
	}

	// Playing
	line = m.renderTrackLine(q.Tracks()[0], 0, 0, 50)
	stripped = stripANSI(line)
	if !strings.Contains(stripped, "▶") {
		t.Errorf("playing track should have play symbol")
	}
}

func TestRenderTrackLine_SelectionSuffix(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	m := New(q)
	m.SetSize(60, 10)

	// Not selected
	line := m.renderTrackLine(q.Tracks()[0], 0, -1, 50)
	stripped := stripANSI(line)
	if strings.Contains(stripped, "●") {
		t.Errorf("non-selected track should not have selection symbol")
	}

	// Selected
	m.selected[0] = true
	line = m.renderTrackLine(q.Tracks()[0], 0, -1, 50)
	stripped = stripANSI(line)
	if !strings.Contains(stripped, "●") {
		t.Errorf("selected track should have selection symbol")
	}
}

func TestRenderModeIcons_Empty(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	q.SetShuffle(false)
	q.SetRepeatMode(playlist.RepeatOff)
	m := New(q)

	styled, width := m.renderModeIcons()

	if styled != "" {
		t.Errorf("no icons should return empty string, got: %q", styled)
	}
	if width != 0 {
		t.Errorf("no icons should return width 0, got: %d", width)
	}
}

func TestRenderModeIcons_ShuffleOnly(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	q.SetShuffle(true)
	q.SetRepeatMode(playlist.RepeatOff)
	m := New(q)

	styled, width := m.renderModeIcons()

	// Default icon style uses [S] for shuffle
	if !strings.Contains(stripANSI(styled), "[S]") {
		t.Errorf("shuffle icon should be present, got: %q", styled)
	}
	if width == 0 {
		t.Errorf("width should be non-zero when icon present")
	}
}

func TestRenderModeIcons_Both(t *testing.T) {
	q := newTestQueue(testTrack("Song", "Artist"))
	q.SetShuffle(true)
	q.SetRepeatMode(playlist.RepeatAll)
	m := New(q)

	styled, width := m.renderModeIcons()
	stripped := stripANSI(styled)

	// Default icon style uses [S] for shuffle, [R] for repeat all
	if !strings.Contains(stripped, "[S]") {
		t.Errorf("shuffle icon should be present, got: %q", stripped)
	}
	if !strings.Contains(stripped, "[R]") {
		t.Errorf("repeat icon should be present, got: %q", stripped)
	}
	if width == 0 {
		t.Errorf("width should be non-zero when icons present")
	}
}

func TestTrackStyle_Combinations(t *testing.T) {
	q := newTestQueue(
		testTrack("Song 1", "Artist 1"),
		testTrack("Song 2", "Artist 2"),
		testTrack("Song 3", "Artist 3"),
	)
	q.JumpTo(1) // Second track is playing
	m := New(q)
	m.SetSize(60, 10)
	m.SetFocused(true)
	m.cursor.Jump(1, q.Len(), m.listHeight()) // Cursor on playing track

	// Track 0: played (before current) - should use dimmed style
	style0 := m.trackStyle(0, 1)
	rendered0 := style0.Render("test")
	if rendered0 == "" {
		t.Error("style should produce non-empty output")
	}

	// Track 1: playing and cursor - should combine styles
	style1 := m.trackStyle(1, 1)
	rendered1 := style1.Render("test")
	if rendered1 == "" {
		t.Error("style should produce non-empty output")
	}

	// Track 2: upcoming - should use default track style
	style2 := m.trackStyle(2, 1)
	rendered2 := style2.Render("test")
	if rendered2 == "" {
		t.Error("style should produce non-empty output")
	}

	// Move cursor to track 0 (played + cursor)
	m.cursor.Jump(0, q.Len(), m.listHeight())
	style0Cursor := m.trackStyle(0, 1)
	rendered0Cursor := style0Cursor.Render("test")
	if rendered0Cursor == "" {
		t.Error("style should produce non-empty output")
	}

	// Unfocused - cursor style should not apply
	m.SetFocused(false)
	styleUnfocused := m.trackStyle(0, 1)
	renderedUnfocused := styleUnfocused.Render("test")
	if renderedUnfocused == "" {
		t.Error("style should produce non-empty output")
	}
}
