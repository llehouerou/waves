package helpbindings

import (
	"testing"

	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestHelpPopup(contexts []string) *testutil.PopupHarness {
	m := New()
	m.SetContexts(contexts)
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

func assertClosed(t *testing.T, h *testutil.PopupHarness) {
	t.Helper()
	cmd := h.LastCommand()
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	msg := testutil.ExecuteCmd(cmd)
	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}
	if _, ok := actionMsg.Action.(Close); !ok {
		t.Fatalf("expected Close, got %T", actionMsg.Action)
	}
}

// Close tests

func TestHelpBindings_CloseWithEscape(t *testing.T) {
	h := newTestHelpPopup([]string{"global"})

	h.SendEscape()

	assertClosed(t, h)
}

func TestHelpBindings_CloseWithQ(t *testing.T) {
	h := newTestHelpPopup([]string{"global"})

	h.SendKey("q")

	assertClosed(t, h)
}

func TestHelpBindings_CloseWithQuestionMark(t *testing.T) {
	h := newTestHelpPopup([]string{"global"})

	h.SendKey("?")

	assertClosed(t, h)
}

// Scroll tests

func TestHelpBindings_ScrollDown(t *testing.T) {
	// Use multiple contexts to ensure enough content to scroll
	h := newTestHelpPopup([]string{"global", "playback", "navigator"})

	// Get the popup to check scroll offset
	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	initialOffset := m.scrollOffset

	h.SendDown()
	h.SendDown()

	if m.scrollOffset <= initialOffset {
		t.Error("scroll offset should increase after scrolling down")
	}
}

func TestHelpBindings_ScrollDownWithJ(t *testing.T) {
	h := newTestHelpPopup([]string{"global", "playback", "navigator"})

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	initialOffset := m.scrollOffset

	h.SendKey("j")
	h.SendKey("j")

	if m.scrollOffset <= initialOffset {
		t.Error("scroll offset should increase after pressing j")
	}
}

func TestHelpBindings_ScrollUp(t *testing.T) {
	h := newTestHelpPopup([]string{"global", "playback", "navigator"})

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// First scroll down
	h.SendDown()
	h.SendDown()
	h.SendDown()
	afterDown := m.scrollOffset

	// Then scroll up
	h.SendUp()

	if m.scrollOffset >= afterDown {
		t.Error("scroll offset should decrease after scrolling up")
	}
}

func TestHelpBindings_ScrollUpWithK(t *testing.T) {
	h := newTestHelpPopup([]string{"global", "playback", "navigator"})

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// First scroll down
	h.SendKey("j")
	h.SendKey("j")
	h.SendKey("j")
	afterDown := m.scrollOffset

	// Then scroll up with k
	h.SendKey("k")

	if m.scrollOffset >= afterDown {
		t.Error("scroll offset should decrease after pressing k")
	}
}

func TestHelpBindings_ScrollUpAtTopDoesNothing(t *testing.T) {
	h := newTestHelpPopup([]string{"global"})

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// Try to scroll up at top
	h.SendUp()
	h.SendUp()

	if m.scrollOffset != 0 {
		t.Errorf("scroll offset = %d, want 0 when at top", m.scrollOffset)
	}
}

// View tests

func TestHelpBindings_ViewShowsTitle(t *testing.T) {
	h := newTestHelpPopup([]string{"global"})

	if err := h.AssertViewContains("Help"); err != "" {
		t.Error(err)
	}
}

func TestHelpBindings_ViewShowsCloseHint(t *testing.T) {
	h := newTestHelpPopup([]string{"global"})

	if err := h.AssertViewContains("close"); err != "" {
		t.Error(err)
	}
}

func TestHelpBindings_ViewShowsCategoryHeader(t *testing.T) {
	h := newTestHelpPopup([]string{"playback"})

	if err := h.AssertViewContains("Playback"); err != "" {
		t.Error(err)
	}
}

func TestHelpBindings_ViewShowsMultipleCategories(t *testing.T) {
	// Use a larger size to fit both categories
	m := New()
	m.SetContexts([]string{"global", "playback"})
	m.SetSize(80, 100) // Large enough to show all content
	h := testutil.NewPopupHarness(&m)

	if err := h.AssertViewContains("Global"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Playback"); err != "" {
		t.Error(err)
	}
}

func TestHelpBindings_EmptyViewWhenNoSize(t *testing.T) {
	m := New()
	m.SetContexts([]string{"global"})
	// Don't set size
	h := testutil.NewPopupHarness(&m)

	if h.View() != "" {
		t.Errorf("view = %q, want empty when no size", h.View())
	}
}

// SetContexts tests

func TestHelpBindings_SetContextsResetsScroll(t *testing.T) {
	m := New()
	m.SetContexts([]string{"global", "playback", "navigator"})
	m.SetSize(80, 24)
	h := testutil.NewPopupHarness(&m)

	// Scroll down
	h.SendDown()
	h.SendDown()

	if m.scrollOffset == 0 {
		t.Skip("could not scroll down, skipping reset test")
	}

	// Set new contexts - should reset scroll
	m.SetContexts([]string{"global"})

	if m.scrollOffset != 0 {
		t.Errorf("scroll offset = %d after SetContexts, want 0", m.scrollOffset)
	}
}

func TestHelpBindings_SetContextsRespectsCategoryOrder(t *testing.T) {
	m := New()
	// Set contexts in non-standard order
	m.SetContexts([]string{"playback", "global"})
	m.SetSize(80, 100) // Large enough to show all content
	h := testutil.NewPopupHarness(&m)

	view := h.View()

	// Global should appear before Playback (categoryOrder defines the order)
	globalIdx := findSubstringIndex(view, "Global")
	playbackIdx := findSubstringIndex(view, "Playback")

	if globalIdx == -1 || playbackIdx == -1 {
		t.Skip("could not find categories in view")
	}

	if globalIdx > playbackIdx {
		t.Error("Global should appear before Playback regardless of SetContexts order")
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
