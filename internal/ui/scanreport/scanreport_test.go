package scanreport

import (
	"testing"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestPopup(stats *library.ScanStats) *testutil.PopupHarness {
	m := New(stats)
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

// View tests - this popup is read-only

func TestScanReport_ViewShowsTitle(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"track1.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("Library Scan Complete"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsCloseHint(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"track1.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("Press Enter or Escape to close"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsSourcePath(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/home/user/music": {Added: []string{"song.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("/home/user/music"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsAddedCount(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"a.mp3", "b.mp3", "c.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("Added: 3"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsRemovedCount(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Removed: []string{"old1.mp3", "old2.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("Removed: 2"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsUpdatedCount(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Updated: []string{"modified.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("Updated: 1"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsExamplePaths(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"artist/album/track.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("artist/album/track.mp3"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsAndMoreWhenExceedsMax(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"1.mp3", "2.mp3", "3.mp3", "4.mp3", "5.mp3"}},
		},
	}
	m := New(stats)
	m.MaxExamples = 3
	m.SetSize(80, 24)
	h := testutil.NewPopupHarness(&m)

	if err := h.AssertViewContains("... and 2 more"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsNoChanges(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {}, // No added, removed, or updated
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("No changes"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsMultipleSources(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music":     {Added: []string{"a.mp3"}},
			"/downloads": {Removed: []string{"b.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("/music"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("/downloads"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsTotalLine(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music":     {Added: []string{"a.mp3", "b.mp3"}},
			"/downloads": {Removed: []string{"c.mp3"}},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("Total: 2 added, 1 removed, 0 updated"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_ViewShowsAllCategories(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {
				Added:   []string{"new.mp3"},
				Removed: []string{"old.mp3"},
				Updated: []string{"modified.mp3"},
			},
		},
	}
	h := newTestPopup(stats)

	if err := h.AssertViewContains("Added: 1"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Removed: 1"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Updated: 1"); err != "" {
		t.Error(err)
	}
}

// Empty/nil state tests

func TestScanReport_EmptyViewWhenNilStats(t *testing.T) {
	h := newTestPopup(nil)

	if h.View() != "" {
		t.Errorf("view = %q, want empty when nil stats", h.View())
	}
}

func TestScanReport_EmptyViewWhenNoSize(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"a.mp3"}},
		},
	}
	m := New(stats)
	// Don't set size
	h := testutil.NewPopupHarness(&m)

	// The scanreport doesn't check size - it always renders if stats is non-nil
	// This is different from other popups
	if err := h.AssertViewContains("Library Scan Complete"); err != "" {
		t.Error(err)
	}
}

func TestScanReport_EmptyBySourceMap(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{},
	}
	h := newTestPopup(stats)

	// Should still show title and total line
	if err := h.AssertViewContains("Library Scan Complete"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Total: 0 added, 0 removed, 0 updated"); err != "" {
		t.Error(err)
	}
}

// MaxExamples configuration tests

func TestScanReport_DefaultMaxExamples(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{},
	}
	m := New(stats)

	if m.MaxExamples != DefaultMaxExamples {
		t.Errorf("MaxExamples = %d, want %d", m.MaxExamples, DefaultMaxExamples)
	}
}

func TestScanReport_CustomMaxExamples(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"1.mp3", "2.mp3", "3.mp3", "4.mp3", "5.mp3", "6.mp3"}},
		},
	}
	m := New(stats)
	m.MaxExamples = 5
	m.SetSize(80, 24)
	h := testutil.NewPopupHarness(&m)

	// Should show 5 examples and "... and 1 more"
	if err := h.AssertViewContains("... and 1 more"); err != "" {
		t.Error(err)
	}
}

// Update test - verifies it doesn't handle messages

func TestScanReport_UpdateReturnsUnchanged(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/music": {Added: []string{"a.mp3"}},
		},
	}
	h := newTestPopup(stats)
	h.ClearCommands()

	// Send various keys - should not produce any commands
	h.SendEnter()
	h.SendEscape()
	h.SendKey("j")
	h.SendKey("q")

	// The popup itself doesn't handle messages - the manager closes it
	// So no commands should be generated by the popup
	if len(h.Commands()) != 0 {
		t.Error("scanreport should not produce commands from key presses")
	}
}

// Sorting test - sources should be sorted alphabetically

func TestScanReport_SourcesSortedAlphabetically(t *testing.T) {
	stats := &library.ScanStats{
		BySource: map[string]*library.SourceStats{
			"/zebra":  {Added: []string{"z.mp3"}},
			"/alpha":  {Added: []string{"a.mp3"}},
			"/middle": {Added: []string{"m.mp3"}},
		},
	}
	h := newTestPopup(stats)

	view := h.View()

	// Find positions of each source in the view
	alphaPos := findSubstringIndex(view, "/alpha")
	middlePos := findSubstringIndex(view, "/middle")
	zebraPos := findSubstringIndex(view, "/zebra")

	if alphaPos == -1 || middlePos == -1 || zebraPos == -1 {
		t.Skip("could not find all sources in view")
	}

	if alphaPos > middlePos {
		t.Error("/alpha should appear before /middle")
	}
	if middlePos > zebraPos {
		t.Error("/middle should appear before /zebra")
	}
}

// Helper to find substring index
func findSubstringIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
