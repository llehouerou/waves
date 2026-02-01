package downloads

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	dl "github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// Helper to create test downloads
func sampleDownload(id int64, artist, album, status string) dl.Download {
	return dl.Download{
		ID:           id,
		MBArtistName: artist,
		MBAlbumTitle: album,
		Status:       status,
		Files: []dl.DownloadFile{
			{ID: id*10 + 1, DownloadID: id, Filename: "01-track.mp3", Size: 5000000, Status: status},
			{ID: id*10 + 2, DownloadID: id, Filename: "02-track.mp3", Size: 5000000, Status: status},
		},
	}
}

func sampleDownloads() []dl.Download {
	return []dl.Download{
		sampleDownload(1, "Artist One", "Album One", dl.StatusCompleted),
		sampleDownload(2, "Artist Two", "Album Two", dl.StatusDownloading),
		sampleDownload(3, "Artist Three", "Album Three", dl.StatusPending),
	}
}

func sendKey(m *Model, key string) {
	*m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func sendSpecialKey(m *Model, keyType tea.KeyType) {
	*m, _ = m.Update(tea.KeyMsg{Type: keyType})
}

func getAction(t *testing.T, m *Model, key string) action.Action {
	t.Helper()
	var cmd tea.Cmd
	*m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	msg := testutil.ExecuteCmd(cmd)
	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}
	return actionMsg.Action
}

// Model creation and configuration tests

func TestDownloads_New(t *testing.T) {
	m := New()

	if m.IsConfigured() {
		t.Error("new model should not be configured by default")
	}
	if m.IsFocused() {
		t.Error("new model should not be focused by default")
	}
	if !m.IsEmpty() {
		t.Error("new model should be empty")
	}
}

func TestDownloads_SetConfigured(t *testing.T) {
	m := New()

	m.SetConfigured(true)
	if !m.IsConfigured() {
		t.Error("should be configured after SetConfigured(true)")
	}

	m.SetConfigured(false)
	if m.IsConfigured() {
		t.Error("should not be configured after SetConfigured(false)")
	}
}

func TestDownloads_SetFocused(t *testing.T) {
	m := New()

	m.SetFocused(true)
	if !m.IsFocused() {
		t.Error("should be focused after SetFocused(true)")
	}

	m.SetFocused(false)
	if m.IsFocused() {
		t.Error("should not be focused after SetFocused(false)")
	}
}

func TestDownloads_SetSize(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	if m.Width() != 80 {
		t.Errorf("Width() = %d, want 80", m.Width())
	}
	if m.Height() != 24 {
		t.Errorf("Height() = %d, want 24", m.Height())
	}
}

// Download list management tests

func TestDownloads_SetDownloads(t *testing.T) {
	m := New()
	downloads := sampleDownloads()

	m.SetDownloads(downloads)

	if m.IsEmpty() {
		t.Error("should not be empty after SetDownloads")
	}
	if len(m.Downloads()) != 3 {
		t.Errorf("Downloads() len = %d, want 3", len(m.Downloads()))
	}
}

func TestDownloads_SelectedDownload(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	downloads := sampleDownloads()
	m.SetDownloads(downloads)

	selected := m.SelectedDownload()
	if selected == nil {
		t.Fatal("SelectedDownload() should not be nil")
	}
	if selected.ID != 1 {
		t.Errorf("SelectedDownload().ID = %d, want 1", selected.ID)
	}
}

func TestDownloads_SelectedDownloadEmpty(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	if m.SelectedDownload() != nil {
		t.Error("SelectedDownload() should be nil when empty")
	}
}

// Navigation tests

func TestDownloads_NavigateWithJ(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	sendKey(&m, "j")

	selected := m.SelectedDownload()
	if selected == nil {
		t.Fatal("SelectedDownload() should not be nil")
	}
	if selected.ID != 2 {
		t.Errorf("after j, SelectedDownload().ID = %d, want 2", selected.ID)
	}
}

func TestDownloads_NavigateWithK(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	sendKey(&m, "j") // Move to 2
	sendKey(&m, "k") // Back to 1

	selected := m.SelectedDownload()
	if selected == nil {
		t.Fatal("SelectedDownload() should not be nil")
	}
	if selected.ID != 1 {
		t.Errorf("after j,k, SelectedDownload().ID = %d, want 1", selected.ID)
	}
}

func TestDownloads_NavigateWithArrows(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	sendSpecialKey(&m, tea.KeyDown)

	selected := m.SelectedDownload()
	if selected == nil {
		t.Fatal("SelectedDownload() should not be nil")
	}
	if selected.ID != 2 {
		t.Errorf("after down arrow, SelectedDownload().ID = %d, want 2", selected.ID)
	}
}

// Toggle expanded tests

func TestDownloads_ToggleExpanded(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	// Initially not expanded
	if m.isExpanded(1) {
		t.Error("download should not be expanded initially")
	}

	// Press enter to toggle
	sendSpecialKey(&m, tea.KeyEnter)

	if !m.isExpanded(1) {
		t.Error("download should be expanded after enter")
	}

	// Press enter again to collapse
	sendSpecialKey(&m, tea.KeyEnter)

	if m.isExpanded(1) {
		t.Error("download should be collapsed after second enter")
	}
}

// Action tests

func TestDownloads_DeleteAction(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	act := getAction(t, &m, "d")

	del, ok := act.(DeleteDownload)
	if !ok {
		t.Fatalf("expected DeleteDownload, got %T", act)
	}
	if del.ID != 1 {
		t.Errorf("DeleteDownload.ID = %d, want 1", del.ID)
	}
}

func TestDownloads_ClearCompletedAction(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	act := getAction(t, &m, "D")

	if _, ok := act.(ClearCompleted); !ok {
		t.Fatalf("expected ClearCompleted, got %T", act)
	}
}

func TestDownloads_RefreshAction(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	act := getAction(t, &m, "r")

	if _, ok := act.(RefreshRequest); !ok {
		t.Fatalf("expected RefreshRequest, got %T", act)
	}
}

func TestDownloads_ImportActionReady(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)

	// Create a download that's ready for import (all files completed and verified)
	downloads := []dl.Download{{
		ID:           1,
		MBArtistName: "Artist",
		MBAlbumTitle: "Album",
		Status:       dl.StatusCompleted,
		Files: []dl.DownloadFile{
			{ID: 1, Filename: "track.mp3", Status: dl.StatusCompleted, VerifiedOnDisk: true},
		},
	}}
	m.SetDownloads(downloads)

	act := getAction(t, &m, "i")

	openImport, ok := act.(OpenImport)
	if !ok {
		t.Fatalf("expected OpenImport, got %T", act)
	}
	if openImport.Download == nil {
		t.Error("OpenImport.Download should not be nil")
	}
}

func TestDownloads_ImportActionNotReady(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)

	// Create a download that's still downloading
	downloads := []dl.Download{{
		ID:           1,
		MBArtistName: "Artist",
		MBAlbumTitle: "Album",
		Status:       dl.StatusDownloading,
		Files: []dl.DownloadFile{
			{ID: 1, Filename: "track.mp3", Status: dl.StatusDownloading, Size: 1000, BytesRead: 500},
		},
	}}
	m.SetDownloads(downloads)

	act := getAction(t, &m, "i")

	notReady, ok := act.(ImportNotReady)
	if !ok {
		t.Fatalf("expected ImportNotReady, got %T", act)
	}
	if notReady.Reason == "" {
		t.Error("ImportNotReady.Reason should not be empty")
	}
}

// Import blocked reason tests

func TestDownloads_ImportBlockedNoFiles(t *testing.T) {
	m := New()
	d := &dl.Download{ID: 1, Files: nil}

	reason := m.importBlockedReason(d)
	if reason == "" {
		t.Error("should be blocked when no files")
	}
	if !strings.Contains(reason, "no files") {
		t.Errorf("reason should mention no files, got: %s", reason)
	}
}

func TestDownloads_ImportBlockedDownloading(t *testing.T) {
	m := New()
	d := &dl.Download{
		ID: 1,
		Files: []dl.DownloadFile{
			{Status: dl.StatusDownloading},
			{Status: dl.StatusCompleted},
		},
	}

	reason := m.importBlockedReason(d)
	if reason == "" {
		t.Error("should be blocked when downloading")
	}
	if !strings.Contains(reason, "downloading") {
		t.Errorf("reason should mention downloading, got: %s", reason)
	}
}

func TestDownloads_ImportBlockedPending(t *testing.T) {
	m := New()
	d := &dl.Download{
		ID: 1,
		Files: []dl.DownloadFile{
			{Status: dl.StatusPending},
		},
	}

	reason := m.importBlockedReason(d)
	if reason == "" {
		t.Error("should be blocked when pending")
	}
	if !strings.Contains(reason, "Waiting") {
		t.Errorf("reason should mention waiting, got: %s", reason)
	}
}

func TestDownloads_ImportBlockedFailed(t *testing.T) {
	m := New()
	d := &dl.Download{
		ID: 1,
		Files: []dl.DownloadFile{
			{Status: dl.StatusFailed},
			{Status: dl.StatusCompleted},
		},
	}

	reason := m.importBlockedReason(d)
	if reason == "" {
		t.Error("should be blocked when files failed")
	}
	if !strings.Contains(reason, "failed") {
		t.Errorf("reason should mention failed, got: %s", reason)
	}
}

func TestDownloads_ImportBlockedNotVerified(t *testing.T) {
	m := New()
	d := &dl.Download{
		ID: 1,
		Files: []dl.DownloadFile{
			{Status: dl.StatusCompleted, VerifiedOnDisk: false},
		},
	}

	reason := m.importBlockedReason(d)
	if reason == "" {
		t.Error("should be blocked when not verified")
	}
	if !strings.Contains(reason, "Verifying") {
		t.Errorf("reason should mention verifying, got: %s", reason)
	}
}

func TestDownloads_ImportReady(t *testing.T) {
	m := New()
	d := &dl.Download{
		ID: 1,
		Files: []dl.DownloadFile{
			{Status: dl.StatusCompleted, VerifiedOnDisk: true},
			{Status: dl.StatusCompleted, VerifiedOnDisk: true},
		},
	}

	reason := m.importBlockedReason(d)
	if reason != "" {
		t.Errorf("should be ready for import, but got reason: %s", reason)
	}

	if !m.isReadyForImport(d) {
		t.Error("isReadyForImport should return true")
	}
}

// View tests

func TestDownloads_ViewZeroSize(t *testing.T) {
	m := New()
	// Don't set size

	if m.View() != "" {
		t.Error("View() should return empty string with zero size")
	}
}

func TestDownloads_ViewNotConfigured(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetConfigured(false)

	view := testutil.StripANSI(m.View())

	if !strings.Contains(view, "not configured") {
		t.Error("view should show 'not configured' message")
	}
	if !strings.Contains(view, "config.toml") {
		t.Error("view should show config.toml path")
	}
}

func TestDownloads_ViewEmpty(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetConfigured(true)

	view := testutil.StripANSI(m.View())

	if !strings.Contains(view, "No downloads") {
		t.Error("view should show 'No downloads' when empty")
	}
}

func TestDownloads_ViewShowsHeader(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetConfigured(true)

	view := testutil.StripANSI(m.View())

	if !strings.Contains(view, "Downloads") {
		t.Error("view should show Downloads header")
	}
}

func TestDownloads_ViewShowsDownloads(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetConfigured(true)
	m.SetDownloads(sampleDownloads())

	view := testutil.StripANSI(m.View())

	if !strings.Contains(view, "Artist One") {
		t.Error("view should show Artist One")
	}
	if !strings.Contains(view, "Album One") {
		t.Error("view should show Album One")
	}
}

func TestDownloads_ViewShowsStatusCounts(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetConfigured(true)
	m.SetDownloads(sampleDownloads())

	view := testutil.StripANSI(m.View())

	// Should show status counts in header
	if !strings.Contains(view, "active") {
		t.Error("view should show active count")
	}
	if !strings.Contains(view, "done") {
		t.Error("view should show done count")
	}
	if !strings.Contains(view, "pending") {
		t.Error("view should show pending count")
	}
}

func TestDownloads_ViewExpandedShowsFiles(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetConfigured(true)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	// Expand first download
	sendSpecialKey(&m, tea.KeyEnter)

	view := testutil.StripANSI(m.View())

	// Should show file names when expanded
	if !strings.Contains(view, "01-track.mp3") {
		t.Error("expanded view should show file names")
	}
}

// Header text tests

func TestDownloads_BuildHeaderTextEmpty(t *testing.T) {
	m := New()

	header := m.buildHeaderText()
	if header != "Downloads" {
		t.Errorf("header = %q, want 'Downloads'", header)
	}
}

func TestDownloads_BuildHeaderTextWithDownloads(t *testing.T) {
	m := New()
	m.SetDownloads(sampleDownloads())

	header := m.buildHeaderText()

	if !strings.Contains(header, "Downloads") {
		t.Error("header should contain 'Downloads'")
	}
	if !strings.Contains(header, "1 active") {
		t.Errorf("header should show '1 active', got: %s", header)
	}
	if !strings.Contains(header, "1 done") {
		t.Errorf("header should show '1 done', got: %s", header)
	}
	if !strings.Contains(header, "1 pending") {
		t.Errorf("header should show '1 pending', got: %s", header)
	}
}

func TestDownloads_BuildHeaderTextFailed(t *testing.T) {
	m := New()
	m.SetDownloads([]dl.Download{
		sampleDownload(1, "Artist", "Album", dl.StatusFailed),
	})

	header := m.buildHeaderText()

	if !strings.Contains(header, "1 failed") {
		t.Errorf("header should show '1 failed', got: %s", header)
	}
}

// Expanded state cleanup tests

func TestDownloads_ExpandedStateCleanup(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFocused(true)
	m.SetDownloads(sampleDownloads())

	// Expand first download
	sendSpecialKey(&m, tea.KeyEnter)
	if !m.isExpanded(1) {
		t.Fatal("download 1 should be expanded")
	}

	// Update with new downloads that don't include ID 1
	m.SetDownloads([]dl.Download{
		sampleDownload(4, "New Artist", "New Album", dl.StatusPending),
	})

	// Old expanded state should be cleaned up
	if m.isExpanded(1) {
		t.Error("expanded state for removed download should be cleaned up")
	}
}

// Action types tests

func TestDownloads_ActionTypes(t *testing.T) {
	tests := []struct {
		action   action.Action
		expected string
	}{
		{DeleteDownload{ID: 1}, "downloads.delete"},
		{ClearCompleted{}, "downloads.clear_completed"},
		{RefreshRequest{}, "downloads.refresh"},
		{OpenImport{}, "downloads.open_import"},
		{ImportNotReady{}, "downloads.import_not_ready"},
	}

	for _, tt := range tests {
		if tt.action.ActionType() != tt.expected {
			t.Errorf("%T.ActionType() = %q, want %q", tt.action, tt.action.ActionType(), tt.expected)
		}
	}
}

// Not focused ignores custom keys

func TestDownloads_NotFocusedIgnoresKeys(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetConfigured(true)
	m.SetFocused(false)
	m.SetDownloads(sampleDownloads())

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})

	if cmd != nil {
		t.Error("unfocused model should not produce command for custom keys")
	}
}

// Extract year helper test

func TestDownloads_ExtractYear(t *testing.T) {
	tests := []struct {
		date     string
		expected string
	}{
		{"2024-01-15", "2024"},
		{"2024", "2024"},
		{"24", "24"},
		{"", ""},
	}

	for _, tt := range tests {
		result := extractYear(tt.date)
		if result != tt.expected {
			t.Errorf("extractYear(%q) = %q, want %q", tt.date, result, tt.expected)
		}
	}
}
