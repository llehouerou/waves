package librarysources

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestPopup(sources []string) *testutil.PopupHarness {
	m := New()
	m.SetSources(sources)
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

func getAction(t *testing.T, h *testutil.PopupHarness) action.Action {
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
	return actionMsg.Action
}

func assertClose(t *testing.T, h *testutil.PopupHarness) {
	t.Helper()
	act := getAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Fatalf("expected Close, got %T", act)
	}
}

// List mode tests

func TestListMode_Close(t *testing.T) {
	h := newTestPopup([]string{"/music", "/other"})

	h.SendEscape()

	assertClose(t, h)
}

func TestListMode_NavigateDown(t *testing.T) {
	sources := []string{"/first", "/second", "/third"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	h.SendDown()

	if m.SelectedPath() != "/second" {
		t.Errorf("SelectedPath = %q, want /second", m.SelectedPath())
	}
}

func TestListMode_NavigateWithJK(t *testing.T) {
	sources := []string{"/first", "/second", "/third"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	h.SendKey("j") // -> /second
	h.SendKey("j") // -> /third
	h.SendKey("k") // -> /second

	if m.SelectedPath() != "/second" {
		t.Errorf("SelectedPath = %q, want /second", m.SelectedPath())
	}
}

func TestListMode_DeleteRequestsTrackCount(t *testing.T) {
	sources := []string{"/music", "/other"}
	h := newTestPopup(sources)

	h.SendKey("d")

	act := getAction(t, h)
	req, ok := act.(RequestTrackCount)
	if !ok {
		t.Fatalf("expected RequestTrackCount, got %T", act)
	}
	if req.Path != "/music" {
		t.Errorf("Path = %q, want /music", req.Path)
	}
}

func TestListMode_DeleteOnEmptyDoesNothing(t *testing.T) {
	h := newTestPopup(nil) // Empty sources
	h.ClearCommands()

	h.SendKey("d")

	if len(h.Commands()) != 0 {
		t.Error("delete on empty list should not produce command")
	}
}

func TestListMode_EnterAddMode(t *testing.T) {
	h := newTestPopup([]string{"/music"})

	h.SendKey("a")

	// View should show add mode content
	if err := h.AssertViewContains("Enter path"); err != "" {
		t.Error(err)
	}
}

// Add mode tests

func TestAddMode_CancelWithEscape(t *testing.T) {
	h := newTestPopup([]string{"/music"})

	h.SendKey("a") // Enter add mode
	h.SendKey("t")
	h.SendKey("e")
	h.SendKey("s")
	h.SendKey("t")
	h.SendEscape() // Cancel

	// Should be back in list mode
	if err := h.AssertViewContains("a: add"); err != "" {
		t.Error(err)
	}
}

func TestAddMode_EmptyInputReturnsToList(t *testing.T) {
	h := newTestPopup([]string{"/music"})
	h.ClearCommands()

	h.SendKey("a") // Enter add mode
	h.SendEnter()  // Submit empty

	// Should be back in list mode with no command
	if err := h.AssertViewContains("a: add"); err != "" {
		t.Error(err)
	}
	if len(h.Commands()) != 0 {
		t.Error("empty input should not produce command")
	}
}

func TestAddMode_ValidPathEmitsSourceAdded(t *testing.T) {
	// Create a real temp directory for validation
	tmpDir := t.TempDir()

	h := newTestPopup(nil)

	h.SendKey("a") // Enter add mode
	for _, c := range tmpDir {
		h.SendKey(string(c))
	}
	h.SendEnter()

	act := getAction(t, h)
	added, ok := act.(SourceAdded)
	if !ok {
		t.Fatalf("expected SourceAdded, got %T", act)
	}
	if added.Path != tmpDir {
		t.Errorf("Path = %q, want %q", added.Path, tmpDir)
	}
}

func TestAddMode_InvalidPathShowsError(t *testing.T) {
	h := newTestPopup(nil)

	h.SendKey("a") // Enter add mode
	// Type a path that doesn't exist
	for _, c := range "/nonexistent/path/12345" {
		h.SendKey(string(c))
	}
	h.SendEnter()

	if err := h.AssertViewContains("Path does not exist"); err != "" {
		t.Error(err)
	}
}

func TestAddMode_FilePathShowsError(t *testing.T) {
	// Create a temp file (not directory)
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	h := newTestPopup(nil)

	h.SendKey("a")
	for _, c := range tmpFile {
		h.SendKey(string(c))
	}
	h.SendEnter()

	if err := h.AssertViewContains("not a directory"); err != "" {
		t.Error(err)
	}
}

func TestAddMode_Backspace(t *testing.T) {
	h := newTestPopup(nil)

	h.SendKey("a") // Enter add mode
	h.SendKey("a")
	h.SendKey("b")
	h.SendKey("c")
	h.SendSpecialKey(tea.KeyBackspace)
	h.SendSpecialKey(tea.KeyBackspace)

	if err := h.AssertViewContains("> a"); err != "" {
		t.Error(err)
	}
}

func TestAddMode_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("could not get home directory")
	}

	// Create a temp dir in home to test tilde expansion
	tmpDir, err := os.MkdirTemp(home, "libtest")
	if err != nil {
		t.Skip("could not create temp dir in home")
	}
	defer os.RemoveAll(tmpDir)

	// Get the relative part after home
	relPath := tmpDir[len(home):]
	tildeInput := "~" + relPath

	h := newTestPopup(nil)

	h.SendKey("a")
	for _, c := range tildeInput {
		h.SendKey(string(c))
	}
	h.SendEnter()

	act := getAction(t, h)
	added, ok := act.(SourceAdded)
	if !ok {
		t.Fatalf("expected SourceAdded, got %T", act)
	}
	if added.Path != tmpDir {
		t.Errorf("Path = %q, want %q (tilde should expand)", added.Path, tmpDir)
	}
}

// Confirm mode tests

func TestConfirmMode_ConfirmWithY(t *testing.T) {
	sources := []string{"/music", "/other"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.EnterConfirmMode(10)

	h.SendKey("y")

	act := getAction(t, h)
	removed, ok := act.(SourceRemoved)
	if !ok {
		t.Fatalf("expected SourceRemoved, got %T", act)
	}
	if removed.Path != "/music" {
		t.Errorf("Path = %q, want /music", removed.Path)
	}
}

func TestConfirmMode_ConfirmWithUpperY(t *testing.T) {
	sources := []string{"/music"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.EnterConfirmMode(5)

	h.SendKey("Y")

	act := getAction(t, h)
	if _, ok := act.(SourceRemoved); !ok {
		t.Fatalf("expected SourceRemoved, got %T", act)
	}
}

func TestConfirmMode_CancelWithN(t *testing.T) {
	sources := []string{"/music"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.EnterConfirmMode(5)
	h.ClearCommands()

	h.SendKey("n")

	// Should return to list mode without emitting removal
	if len(h.Commands()) != 0 {
		t.Error("cancel should not produce command")
	}
	if err := h.AssertViewContains("a: add"); err != "" {
		t.Error(err)
	}
}

func TestConfirmMode_CancelWithEscape(t *testing.T) {
	sources := []string{"/music"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.EnterConfirmMode(5)
	h.ClearCommands()

	h.SendEscape()

	if len(h.Commands()) != 0 {
		t.Error("escape should not produce command")
	}
}

func TestConfirmMode_ShowsTrackCount(t *testing.T) {
	sources := []string{"/music"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.EnterConfirmMode(42)

	if err := h.AssertViewContains("42 tracks"); err != "" {
		t.Error(err)
	}
}

func TestConfirmMode_ZeroTracksMessage(t *testing.T) {
	sources := []string{"/music"}
	h := newTestPopup(sources)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.EnterConfirmMode(0)

	if err := h.AssertViewContains("No tracks will be affected"); err != "" {
		t.Error(err)
	}
}

// View tests

func TestView_ShowsTitle(t *testing.T) {
	h := newTestPopup([]string{"/music"})

	if err := h.AssertViewContains("Library Sources"); err != "" {
		t.Error(err)
	}
}

func TestView_ShowsSources(t *testing.T) {
	h := newTestPopup([]string{"/music", "/downloads"})

	if err := h.AssertViewContains("/music"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("/downloads"); err != "" {
		t.Error(err)
	}
}

func TestView_EmptySourcesMessage(t *testing.T) {
	h := newTestPopup(nil)

	if err := h.AssertViewContains("No sources configured"); err != "" {
		t.Error(err)
	}
}

func TestView_EmptyWhenNoSize(t *testing.T) {
	m := New()
	m.SetSources([]string{"/music"})
	// Don't set size
	h := testutil.NewPopupHarness(&m)

	if h.View() != "" {
		t.Errorf("view = %q, want empty when no size", h.View())
	}
}

// Reset test

func TestReset(t *testing.T) {
	m := New()
	m.SetSources([]string{"/music"})
	m.SetSize(80, 24)

	// Enter add mode and type something
	h := testutil.NewPopupHarness(&m)
	h.SendKey("a")
	h.SendKey("t")
	h.SendKey("e")
	h.SendKey("s")
	h.SendKey("t")

	m.Reset()

	// Should be back to list mode
	if err := h.AssertViewContains("a: add"); err != "" {
		t.Error(err)
	}
}
