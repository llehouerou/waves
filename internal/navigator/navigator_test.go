//nolint:goconst // test file with many repeated string literals
package navigator

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockSourceWithParent extends mockSource with proper Parent implementation.
type mockSourceWithParent struct {
	mockSource
	parents map[string]mockNode
}

func (s *mockSourceWithParent) Parent(node mockNode) *mockNode {
	if parent, ok := s.parents[node.id]; ok {
		return &parent
	}
	return nil
}

// newTestSourceWithParent creates a test source with proper Parent implementation.
func newTestSourceWithParent() *mockSourceWithParent {
	root := mockNode{id: "root", name: "Root", isContainer: true, iconType: IconFolder}
	folder1 := mockNode{id: "folder1", name: "Folder1", isContainer: true, iconType: IconFolder}
	folder2 := mockNode{id: "folder2", name: "Folder2", isContainer: true, iconType: IconFolder}
	file1 := mockNode{id: "file1", name: "File1.mp3", isContainer: false, iconType: IconAudio}
	subfolder := mockNode{id: "subfolder", name: "SubFolder", isContainer: true, iconType: IconFolder}
	subfile := mockNode{id: "subfile", name: "SubFile.mp3", isContainer: false, iconType: IconAudio}

	return &mockSourceWithParent{
		mockSource: mockSource{
			root: root,
			children: map[string][]mockNode{
				"root":    {folder1, folder2, file1},
				"folder1": {subfolder, subfile},
			},
		},
		parents: map[string]mockNode{
			"folder1":   root,
			"folder2":   root,
			"file1":     root,
			"subfolder": folder1,
			"subfile":   folder1,
		},
	}
}

func newTestNavigatorWithParent(t *testing.T) Model[mockNode] {
	t.Helper()
	source := newTestSourceWithParent()
	nav, err := New[mockNode](source)
	if err != nil {
		t.Fatalf("failed to create navigator: %v", err)
	}
	nav.SetFocused(true)
	nav.width = 80
	nav.height = 20
	return nav
}

func TestNew(t *testing.T) {
	source := newTestSource()
	nav, err := New[mockNode](source)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if nav.current.ID() != "root" {
		t.Errorf("current = %q, want root", nav.current.ID())
	}

	if len(nav.currentItems) != 3 {
		t.Errorf("currentItems count = %d, want 3", len(nav.currentItems))
	}
}

func TestModel_SetFocused(t *testing.T) {
	nav := newTestNavigator(t)

	if !nav.IsFocused() {
		t.Error("should be focused after newTestNavigator")
	}

	nav.SetFocused(false)
	if nav.IsFocused() {
		t.Error("should be unfocused after SetFocused(false)")
	}

	nav.SetFocused(true)
	if !nav.IsFocused() {
		t.Error("should be focused after SetFocused(true)")
	}
}

func TestModel_Favorites(t *testing.T) {
	nav := newTestNavigator(t)

	// No favorites set
	if nav.IsFavorite(123) {
		t.Error("should not be favorite when no favorites set")
	}

	// Set favorites
	favs := map[int64]bool{123: true, 456: true}
	nav.SetFavorites(favs)

	if !nav.IsFavorite(123) {
		t.Error("123 should be favorite")
	}
	if !nav.IsFavorite(456) {
		t.Error("456 should be favorite")
	}
	if nav.IsFavorite(789) {
		t.Error("789 should not be favorite")
	}
}

func TestModel_Selected(t *testing.T) {
	nav := newTestNavigator(t)

	selected := nav.Selected()
	if selected == nil {
		t.Fatal("Selected() should not be nil")
	}

	if selected.ID() != "folder1" {
		t.Errorf("Selected ID = %q, want folder1", selected.ID())
	}
}

func TestModel_SelectedName(t *testing.T) {
	nav := newTestNavigator(t)

	name := nav.SelectedName()
	if name != "Folder1" {
		t.Errorf("SelectedName = %q, want Folder1", name)
	}
}

func TestModel_SelectedID(t *testing.T) {
	nav := newTestNavigator(t)

	id := nav.SelectedID()
	if id != "folder1" {
		t.Errorf("SelectedID = %q, want folder1", id)
	}
}

func TestModel_Selected_Empty(t *testing.T) {
	source := &mockSource{
		root:     mockNode{id: "root", name: "Root", isContainer: true},
		children: map[string][]mockNode{}, // No children
	}
	nav, err := New[mockNode](source)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if nav.Selected() != nil {
		t.Error("Selected() should be nil for empty list")
	}
	if nav.SelectedName() != "" {
		t.Error("SelectedName() should be empty for empty list")
	}
	if nav.SelectedID() != "" {
		t.Error("SelectedID() should be empty for empty list")
	}
}

func TestModel_CurrentPath(t *testing.T) {
	nav := newTestNavigator(t)

	path := nav.CurrentPath()
	if path != "/Root" {
		t.Errorf("CurrentPath = %q, want /Root", path)
	}
}

func TestModel_Current(t *testing.T) {
	nav := newTestNavigator(t)

	current := nav.Current()
	if current.ID() != "root" {
		t.Errorf("Current ID = %q, want root", current.ID())
	}
}

func TestModel_CurrentItems(t *testing.T) {
	nav := newTestNavigator(t)

	items := nav.CurrentItems()
	if len(items) != 3 {
		t.Errorf("CurrentItems count = %d, want 3", len(items))
	}
}

func TestModel_FocusByName(t *testing.T) {
	nav := newTestNavigator(t)

	// Initially on Folder1
	if nav.SelectedName() != "Folder1" {
		t.Errorf("initial selection = %q, want Folder1", nav.SelectedName())
	}

	// Focus on File1
	nav.FocusByName("File1.mp3")
	if nav.SelectedName() != "File1.mp3" {
		t.Errorf("after FocusByName, selection = %q, want File1.mp3", nav.SelectedName())
	}

	// Focus on non-existent item (should stay on current)
	nav.FocusByName("NonExistent")
	if nav.SelectedName() != "File1.mp3" {
		t.Errorf("after FocusByName non-existent, selection = %q, want File1.mp3", nav.SelectedName())
	}
}

func TestModel_SelectByID(t *testing.T) {
	nav := newTestNavigator(t)

	// Select folder2
	found := nav.SelectByID("folder2")
	if !found {
		t.Error("SelectByID should return true for existing item")
	}
	if nav.SelectedID() != "folder2" {
		t.Errorf("after SelectByID, selection = %q, want folder2", nav.SelectedID())
	}

	// Select non-existent
	found = nav.SelectByID("nonexistent")
	if found {
		t.Error("SelectByID should return false for non-existent item")
	}
	// Should stay on folder2
	if nav.SelectedID() != "folder2" {
		t.Errorf("after failed SelectByID, selection = %q, want folder2", nav.SelectedID())
	}
}

func TestModel_NavigateTo_Container(t *testing.T) {
	nav := newTestNavigator(t)

	// Navigate to folder1
	ok := nav.NavigateTo("folder1")
	if !ok {
		t.Error("NavigateTo should return true for existing container")
	}

	if nav.Current().ID() != "folder1" {
		t.Errorf("after NavigateTo, current = %q, want folder1", nav.Current().ID())
	}

	// Should now see folder1's children
	items := nav.CurrentItems()
	if len(items) != 2 {
		t.Errorf("folder1 should have 2 children, got %d", len(items))
	}
}

func TestModel_NavigateTo_NonExistent(t *testing.T) {
	nav := newTestNavigator(t)

	ok := nav.NavigateTo("nonexistent")
	if ok {
		t.Error("NavigateTo should return false for non-existent node")
	}

	// Should stay at root
	if nav.Current().ID() != "root" {
		t.Errorf("current should still be root, got %q", nav.Current().ID())
	}
}

func TestModel_FocusByID(t *testing.T) {
	nav := newTestNavigatorWithParent(t)

	// FocusByID on subfile should navigate to its parent (folder1) and focus
	ok := nav.FocusByID("subfile")
	if !ok {
		t.Error("FocusByID should return true")
	}

	if nav.Current().ID() != "folder1" {
		t.Errorf("current should be folder1, got %q", nav.Current().ID())
	}

	if nav.SelectedID() != "subfile" {
		t.Errorf("should be focused on subfile, got %q", nav.SelectedID())
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	nav := newTestNavigator(t)

	nav, _ = nav.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if nav.width != 120 {
		t.Errorf("width = %d, want 120", nav.width)
	}
	if nav.height != 40 {
		t.Errorf("height = %d, want 40", nav.height)
	}
}

func TestModel_Update_KeyNavigation_Down(t *testing.T) {
	nav := newTestNavigator(t)

	// Move down
	nav, cmd := nav.Update(tea.KeyMsg{Type: tea.KeyDown})

	if nav.SelectedID() != "folder2" {
		t.Errorf("after down, selection = %q, want folder2", nav.SelectedID())
	}

	// Should return navigation changed command
	if cmd == nil {
		t.Error("navigation should return a command")
	}
}

func TestModel_Update_KeyNavigation_Up(t *testing.T) {
	nav := newTestNavigator(t)

	// Move down first
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Then up
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyUp})

	if nav.SelectedID() != "folder1" {
		t.Errorf("after up, selection = %q, want folder1", nav.SelectedID())
	}
}

func TestModel_Update_KeyNavigation_J(t *testing.T) {
	nav := newTestNavigator(t)

	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	if nav.SelectedID() != "folder2" {
		t.Errorf("after j, selection = %q, want folder2", nav.SelectedID())
	}
}

func TestModel_Update_KeyNavigation_K(t *testing.T) {
	nav := newTestNavigator(t)

	// Move down first
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	// Then up with k
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	if nav.SelectedID() != "folder1" {
		t.Errorf("after k, selection = %q, want folder1", nav.SelectedID())
	}
}

func TestModel_Update_KeyNavigation_Right(t *testing.T) {
	nav := newTestNavigator(t)

	// Cursor is on folder1 which is a container
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRight})

	if nav.Current().ID() != "folder1" {
		t.Errorf("after right, current = %q, want folder1", nav.Current().ID())
	}
}

func TestModel_Update_KeyNavigation_Left(t *testing.T) {
	nav := newTestNavigatorWithParent(t)

	// First navigate into folder1
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRight})
	// Then go back
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyLeft})

	if nav.Current().ID() != "root" {
		t.Errorf("after left, current = %q, want root", nav.Current().ID())
	}

	// Should focus on folder1 (the folder we came from)
	if nav.SelectedID() != "folder1" {
		t.Errorf("after left, selection = %q, want folder1", nav.SelectedID())
	}
}

func TestModel_Update_KeyNavigation_L(t *testing.T) {
	nav := newTestNavigator(t)

	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

	if nav.Current().ID() != "folder1" {
		t.Errorf("after l, current = %q, want folder1", nav.Current().ID())
	}
}

func TestModel_Update_KeyNavigation_H(t *testing.T) {
	nav := newTestNavigatorWithParent(t)

	// Navigate in first
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	// Then back with h
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

	if nav.Current().ID() != "root" {
		t.Errorf("after h, current = %q, want root", nav.Current().ID())
	}
}

func TestModel_Update_KeyNavigation_Enter(t *testing.T) {
	nav := newTestNavigator(t)

	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if nav.Current().ID() != "folder1" {
		t.Errorf("after enter, current = %q, want folder1", nav.Current().ID())
	}
}

func TestModel_Update_KeyNavigation_Enter_NonContainer(t *testing.T) {
	nav := newTestNavigator(t)

	// Move to file1 (not a container)
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyDown})
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Verify we're on file1
	if nav.SelectedID() != "file1" {
		t.Fatalf("should be on file1, got %q", nav.SelectedID())
	}

	// Enter on non-container should do nothing
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should still be at root
	if nav.Current().ID() != "root" {
		t.Errorf("enter on file should not navigate, current = %q", nav.Current().ID())
	}
}

func TestModel_Update_KeyNavigation_G(t *testing.T) {
	nav := newTestNavigator(t)

	// G should go to end
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if nav.SelectedID() != "file1" {
		t.Errorf("after G, selection = %q, want file1", nav.SelectedID())
	}
}

func TestModel_Update_KeyNavigation_gg(t *testing.T) {
	nav := newTestNavigator(t)

	// Go to end first
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	// Then gg to go to start
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	if nav.SelectedID() != "folder1" {
		t.Errorf("after gg, selection = %q, want folder1", nav.SelectedID())
	}
}

func TestModel_Update_Mouse_WheelDown(t *testing.T) {
	nav := newTestNavigator(t)

	nav, _ = nav.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelDown,
	})

	if nav.SelectedID() != "folder2" {
		t.Errorf("after wheel down, selection = %q, want folder2", nav.SelectedID())
	}
}

func TestModel_Update_Mouse_WheelUp(t *testing.T) {
	nav := newTestNavigator(t)

	// Move down first
	nav, _ = nav.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	// Then wheel up
	nav, _ = nav.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})

	if nav.SelectedID() != "folder1" {
		t.Errorf("after wheel up, selection = %q, want folder1", nav.SelectedID())
	}
}

func TestModel_Update_Mouse_WheelUp_AtTop(t *testing.T) {
	nav := newTestNavigator(t)

	// Already at top, wheel up should return false (no change)
	nav, cmd := nav.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})

	if nav.SelectedID() != "folder1" {
		t.Errorf("should stay on folder1, got %q", nav.SelectedID())
	}
	if cmd != nil {
		t.Error("no navigation change, should return nil cmd")
	}
}

func TestModel_Update_Mouse_WheelDown_AtBottom(t *testing.T) {
	nav := newTestNavigator(t)

	// Go to bottom
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	// Wheel down at bottom should return nil cmd
	nav, cmd := nav.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})

	if nav.SelectedID() != "file1" {
		t.Errorf("should stay on file1, got %q", nav.SelectedID())
	}
	if cmd != nil {
		t.Error("no navigation change at bottom, should return nil cmd")
	}
}

func TestModel_Update_Mouse_MiddleClick(t *testing.T) {
	nav := newTestNavigator(t)

	// Middle click should navigate into container
	nav, _ = nav.Update(tea.MouseMsg{
		Button: tea.MouseButtonMiddle,
		Action: tea.MouseActionPress,
	})

	if nav.Current().ID() != "folder1" {
		t.Errorf("after middle click, current = %q, want folder1", nav.Current().ID())
	}
}

func TestModel_Update_Mouse_RightClick(t *testing.T) {
	nav := newTestNavigatorWithParent(t)

	// Navigate into folder first
	nav, _ = nav.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Right click should go back
	nav, _ = nav.Update(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})

	if nav.Current().ID() != "root" {
		t.Errorf("after right click, current = %q, want root", nav.Current().ID())
	}
}

func TestModel_Update_Mouse_RightClick_AtRoot(t *testing.T) {
	nav := newTestNavigator(t)

	// Right click at root should do nothing
	nav, cmd := nav.Update(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})

	if nav.Current().ID() != "root" {
		t.Errorf("should stay at root, got %q", nav.Current().ID())
	}
	if cmd != nil {
		t.Error("right click at root should return nil cmd")
	}
}

func TestModel_Update_Mouse_Release_Ignored(t *testing.T) {
	nav := newTestNavigator(t)

	// Mouse release should be ignored
	nav, cmd := nav.Update(tea.MouseMsg{
		Button: tea.MouseButtonMiddle,
		Action: tea.MouseActionRelease,
	})

	if nav.Current().ID() != "root" {
		t.Errorf("release should be ignored, current = %q", nav.Current().ID())
	}
	if cmd != nil {
		t.Error("release should return nil cmd")
	}
}

func TestModel_Refresh(t *testing.T) {
	nav := newTestNavigator(t)

	// Navigate somewhere
	nav.NavigateTo("folder1")

	// Refresh should reload current
	nav.Refresh()

	if nav.Current().ID() != "folder1" {
		t.Errorf("after Refresh, current = %q, want folder1", nav.Current().ID())
	}
	if len(nav.CurrentItems()) != 2 {
		t.Errorf("folder1 should have 2 items, got %d", len(nav.CurrentItems()))
	}
}

func TestModel_Init(t *testing.T) {
	nav := newTestNavigator(t)

	cmd := nav.Init()
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestModel_PreviewColumnWidth(t *testing.T) {
	nav := newTestNavigator(t)

	// With zero width, should return default
	nav.width = 0
	if nav.previewColumnWidth() != 40 {
		t.Errorf("with zero width, should return 40, got %d", nav.previewColumnWidth())
	}

	// With set width
	nav.width = 100
	width := nav.previewColumnWidth()
	if width <= 0 {
		t.Errorf("preview width should be positive, got %d", width)
	}
}

func TestModel_ListHeight(t *testing.T) {
	nav := newTestNavigator(t)

	nav.height = 30
	height := nav.listHeight()

	// Should be height minus overhead
	if height >= nav.height {
		t.Errorf("listHeight should be less than total height")
	}
	if height <= 0 {
		t.Errorf("listHeight should be positive, got %d", height)
	}
}

func TestModel_Left_AtRoot(t *testing.T) {
	nav := newTestNavigator(t)

	// At root, left should do nothing
	nav, cmd := nav.Update(tea.KeyMsg{Type: tea.KeyLeft})

	if nav.Current().ID() != "root" {
		t.Errorf("should stay at root, got %q", nav.Current().ID())
	}
	if cmd != nil {
		t.Error("left at root should return nil cmd")
	}
}

func TestModel_Right_OnNonContainer(t *testing.T) {
	nav := newTestNavigator(t)

	// Move to file1
	nav.FocusByName("File1.mp3")

	// Right on non-container should do nothing
	nav, cmd := nav.Update(tea.KeyMsg{Type: tea.KeyRight})

	if nav.Current().ID() != "root" {
		t.Errorf("should stay at root, got %q", nav.Current().ID())
	}
	if cmd != nil {
		t.Error("right on file should return nil cmd")
	}
}

func TestModel_MiddleClick_OnNonContainer(t *testing.T) {
	nav := newTestNavigator(t)

	// Move to file1
	nav.FocusByName("File1.mp3")

	// Middle click on non-container should do nothing
	nav, cmd := nav.Update(tea.MouseMsg{
		Button: tea.MouseButtonMiddle,
		Action: tea.MouseActionPress,
	})

	if nav.Current().ID() != "root" {
		t.Errorf("should stay at root, got %q", nav.Current().ID())
	}
	if cmd != nil {
		t.Error("middle click on file should return nil cmd")
	}
}

func TestModel_MiddleClick_EmptyList(t *testing.T) {
	source := &mockSource{
		root:     mockNode{id: "root", name: "Root", isContainer: true},
		children: map[string][]mockNode{}, // No children
	}
	nav, _ := New[mockNode](source)
	nav.SetFocused(true)
	nav.width = 80
	nav.height = 20

	// Middle click with no items should do nothing
	nav, cmd := nav.Update(tea.MouseMsg{
		Button: tea.MouseButtonMiddle,
		Action: tea.MouseActionPress,
	})

	if cmd != nil {
		t.Error("middle click on empty should return nil cmd")
	}
}

func TestModel_ParentItems(t *testing.T) {
	nav := newTestNavigatorWithParent(t)

	// At root, parent items should be nil
	if nav.parentItems != nil {
		t.Error("at root, parentItems should be nil")
	}

	// Navigate into folder1
	nav.NavigateTo("folder1")

	// Now parent items should be root's children
	if len(nav.parentItems) != 3 {
		t.Errorf("parentItems should have 3 items, got %d", len(nav.parentItems))
	}
	if nav.parentCursor != 0 {
		t.Errorf("parentCursor should be 0 (folder1 is first), got %d", nav.parentCursor)
	}
}
