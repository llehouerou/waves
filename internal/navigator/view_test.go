package navigator

import (
	"regexp"
	"strings"
	"testing"
)

// stripANSI removes ANSI escape codes from a string for easier testing.
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// mockNode implements the Node interface for testing.
type mockNode struct {
	id          string
	name        string
	isContainer bool
	iconType    IconType
}

func (n mockNode) ID() string { return n.id }

func (n mockNode) DisplayName() string { return n.name }

func (n mockNode) IsContainer() bool { return n.isContainer }

func (n mockNode) IconType() IconType { return n.iconType }

// mockSource implements Source[mockNode] for testing.
type mockSource struct {
	root     mockNode
	children map[string][]mockNode
}

func (s *mockSource) Root() mockNode { return s.root }

func (s *mockSource) Children(parent mockNode) ([]mockNode, error) {
	if children, ok := s.children[parent.id]; ok {
		return children, nil
	}
	return nil, nil
}

func (s *mockSource) Parent(_ mockNode) *mockNode {
	return nil // Simplified for tests
}

func (s *mockSource) DisplayPath(node mockNode) string {
	return "/" + node.name
}

func (s *mockSource) NodeFromID(id string) (mockNode, bool) {
	if id == s.root.id {
		return s.root, true
	}
	for _, children := range s.children {
		for _, child := range children {
			if child.id == id {
				return child, true
			}
		}
	}
	return mockNode{}, false
}

func newTestSource() *mockSource {
	root := mockNode{id: "root", name: "Root", isContainer: true, iconType: IconFolder}
	return &mockSource{
		root: root,
		children: map[string][]mockNode{
			"root": {
				{id: "folder1", name: "Folder1", isContainer: true, iconType: IconFolder},
				{id: "folder2", name: "Folder2", isContainer: true, iconType: IconFolder},
				{id: "file1", name: "File1.mp3", isContainer: false, iconType: IconAudio},
			},
			"folder1": {
				{id: "subfolder", name: "SubFolder", isContainer: true, iconType: IconFolder},
				{id: "subfile", name: "SubFile.mp3", isContainer: false, iconType: IconAudio},
			},
		},
	}
}

func newTestNavigator(t *testing.T) Model[mockNode] {
	t.Helper()
	source := newTestSource()
	nav, err := New[mockNode](source)
	if err != nil {
		t.Fatalf("failed to create navigator: %v", err)
	}
	nav.SetFocused(true)
	nav.width = 80
	nav.height = 20
	return nav
}

func TestView_ZeroWidth(t *testing.T) {
	source := newTestSource()
	nav, err := New[mockNode](source)
	if err != nil {
		t.Fatalf("failed to create navigator: %v", err)
	}
	// Don't set width

	output := nav.View()
	if output != "Loading..." {
		t.Errorf("zero width should show loading, got: %q", output)
	}
}

func TestView_ShowsPath(t *testing.T) {
	nav := newTestNavigator(t)

	output := nav.View()
	stripped := stripANSI(output)

	// Should show path in header
	if !strings.Contains(stripped, "/Root") {
		t.Errorf("should show path in header, got: %s", stripped)
	}
}

func TestView_ShowsItems(t *testing.T) {
	nav := newTestNavigator(t)

	output := nav.View()
	stripped := stripANSI(output)

	// Should show items from root
	if !strings.Contains(stripped, "Folder1") {
		t.Errorf("should show Folder1, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Folder2") {
		t.Errorf("should show Folder2, got: %s", stripped)
	}
	if !strings.Contains(stripped, "File1") {
		t.Errorf("should show File1, got: %s", stripped)
	}
}

func TestView_ShowsCursor(t *testing.T) {
	nav := newTestNavigator(t)

	output := nav.View()
	stripped := stripANSI(output)

	// Should show cursor indicator on first item
	if !strings.Contains(stripped, "> ") {
		t.Errorf("should show cursor indicator, got: %s", stripped)
	}
}

func TestView_ShowsSeparator(t *testing.T) {
	nav := newTestNavigator(t)

	output := nav.View()
	stripped := stripANSI(output)

	// Should show separator line
	if !strings.Contains(stripped, "─") {
		t.Errorf("should show separator, got: %s", stripped)
	}
}

func TestView_ShowsColumnDivider(t *testing.T) {
	nav := newTestNavigator(t)

	output := nav.View()
	stripped := stripANSI(output)

	// Should show column divider between columns
	if !strings.Contains(stripped, "│") {
		t.Errorf("should show column divider, got: %s", stripped)
	}
}

func TestView_ShowsPreview(t *testing.T) {
	nav := newTestNavigator(t)
	// Cursor is on Folder1 which has children

	output := nav.View()
	stripped := stripANSI(output)

	// Should show preview of Folder1's contents
	if !strings.Contains(stripped, "SubFolder") {
		t.Errorf("should show SubFolder in preview, got: %s", stripped)
	}
	if !strings.Contains(stripped, "SubFile") {
		t.Errorf("should show SubFile in preview, got: %s", stripped)
	}
}

func TestView_UnfocusedNoCursor(t *testing.T) {
	nav := newTestNavigator(t)
	nav.SetFocused(false)

	output := nav.View()
	stripped := stripANSI(output)

	// Items should still be visible
	if !strings.Contains(stripped, "Folder1") {
		t.Errorf("should still show items when unfocused, got: %s", stripped)
	}
}

func TestRenderColumn_EmptyItems(t *testing.T) {
	nav := newTestNavigator(t)

	lines := nav.renderColumn(nil, -1, 0, 30, 5)

	if len(lines) != 5 {
		t.Errorf("should have 5 lines, got: %d", len(lines))
	}

	// All lines should be spaces (empty)
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			t.Errorf("line %d should be empty, got: %q", i, line)
		}
	}
}

func TestRenderColumn_WithItems(t *testing.T) {
	nav := newTestNavigator(t)
	items := []mockNode{
		{id: "1", name: "Item1", isContainer: false, iconType: IconAudio},
		{id: "2", name: "Item2", isContainer: false, iconType: IconAudio},
	}

	lines := nav.renderColumn(items, 0, 0, 30, 5)

	if len(lines) != 5 {
		t.Errorf("should have 5 lines, got: %d", len(lines))
	}

	// First line should have cursor
	if !strings.Contains(lines[0], "> ") {
		t.Errorf("first line should have cursor, got: %q", lines[0])
	}
	if !strings.Contains(lines[0], "Item1") {
		t.Errorf("first line should have Item1, got: %q", lines[0])
	}

	// Second line should have item without cursor
	if strings.Contains(lines[1], "> ") {
		t.Errorf("second line should not have cursor, got: %q", lines[1])
	}
	if !strings.Contains(lines[1], "Item2") {
		t.Errorf("second line should have Item2, got: %q", lines[1])
	}
}

func TestRenderColumn_NoCursor(t *testing.T) {
	nav := newTestNavigator(t)
	items := []mockNode{
		{id: "1", name: "Item1", isContainer: false, iconType: IconAudio},
	}

	// cursor = -1 means no cursor
	lines := nav.renderColumn(items, -1, 0, 30, 5)

	// No line should have cursor
	for i, line := range lines {
		if strings.Contains(line, "> ") {
			t.Errorf("line %d should not have cursor with cursor=-1, got: %q", i, line)
		}
	}
}

func TestRenderColumn_WithOffset(t *testing.T) {
	nav := newTestNavigator(t)
	items := []mockNode{
		{id: "1", name: "Item1", isContainer: false, iconType: IconAudio},
		{id: "2", name: "Item2", isContainer: false, iconType: IconAudio},
		{id: "3", name: "Item3", isContainer: false, iconType: IconAudio},
	}

	// offset=1 should skip Item1
	lines := nav.renderColumn(items, 1, 1, 30, 5)

	// First visible line should be Item2 with cursor
	if !strings.Contains(lines[0], "Item2") {
		t.Errorf("first line should have Item2 (offset=1), got: %q", lines[0])
	}
	if !strings.Contains(lines[0], "> ") {
		t.Errorf("first line should have cursor on Item2, got: %q", lines[0])
	}
}

func TestJoinThreeColumns_Basic(t *testing.T) {
	nav := newTestNavigator(t)

	col1 := []string{"A", "B", "C"}
	col2 := []string{"1", "2", "3"}
	col3 := []string{"X", "Y", "Z"}

	result := nav.joinThreeColumns(col1, col2, col3)
	lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")

	if len(lines) != 3 {
		t.Errorf("should have 3 lines, got: %d", len(lines))
	}

	// Each line should have 2 dividers (for 3 columns)
	for i, line := range lines {
		if strings.Count(line, "│") != 2 {
			t.Errorf("line %d should have 2 dividers, got: %q", i, line)
		}
	}
}

func TestJoinThreeColumns_UnequalLengths(t *testing.T) {
	nav := newTestNavigator(t)

	col1 := []string{"A", "B"}
	col2 := []string{"1", "2", "3", "4"}
	col3 := []string{"X"}

	result := nav.joinThreeColumns(col1, col2, col3)
	lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")

	// Should have max(2, 4, 1) = 4 lines
	if len(lines) != 4 {
		t.Errorf("should have 4 lines (max of all), got: %d", len(lines))
	}
}
