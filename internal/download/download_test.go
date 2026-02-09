package download

import (
	"testing"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/musicbrainz/workflow"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newDownloadPopup() *testutil.PopupHarness {
	m := New("", "", FilterConfig{}, nil)
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m)
}

func getModel(t *testing.T, h *testutil.PopupHarness) *Model {
	t.Helper()
	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	return m
}

func getDownloadAction(t *testing.T, h *testutil.PopupHarness) action.Action {
	t.Helper()
	cmd := h.LastCommand()
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	msg := testutil.ExecuteCmd(cmd)
	actMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}
	return actMsg.Action
}

// === Close Tests ===

func TestDownload_CloseWithEscape(t *testing.T) {
	h := newDownloadPopup()

	h.SendEscape()

	act := getDownloadAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Errorf("expected Close action, got %T", act)
	}
}

func TestDownload_CloseWithEscapeFromAnyState(t *testing.T) {
	states := []State{
		StateSearch,
		StateArtistResults,
		StateReleaseGroupResults,
		StateReleaseResults,
		StateSlskdResults,
	}

	for _, state := range states {
		h := newDownloadPopup()
		m := getModel(t, h)
		m.state = state

		h.SendEscape()

		act := getDownloadAction(t, h)
		if _, ok := act.(Close); !ok {
			t.Errorf("state %d: expected Close action, got %T", state, act)
		}
	}
}

// === State Tests ===

func TestDownload_InitialStateIsSearch(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)

	if m.state != StateSearch {
		t.Errorf("initial state = %d, want StateSearch", m.state)
	}
}

func TestDownload_StateAccessor(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)

	if m.State() != StateSearch {
		t.Errorf("State() = %d, want StateSearch", m.State())
	}

	m.state = StateArtistResults
	if m.State() != StateArtistResults {
		t.Errorf("State() = %d, want StateArtistResults", m.State())
	}
}

// === State Phase Tests ===

func TestDownload_IsSearchPhase(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateSearch, true},
		{StateArtistSearching, true},
		{StateArtistResults, true},
		{StateReleaseGroupLoading, false},
		{StateSlskdResults, false},
	}

	for _, tt := range tests {
		if got := tt.state.IsSearchPhase(); got != tt.expected {
			t.Errorf("State(%d).IsSearchPhase() = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestDownload_IsReleaseGroupPhase(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateSearch, false},
		{StateReleaseGroupLoading, true},
		{StateReleaseGroupResults, true},
		{StateReleaseLoading, false},
	}

	for _, tt := range tests {
		if got := tt.state.IsReleaseGroupPhase(); got != tt.expected {
			t.Errorf("State(%d).IsReleaseGroupPhase() = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestDownload_IsReleasePhase(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateReleaseGroupResults, false},
		{StateReleaseLoading, true},
		{StateReleaseResults, true},
		{StateReleaseDetailsLoading, true},
		{StateSlskdSearching, false},
	}

	for _, tt := range tests {
		if got := tt.state.IsReleasePhase(); got != tt.expected {
			t.Errorf("State(%d).IsReleasePhase() = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestDownload_IsSlskdPhase(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateReleaseResults, false},
		{StateSlskdSearching, true},
		{StateSlskdResults, true},
		{StateDownloading, true},
	}

	for _, tt := range tests {
		if got := tt.state.IsSlskdPhase(); got != tt.expected {
			t.Errorf("State(%d).IsSlskdPhase() = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestDownload_IsLoading(t *testing.T) {
	loadingStates := []State{
		StateArtistSearching,
		StateReleaseGroupLoading,
		StateReleaseLoading,
		StateReleaseDetailsLoading,
		StateSlskdSearching,
		StateDownloading,
	}

	for _, state := range loadingStates {
		if !state.IsLoading() {
			t.Errorf("State(%d).IsLoading() = false, want true", state)
		}
	}

	nonLoadingStates := []State{
		StateSearch,
		StateArtistResults,
		StateReleaseGroupResults,
		StateReleaseResults,
		StateSlskdResults,
	}

	for _, state := range nonLoadingStates {
		if state.IsLoading() {
			t.Errorf("State(%d).IsLoading() = true, want false", state)
		}
	}
}

func TestDownload_CanNavigate(t *testing.T) {
	navigableStates := []State{
		StateArtistResults,
		StateReleaseGroupResults,
		StateReleaseResults,
		StateSlskdResults,
	}

	for _, state := range navigableStates {
		if !state.CanNavigate() {
			t.Errorf("State(%d).CanNavigate() = false, want true", state)
		}
	}

	nonNavigableStates := []State{
		StateSearch,
		StateArtistSearching,
		StateReleaseGroupLoading,
		StateReleaseLoading,
		StateSlskdSearching,
	}

	for _, state := range nonNavigableStates {
		if state.CanNavigate() {
			t.Errorf("State(%d).CanNavigate() = true, want false", state)
		}
	}
}

// === Reset Tests ===

func TestDownload_Reset(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)

	// Set some state
	m.state = StateSlskdResults
	m.searchQuery = "test"
	m.artistResults = []musicbrainz.Artist{{Name: "Test"}}
	m.downloadComplete = true

	m.Reset()

	if m.state != StateSearch {
		t.Errorf("after Reset, state = %d, want StateSearch", m.state)
	}
	if m.searchQuery != "" {
		t.Errorf("after Reset, searchQuery = %q, want empty", m.searchQuery)
	}
	if m.artistResults != nil {
		t.Error("after Reset, artistResults should be nil")
	}
	if m.downloadComplete {
		t.Error("after Reset, downloadComplete should be false")
	}
}

// === Filter Configuration Tests ===

func TestDownload_FilterConfigDefaults(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)

	if m.formatFilter != FormatBoth {
		t.Errorf("default formatFilter = %d, want FormatBoth", m.formatFilter)
	}
	if !m.filterNoSlot {
		t.Error("default filterNoSlot should be true")
	}
	if !m.filterTrackCount {
		t.Error("default filterTrackCount should be true")
	}
	if !m.albumsOnly {
		t.Error("default albumsOnly should be true")
	}
}

func TestDownload_FilterConfigLossless(t *testing.T) {
	m := New("", "", FilterConfig{Format: "lossless"}, nil)

	if m.formatFilter != FormatLossless {
		t.Errorf("formatFilter = %d, want FormatLossless", m.formatFilter)
	}
}

func TestDownload_FilterConfigLossy(t *testing.T) {
	m := New("", "", FilterConfig{Format: "lossy"}, nil)

	if m.formatFilter != FormatLossy {
		t.Errorf("formatFilter = %d, want FormatLossy", m.formatFilter)
	}
}

func TestDownload_FilterConfigCustomBooleans(t *testing.T) {
	falseVal := false
	m := New("", "", FilterConfig{
		NoSlot:     &falseVal,
		TrackCount: &falseVal,
		AlbumsOnly: &falseVal,
	}, nil)

	if m.filterNoSlot {
		t.Error("filterNoSlot should be false")
	}
	if m.filterTrackCount {
		t.Error("filterTrackCount should be false")
	}
	if m.albumsOnly {
		t.Error("albumsOnly should be false")
	}
}

// === Navigation Tests ===

func TestDownload_NavigateArtistResultsWithJ(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateArtistResults
	m.artistResults = []musicbrainz.Artist{
		{Name: "Artist 1"},
		{Name: "Artist 2"},
		{Name: "Artist 3"},
	}

	initial := m.artistCursor.Pos()
	h.SendKey("j")

	if m.artistCursor.Pos() != initial+1 {
		t.Errorf("cursor = %d, want %d", m.artistCursor.Pos(), initial+1)
	}
}

func TestDownload_NavigateArtistResultsWithK(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateArtistResults
	m.artistResults = []musicbrainz.Artist{
		{Name: "Artist 1"},
		{Name: "Artist 2"},
	}
	m.artistCursor.SetPos(1)

	h.SendKey("k")

	if m.artistCursor.Pos() != 0 {
		t.Errorf("cursor = %d, want 0", m.artistCursor.Pos())
	}
}

func TestDownload_NavigateReleaseGroupsWithArrows(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateReleaseGroupResults
	m.releaseGroups = []musicbrainz.ReleaseGroup{
		{Title: "Album 1"},
		{Title: "Album 2"},
	}

	h.SendDown()

	if m.releaseGroupCursor.Pos() != 1 {
		t.Errorf("cursor = %d, want 1", m.releaseGroupCursor.Pos())
	}
}

func TestDownload_NavigateSlskdResultsWithJ(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateSlskdResults
	m.slskdResults = []SlskdResult{
		{Username: "user1"},
		{Username: "user2"},
	}

	h.SendKey("j")

	if m.slskdCursor.Pos() != 1 {
		t.Errorf("cursor = %d, want 1", m.slskdCursor.Pos())
	}
}

// === Message Handling Tests ===

func TestDownload_ArtistSearchResultMsg(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateArtistSearching

	artists := []musicbrainz.Artist{
		{Name: "Artist 1"},
		{Name: "Artist 2"},
	}
	h.SendMsg(workflow.ArtistSearchResultMsg{Artists: artists})

	if m.state != StateArtistResults {
		t.Errorf("state = %d, want StateArtistResults", m.state)
	}
	if len(m.artistResults) != 2 {
		t.Errorf("artistResults count = %d, want 2", len(m.artistResults))
	}
}

func TestDownload_ArtistSearchResultMsgWithError(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateArtistSearching

	h.SendMsg(workflow.ArtistSearchResultMsg{Err: errTest})

	if m.state != StateSearch {
		t.Errorf("state = %d, want StateSearch", m.state)
	}
	if m.errorMsg == "" {
		t.Error("errorMsg should be set on error")
	}
}

func TestDownload_ReleaseGroupResultMsg(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateReleaseGroupLoading

	groups := []musicbrainz.ReleaseGroup{
		{Title: "Album 1", PrimaryType: "Album"},
	}
	h.SendMsg(workflow.SearchResultMsg{ReleaseGroups: groups})

	if m.state != StateReleaseGroupResults {
		t.Errorf("state = %d, want StateReleaseGroupResults", m.state)
	}
}

func TestDownload_SlskdSearchResultMsg(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateSlskdSearching

	responses := []slskd.SearchResponse{
		{Username: "user1"},
	}
	h.SendMsg(SlskdSearchResultMsg{RawResponses: responses})

	if m.state != StateSlskdResults {
		t.Errorf("state = %d, want StateSlskdResults", m.state)
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

// === View Tests ===

func TestDownload_ViewShowsSearchPrompt(t *testing.T) {
	h := newDownloadPopup()

	if err := h.AssertViewContains("Search artist"); err != "" {
		t.Error(err)
	}
}

func TestDownload_ViewShowsArtistResults(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateArtistResults
	m.artistResults = []musicbrainz.Artist{
		{Name: "The Beatles"},
	}

	if err := h.AssertViewContains("The Beatles"); err != "" {
		t.Error(err)
	}
}

func TestDownload_ViewShowsReleaseGroups(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateReleaseGroupResults
	m.selectedArtist = &musicbrainz.Artist{Name: "Test Artist"}
	m.releaseGroups = []musicbrainz.ReleaseGroup{
		{Title: "Abbey Road", FirstRelease: "1969"},
	}

	if err := h.AssertViewContains("Abbey Road"); err != "" {
		t.Error(err)
	}
}

func TestDownload_ViewShowsSlskdResults(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateSlskdResults
	m.selectedReleaseGroup = &musicbrainz.ReleaseGroup{Title: "Test Album"}
	m.slskdResults = []SlskdResult{
		{Username: "testuser123", Format: "FLAC", FileCount: 10},
	}

	if err := h.AssertViewContains("testuser123"); err != "" {
		t.Error(err)
	}
}

func TestDownload_ViewShowsSearching(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateArtistSearching
	m.searchQuery = "Test"

	if err := h.AssertViewContains("Searching"); err != "" {
		t.Error(err)
	}
}

func TestDownload_ViewShowsError(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.errorMsg = "Something went wrong"

	if err := h.AssertViewContains("Something went wrong"); err != "" {
		t.Error(err)
	}
}

// === Model Accessor Tests ===

func TestDownload_SelectedReleaseGroup(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)

	if m.SelectedReleaseGroup() != nil {
		t.Error("SelectedReleaseGroup() should be nil initially")
	}

	rg := &musicbrainz.ReleaseGroup{Title: "Test"}
	m.selectedReleaseGroup = rg

	if m.SelectedReleaseGroup() != rg {
		t.Error("SelectedReleaseGroup() should return the set value")
	}
}

func TestDownload_IsDownloadComplete(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)

	if m.IsDownloadComplete() {
		t.Error("IsDownloadComplete() should be false initially")
	}

	m.downloadComplete = true

	if !m.IsDownloadComplete() {
		t.Error("IsDownloadComplete() should be true after setting")
	}
}

// === Size and Layout Tests ===

func TestDownload_SetSize(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)

	m.SetSize(120, 50)

	if m.Width() != 120 {
		t.Errorf("Width() = %d, want 120", m.Width())
	}
	if m.Height() != 50 {
		t.Errorf("Height() = %d, want 50", m.Height())
	}
}

func TestDownload_IsCompactMode(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)

	m.SetSize(80, 40)
	if !m.isCompactMode() {
		t.Error("isCompactMode() should be true for width < 90")
	}

	m.SetSize(100, 40)
	if m.isCompactMode() {
		t.Error("isCompactMode() should be false for width >= 90")
	}
}

// === Init Tests ===

func TestDownload_InitReturnsBlinkCmd(t *testing.T) {
	m := New("", "", FilterConfig{}, nil)
	m.SetSize(100, 40)

	cmd := m.Init()

	if cmd == nil {
		t.Error("Init() should return a command for text input blink")
	}
}

// === Back Navigation Tests ===

func TestDownload_BackspaceFromReleaseGroupGoesToArtists(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateReleaseGroupResults
	m.artistResults = []musicbrainz.Artist{{Name: "Test"}}

	h.SendKey("backspace")

	if m.state != StateArtistResults {
		t.Errorf("state = %d, want StateArtistResults", m.state)
	}
}

func TestDownload_BackspaceFromSlskdGoesToReleaseGroups(t *testing.T) {
	h := newDownloadPopup()
	m := getModel(t, h)
	m.state = StateSlskdResults
	m.releaseGroups = []musicbrainz.ReleaseGroup{{Title: "Test"}}

	h.SendKey("backspace")

	if m.state != StateReleaseGroupResults {
		t.Errorf("state = %d, want StateReleaseGroupResults", m.state)
	}
}
