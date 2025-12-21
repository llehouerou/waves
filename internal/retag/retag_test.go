//nolint:goconst // test cases intentionally repeat strings for readability
package retag

import (
	"testing"

	"github.com/llehouerou/waves/internal/ui/action"
)

func TestNew(t *testing.T) {
	trackPaths := []string{"/music/track1.mp3", "/music/track2.mp3", "/music/track3.mp3"}

	m := New("Artist Name", "Album Title", trackPaths, nil, nil)

	if m == nil {
		t.Fatal("New returned nil")
	}

	if m.State() != StateLoading {
		t.Errorf("initial state = %v, want StateLoading", m.State())
	}

	if m.AlbumArtist() != "Artist Name" {
		t.Errorf("AlbumArtist = %q, want %q", m.AlbumArtist(), "Artist Name")
	}

	if m.AlbumName() != "Album Title" {
		t.Errorf("AlbumName = %q, want %q", m.AlbumName(), "Album Title")
	}

	if m.IsComplete() {
		t.Error("IsComplete should be false initially")
	}

	if m.SuccessCount() != 0 {
		t.Errorf("SuccessCount = %d, want 0", m.SuccessCount())
	}

	if m.FailedCount() != 0 {
		t.Errorf("FailedCount = %d, want 0", m.FailedCount())
	}

	// Check retag status initialized
	if len(m.retagStatus) != len(trackPaths) {
		t.Errorf("retagStatus length = %d, want %d", len(m.retagStatus), len(trackPaths))
	}

	for i, status := range m.retagStatus {
		if status.Status != StatusPending {
			t.Errorf("retagStatus[%d].Status = %v, want StatusPending", i, status.Status)
		}
		if status.Filename != trackPaths[i] {
			t.Errorf("retagStatus[%d].Filename = %q, want %q", i, status.Filename, trackPaths[i])
		}
	}

	// Check initial search
	if m.initialSearch != "Artist Name Album Title" {
		t.Errorf("initialSearch = %q, want %q", m.initialSearch, "Artist Name Album Title")
	}
}

func TestNew_EmptyTracks(t *testing.T) {
	m := New("Artist", "Album", []string{}, nil, nil)

	if m == nil {
		t.Fatal("New returned nil")
	}

	if len(m.retagStatus) != 0 {
		t.Errorf("retagStatus length = %d, want 0", len(m.retagStatus))
	}
}

func TestModel_SetSize(t *testing.T) {
	m := New("Artist", "Album", []string{"/test.mp3"}, nil, nil)

	m.SetSize(100, 50)

	if m.Width() != 100 {
		t.Errorf("Width = %d, want 100", m.Width())
	}

	if m.Height() != 50 {
		t.Errorf("Height = %d, want 50", m.Height())
	}

	// Search input width should be adjusted
	expectedInputWidth := 100 - 4
	if m.searchInput.Width != expectedInputWidth {
		t.Errorf("searchInput.Width = %d, want %d", m.searchInput.Width, expectedInputWidth)
	}
}

func TestModel_State(t *testing.T) {
	m := New("Artist", "Album", []string{"/test.mp3"}, nil, nil)

	// Initial state
	if m.State() != StateLoading {
		t.Errorf("State = %v, want StateLoading", m.State())
	}

	// Modify state directly for testing
	m.state = StateSearching
	if m.State() != StateSearching {
		t.Errorf("State = %v, want StateSearching", m.State())
	}

	m.state = StateComplete
	if m.State() != StateComplete {
		t.Errorf("State = %v, want StateComplete", m.State())
	}
}

func TestModel_IsComplete(t *testing.T) {
	m := New("Artist", "Album", []string{"/test.mp3"}, nil, nil)

	// Not complete initially
	if m.IsComplete() {
		t.Error("IsComplete should be false for StateLoading")
	}

	// Test various states
	states := []struct {
		state    State
		complete bool
	}{
		{StateLoading, false},
		{StateSearching, false},
		{StateReleaseGroupResults, false},
		{StateReleaseLoading, false},
		{StateReleaseResults, false},
		{StateReleaseDetailsLoading, false},
		{StateTagPreview, false},
		{StateRetagging, false},
		{StateComplete, true},
	}

	for _, tt := range states {
		m.state = tt.state
		if m.IsComplete() != tt.complete {
			t.Errorf("IsComplete() for state %v = %v, want %v", tt.state, m.IsComplete(), tt.complete)
		}
	}
}

func TestModel_SuccessCount(t *testing.T) {
	m := New("Artist", "Album", []string{"/test.mp3"}, nil, nil)

	if m.SuccessCount() != 0 {
		t.Errorf("SuccessCount = %d, want 0", m.SuccessCount())
	}

	m.successCount = 5
	if m.SuccessCount() != 5 {
		t.Errorf("SuccessCount = %d, want 5", m.SuccessCount())
	}
}

func TestModel_FailedCount(t *testing.T) {
	m := New("Artist", "Album", []string{"/test.mp3"}, nil, nil)

	if m.FailedCount() != 0 {
		t.Errorf("FailedCount = %d, want 0", m.FailedCount())
	}

	m.failedFiles = []FailedFile{
		{Filename: "file1.mp3", Error: "error 1"},
		{Filename: "file2.mp3", Error: "error 2"},
	}
	if m.FailedCount() != 2 {
		t.Errorf("FailedCount = %d, want 2", m.FailedCount())
	}
}

func TestActionMsg(t *testing.T) {
	closeAction := Close{}
	msg := ActionMsg(closeAction)

	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("ActionMsg should return action.Msg, got %T", msg)
	}

	if actionMsg.Source != "retag" {
		t.Errorf("Source = %q, want %q", actionMsg.Source, "retag")
	}

	if actionMsg.Action != closeAction {
		t.Errorf("Action = %v, want %v", actionMsg.Action, closeAction)
	}
}

func TestClose_ActionType(t *testing.T) {
	c := Close{}
	if c.ActionType() != "retag.Close" {
		t.Errorf("Close.ActionType() = %q, want %q", c.ActionType(), "retag.Close")
	}
}

func TestComplete_ActionType(t *testing.T) {
	c := Complete{
		AlbumArtist:  "Artist",
		AlbumName:    "Album",
		SuccessCount: 10,
		FailedCount:  2,
	}
	if c.ActionType() != "retag.Complete" {
		t.Errorf("Complete.ActionType() = %q, want %q", c.ActionType(), "retag.Complete")
	}
}

func TestComplete_Fields(t *testing.T) {
	c := Complete{
		AlbumArtist:  "Test Artist",
		AlbumName:    "Test Album",
		SuccessCount: 15,
		FailedCount:  3,
	}

	if c.AlbumArtist != "Test Artist" {
		t.Errorf("AlbumArtist = %q, want %q", c.AlbumArtist, "Test Artist")
	}
	if c.AlbumName != "Test Album" {
		t.Errorf("AlbumName = %q, want %q", c.AlbumName, "Test Album")
	}
	if c.SuccessCount != 15 {
		t.Errorf("SuccessCount = %d, want 15", c.SuccessCount)
	}
	if c.FailedCount != 3 {
		t.Errorf("FailedCount = %d, want 3", c.FailedCount)
	}
}

func TestTagDiff(t *testing.T) {
	diff := TagDiff{
		Field:    "Artist",
		OldValue: "Old Artist",
		NewValue: "New Artist",
		Changed:  true,
	}

	if diff.Field != "Artist" {
		t.Errorf("Field = %q, want %q", diff.Field, "Artist")
	}
	if diff.OldValue != "Old Artist" {
		t.Errorf("OldValue = %q, want %q", diff.OldValue, "Old Artist")
	}
	if diff.NewValue != "New Artist" {
		t.Errorf("NewValue = %q, want %q", diff.NewValue, "New Artist")
	}
	if !diff.Changed {
		t.Error("Changed should be true")
	}
}

func TestFileRetagStatus(t *testing.T) {
	status := FileRetagStatus{
		Filename: "/music/song.mp3",
		Status:   StatusComplete,
		Error:    "",
	}

	if status.Filename != "/music/song.mp3" {
		t.Errorf("Filename = %q, want %q", status.Filename, "/music/song.mp3")
	}
	if status.Status != StatusComplete {
		t.Errorf("Status = %v, want StatusComplete", status.Status)
	}
}

func TestStatus_Constants(t *testing.T) {
	// Verify status constants have distinct values
	statuses := map[Status]string{
		StatusPending:   "Pending",
		StatusRetagging: "Retagging",
		StatusComplete:  "Complete",
		StatusFailed:    "Failed",
	}

	seen := make(map[Status]bool)
	for s := range statuses {
		if seen[s] {
			t.Errorf("Duplicate status value: %v", s)
		}
		seen[s] = true
	}

	if len(seen) != 4 {
		t.Errorf("Expected 4 unique status values, got %d", len(seen))
	}
}

func TestState_Constants(t *testing.T) {
	// Verify state constants have distinct values
	states := []State{
		StateLoading,
		StateSearching,
		StateReleaseGroupResults,
		StateReleaseLoading,
		StateReleaseResults,
		StateReleaseDetailsLoading,
		StateTagPreview,
		StateRetagging,
		StateComplete,
	}

	seen := make(map[State]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("Duplicate state value: %v", s)
		}
		seen[s] = true
	}

	if len(seen) != 9 {
		t.Errorf("Expected 9 unique state values, got %d", len(seen))
	}
}

func TestFailedFile(t *testing.T) {
	f := FailedFile{
		Filename: "song.mp3",
		Error:    "permission denied",
	}

	if f.Filename != "song.mp3" {
		t.Errorf("Filename = %q, want %q", f.Filename, "song.mp3")
	}
	if f.Error != "permission denied" {
		t.Errorf("Error = %q, want %q", f.Error, "permission denied")
	}
}

func TestModel_View_ZeroSize(t *testing.T) {
	m := New("Artist", "Album", []string{"/test.mp3"}, nil, nil)

	// With zero size, View should return empty string
	view := m.View()
	if view != "" {
		t.Errorf("View with zero size should return empty string, got %q", view)
	}

	// With only width set
	m.SetSize(100, 0)
	view = m.View()
	if view != "" {
		t.Errorf("View with zero height should return empty string, got %q", view)
	}

	// With only height set
	m.SetSize(0, 50)
	view = m.View()
	if view != "" {
		t.Errorf("View with zero width should return empty string, got %q", view)
	}
}

func TestModel_innerWidth(t *testing.T) {
	m := New("Artist", "Album", []string{"/test.mp3"}, nil, nil)
	m.SetSize(100, 50)

	// innerWidth should account for popup border and padding
	expected := 100 - 8
	if m.innerWidth() != expected {
		t.Errorf("innerWidth = %d, want %d", m.innerWidth(), expected)
	}
}
