package popup

import (
	"testing"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// sampleDownload returns a download with MusicBrainz data for testing.
func sampleDownload() *downloads.Download {
	return &downloads.Download{
		ID:             1,
		MBArtistName:   "Test Artist",
		MBAlbumTitle:   "Test Album",
		SlskdUsername:  "testuser",
		SlskdDirectory: "Test Artist - Test Album",
		Files: []downloads.DownloadFile{
			{ID: 1, Filename: "01 - Track One.flac", Status: downloads.StatusCompleted},
			{ID: 2, Filename: "02 - Track Two.flac", Status: downloads.StatusCompleted},
			{ID: 3, Filename: "03 - Track Three.flac", Status: downloads.StatusCompleted},
		},
		MBReleaseDetails: &musicbrainz.ReleaseDetails{
			Release: musicbrainz.Release{
				ID:     "abc123",
				Title:  "Test Album",
				Artist: "Test Artist",
				Date:   "2024-01-15",
			},
			Tracks: []musicbrainz.Track{
				{Position: 1, Title: "Track One"},
				{Position: 2, Title: "Track Two"},
				{Position: 3, Title: "Track Three"},
			},
		},
		MBReleaseGroup: &musicbrainz.ReleaseGroup{
			ID:           "rg123",
			Title:        "Test Album",
			PrimaryType:  "Album",
			FirstRelease: "2024-01-15",
		},
	}
}

func sampleLibrarySources() []string {
	return []string{"/music/library1", "/music/library2"}
}

func newImportPopup() *testutil.PopupHarness {
	m := New(sampleDownload(), "/downloads/complete", sampleLibrarySources(), nil, rename.Config{})
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m)
}

func newImportPopupWithTags() *testutil.PopupHarness {
	h := newImportPopup()
	// Simulate tags being read
	h.SendMsg(TagsReadMsg{Tags: nil, Err: nil})
	return h
}

func getImportAction(t *testing.T, h *testutil.PopupHarness) action.Action {
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

func getModel(t *testing.T, h *testutil.PopupHarness) *Model {
	t.Helper()
	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	return m
}

// === Close Tests ===

func TestImport_CloseWithEscapeFromTagPreview(t *testing.T) {
	h := newImportPopupWithTags()

	h.SendEscape()

	act := getImportAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Errorf("expected Close action, got %T", act)
	}
}

func TestImport_CloseWithEscapeFromComplete(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateComplete

	h.SendEscape()

	act := getImportAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Errorf("expected Close action, got %T", act)
	}
}

func TestImport_CloseWithEnterFromComplete(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateComplete

	h.SendEnter()

	act := getImportAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Errorf("expected Close action, got %T", act)
	}
}

// === State Transition Tests ===

func TestImport_EnterFromTagPreviewGoesToPathPreview(t *testing.T) {
	h := newImportPopupWithTags()

	h.SendEnter()

	m := getModel(t, h)
	if m.state != StatePathPreview {
		t.Errorf("state = %d, want StatePathPreview (%d)", m.state, StatePathPreview)
	}
}

func TestImport_EscapeFromPathPreviewGoesBack(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	h.SendEscape()

	m := getModel(t, h)
	if m.state != StateTagPreview {
		t.Errorf("state = %d, want StateTagPreview (%d)", m.state, StateTagPreview)
	}
}

func TestImport_EnterFromPathPreviewWithCoverArtStartsImport(t *testing.T) {
	h := newImportPopupWithTags()
	m := getModel(t, h)
	m.coverArtFetched = true
	h.SendEnter() // Go to path preview

	h.SendEnter() // Start import

	if m.state != StateImporting {
		t.Errorf("state = %d, want StateImporting (%d)", m.state, StateImporting)
	}
}

func TestImport_EnterFromPathPreviewWithoutCoverArtDoesNothing(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	m := getModel(t, h)
	m.coverArtFetched = false

	h.SendEnter()

	if m.state != StatePathPreview {
		t.Errorf("state = %d, want StatePathPreview (cover art not fetched)", m.state)
	}
}

// === Navigation Tests ===

func TestImport_NavigateLibrarySourcesWithJ(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	m := getModel(t, h)
	initial := m.selectedSource

	h.SendKey("j")

	if m.selectedSource != (initial+1)%len(m.librarySources) {
		t.Errorf("selectedSource = %d, want %d", m.selectedSource, (initial+1)%len(m.librarySources))
	}
}

func TestImport_NavigateLibrarySourcesWithK(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	m := getModel(t, h)
	m.selectedSource = 1

	h.SendKey("k")

	if m.selectedSource != 0 {
		t.Errorf("selectedSource = %d, want 0", m.selectedSource)
	}
}

func TestImport_NavigateWrapsAround(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	m := getModel(t, h)
	m.selectedSource = 0

	h.SendKey("k")

	expected := len(m.librarySources) - 1
	if m.selectedSource != expected {
		t.Errorf("selectedSource = %d, want %d", m.selectedSource, expected)
	}
}

func TestImport_NavigateInTagPreviewDoesNothing(t *testing.T) {
	h := newImportPopupWithTags()
	m := getModel(t, h)
	initial := m.selectedSource

	h.SendKey("j")

	if m.selectedSource != initial {
		t.Errorf("selectedSource changed in tag preview state")
	}
}

// === Message Handling Tests ===

func TestImport_TagsReadMsgBuildsDiffs(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)

	h.SendMsg(TagsReadMsg{Tags: nil, Err: nil})

	// tagDiffs should be built after tags are read
	if m.tagDiffs == nil {
		t.Error("tagDiffs should be built after TagsReadMsg")
	}
}

func TestImport_CoverArtFetchedMsgSetsFetched(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)

	h.SendMsg(CoverArtFetchedMsg{Data: []byte("test"), Err: nil})

	if !m.coverArtFetched {
		t.Error("coverArtFetched should be true")
	}
	if string(m.coverArt) != "test" {
		t.Errorf("coverArt = %q, want 'test'", string(m.coverArt))
	}
}

func TestImport_CoverArtFetchedMsgWithNilDataIsOK(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)

	h.SendMsg(CoverArtFetchedMsg{Data: nil, Err: nil})

	if !m.coverArtFetched {
		t.Error("coverArtFetched should be true even with nil data")
	}
}

func TestImport_LibraryRefreshedMsgGoesToComplete(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateImporting

	h.SendMsg(LibraryRefreshedMsg{})

	if m.state != StateComplete {
		t.Errorf("state = %d, want StateComplete", m.state)
	}
}

func TestImport_FileImportedMsgUpdatesStatus(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateImporting

	h.SendMsg(FileImportedMsg{Index: 0, DestPath: "/music/test.flac", Err: nil})

	if m.importStatus[0].Status != StatusComplete {
		t.Errorf("status = %d, want StatusComplete", m.importStatus[0].Status)
	}
	if m.successCount != 1 {
		t.Errorf("successCount = %d, want 1", m.successCount)
	}
}

func TestImport_FileImportedMsgWithErrorTracksFailure(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateImporting
	// Mark other files as complete to trigger completion
	m.importStatus[1].Status = StatusComplete
	m.importStatus[2].Status = StatusComplete

	h.SendMsg(FileImportedMsg{Index: 0, Err: errTest})

	if m.importStatus[0].Status != StatusFailed {
		t.Errorf("status = %d, want StatusFailed", m.importStatus[0].Status)
	}
	if len(m.failedFiles) != 1 {
		t.Errorf("failedFiles count = %d, want 1", len(m.failedFiles))
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

// === View Tests ===

func TestImport_ViewShowsTitle(t *testing.T) {
	h := newImportPopupWithTags()

	if err := h.AssertViewContains("Import:"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Test Artist"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Test Album"); err != "" {
		t.Error(err)
	}
}

func TestImport_ViewShowsStepIndicator(t *testing.T) {
	h := newImportPopupWithTags()

	if err := h.AssertViewContains("Tags"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Paths"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Import"); err != "" {
		t.Error(err)
	}
}

func TestImport_TagPreviewShowsTagChangesHeader(t *testing.T) {
	h := newImportPopupWithTags()

	if err := h.AssertViewContains("Tag Changes Preview"); err != "" {
		t.Error(err)
	}
}

func TestImport_TagPreviewShowsCoverArtStatus(t *testing.T) {
	h := newImportPopupWithTags()

	if err := h.AssertViewContains("Cover Art:"); err != "" {
		t.Error(err)
	}
}

func TestImport_TagPreviewShowsFileCount(t *testing.T) {
	h := newImportPopupWithTags()

	if err := h.AssertViewContains("3 files will be retagged"); err != "" {
		t.Error(err)
	}
}

func TestImport_TagPreviewShowsHelpText(t *testing.T) {
	h := newImportPopupWithTags()

	if err := h.AssertViewContains("[Enter] Continue"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("[Esc] Cancel"); err != "" {
		t.Error(err)
	}
}

func TestImport_PathPreviewShowsDestination(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	if err := h.AssertViewContains("Destination Library:"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("/music/library1"); err != "" {
		t.Error(err)
	}
}

func TestImport_PathPreviewShowsFilePaths(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	if err := h.AssertViewContains("File Paths"); err != "" {
		t.Error(err)
	}
}

func TestImport_PathPreviewShowsSelectedLibrary(t *testing.T) {
	h := newImportPopupWithTags()
	h.SendEnter() // Go to path preview

	// First library should be selected (marked with >)
	if err := h.AssertViewContains("> /music/library1"); err != "" {
		t.Error(err)
	}
}

func TestImport_ImportingShowsProgress(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateImporting

	if err := h.AssertViewContains("Importing..."); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Progress:"); err != "" {
		t.Error(err)
	}
}

func TestImport_CompleteShowsSuccess(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateComplete
	m.successCount = 3

	if err := h.AssertViewContains("Import Complete"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("3 files imported successfully"); err != "" {
		t.Error(err)
	}
}

func TestImport_CompleteShowsErrors(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateComplete
	m.successCount = 2
	m.failedFiles = []FailedFile{{Filename: "bad.flac", Error: "test error"}}

	if err := h.AssertViewContains("Import Completed with Errors"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("1 files failed"); err != "" {
		t.Error(err)
	}
}

func TestImport_ViewShowsLoadingMBState(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.loadingMB = true

	if err := h.AssertViewContains("Refreshing MusicBrainz data..."); err != "" {
		t.Error(err)
	}
}

// === Init Tests ===

func TestImport_InitReturnsReadTagsCmd(t *testing.T) {
	download := sampleDownload()
	m := New(download, "/downloads/complete", sampleLibrarySources(), nil, rename.Config{})
	m.SetSize(100, 40)

	cmd := m.Init()

	if cmd == nil {
		t.Error("Init() should return a command")
	}
}

// === Model State Tests ===

func TestImport_IsCompleteReturnsTrueInCompleteState(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.state = StateComplete

	if !m.IsComplete() {
		t.Error("IsComplete() should return true in StateComplete")
	}
}

func TestImport_IsCompleteReturnsFalseOtherwise(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)

	if m.IsComplete() {
		t.Error("IsComplete() should return false in initial state")
	}
}

func TestImport_DownloadReturnsDownload(t *testing.T) {
	download := sampleDownload()
	m := New(download, "/downloads/complete", sampleLibrarySources(), nil, rename.Config{})

	if m.Download() != download {
		t.Error("Download() should return the download")
	}
}

func TestImport_SuccessCountReturnsCount(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.successCount = 5

	if m.SuccessCount() != 5 {
		t.Errorf("SuccessCount() = %d, want 5", m.SuccessCount())
	}
}

func TestImport_FailedCountReturnsCount(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)
	m.failedFiles = []FailedFile{{}, {}}

	if m.FailedCount() != 2 {
		t.Errorf("FailedCount() = %d, want 2", m.FailedCount())
	}
}

// === Helper Function Tests ===

func TestImport_BuildSourcePath(t *testing.T) {
	download := sampleDownload()
	file := &download.Files[0]

	path := BuildSourcePath("/downloads/complete", download, file)

	if path == "" {
		t.Error("BuildSourcePath should return a path")
	}
}

// === Edge Cases ===

func TestImport_EmptyViewWhenZeroSize(t *testing.T) {
	m := New(sampleDownload(), "/downloads/complete", sampleLibrarySources(), nil, rename.Config{})
	// Don't set size

	view := m.View()

	if view != "" {
		t.Errorf("View() with zero size should return empty string, got %q", view)
	}
}

func TestImport_SingleLibrarySourceShowsInline(t *testing.T) {
	download := sampleDownload()
	m := New(download, "/downloads/complete", []string{"/music/only"}, nil, rename.Config{})
	m.SetSize(100, 40)
	h := testutil.NewPopupHarness(m)
	h.SendMsg(TagsReadMsg{Tags: nil, Err: nil})
	h.SendEnter() // Go to path preview

	if err := h.AssertViewContains("Destination:"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("/music/only"); err != "" {
		t.Error(err)
	}
}

func TestImport_StateAccessor(t *testing.T) {
	h := newImportPopup()
	m := getModel(t, h)

	if m.State() != StateTagPreview {
		t.Errorf("State() = %d, want StateTagPreview", m.State())
	}

	m.state = StatePathPreview
	if m.State() != StatePathPreview {
		t.Errorf("State() = %d, want StatePathPreview", m.State())
	}
}

func TestImport_GetReleaseGroup(t *testing.T) {
	download := sampleDownload()
	m := New(download, "/downloads/complete", sampleLibrarySources(), nil, rename.Config{})

	rg := m.GetReleaseGroup()
	if rg == nil {
		t.Fatal("GetReleaseGroup() should return release group")
	}
	if rg.ID != "rg123" {
		t.Errorf("GetReleaseGroup().ID = %q, want 'rg123'", rg.ID)
	}
}

func TestImport_GetReleaseDetails(t *testing.T) {
	download := sampleDownload()
	m := New(download, "/downloads/complete", sampleLibrarySources(), nil, rename.Config{})

	rd := m.GetReleaseDetails()
	if rd == nil {
		t.Fatal("GetReleaseDetails() should return release details")
	}
	if rd.ID != "abc123" {
		t.Errorf("GetReleaseDetails().ID = %q, want 'abc123'", rd.ID)
	}
}
