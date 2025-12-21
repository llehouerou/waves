package search

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/action"
)

// testItem implements Item for testing.
type testItem struct {
	filter  string
	display string
}

func (t testItem) FilterValue() string { return t.filter }

func (t testItem) DisplayText() string { return t.display }

func TestNormalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "hello"},
		{"UPPERCASE", "uppercase"},
		{"MixedCase", "mixedcase"},
		{"already lowercase", "already lowercase"},
		{"", ""},
		{"123", "123"},
		{"Hello World", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalize(tt.input)
			if result != tt.expected {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateTrigrams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		excludes []string
	}{
		{
			name:     "simple word",
			input:    "cat",
			contains: []string{"  c", " ca", "cat", "at "},
			excludes: []string{"   "}, // all-whitespace excluded
		},
		{
			name:     "longer word",
			input:    "hello",
			contains: []string{"  h", " he", "hel", "ell", "llo", "lo ", "o  "},
			excludes: []string{"   "},
		},
		{
			name:     "empty string",
			input:    "",
			contains: nil,
			excludes: nil,
		},
		{
			name:     "short word",
			input:    "ab",
			contains: []string{"  a", " ab", "ab ", "b  "},
			excludes: []string{"   "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateTrigrams(tt.input)

			if tt.input == "" {
				if result != nil {
					t.Errorf("generateTrigrams(%q) = %v, want nil", tt.input, result)
				}
				return
			}

			for _, tri := range tt.contains {
				if _, ok := result[tri]; !ok {
					t.Errorf("generateTrigrams(%q) missing trigram %q", tt.input, tri)
				}
			}

			for _, tri := range tt.excludes {
				if _, ok := result[tri]; ok {
					t.Errorf("generateTrigrams(%q) should not contain %q", tt.input, tri)
				}
			}
		})
	}
}

func TestTrigramCoverage(t *testing.T) {
	tests := []struct {
		name     string
		query    map[string]struct{}
		item     map[string]struct{}
		expected float64
	}{
		{
			name:     "empty query",
			query:    map[string]struct{}{},
			item:     map[string]struct{}{"abc": {}},
			expected: 0,
		},
		{
			name:     "full match",
			query:    map[string]struct{}{"abc": {}, "bcd": {}},
			item:     map[string]struct{}{"abc": {}, "bcd": {}, "cde": {}},
			expected: 1.0,
		},
		{
			name:     "partial match",
			query:    map[string]struct{}{"abc": {}, "bcd": {}, "xyz": {}, "zzz": {}},
			item:     map[string]struct{}{"abc": {}, "bcd": {}},
			expected: 0.5,
		},
		{
			name:     "no match",
			query:    map[string]struct{}{"abc": {}, "bcd": {}},
			item:     map[string]struct{}{"xyz": {}, "zzz": {}},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trigramCoverage(tt.query, tt.item)
			if result != tt.expected {
				t.Errorf("trigramCoverage() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestRemoveDiacritics(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"", ""},
		{"123", "123"},
		{"Hello World", "Hello World"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := RemoveDiacritics(tt.input)
			if result != tt.expected {
				t.Errorf("RemoveDiacritics(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTrigramMatcher_Search_EmptyQuery(t *testing.T) {
	items := []Item{
		testItem{filter: "apple", display: "Apple"},
		testItem{filter: "banana", display: "Banana"},
		testItem{filter: "cherry", display: "Cherry"},
	}

	matcher := NewTrigramMatcher(items)
	matches := matcher.Search("")

	if len(matches) != 3 {
		t.Errorf("Search(\"\") returned %d matches, want 3", len(matches))
	}

	// All items should be returned with zero score
	for i, m := range matches {
		if m.Index != i {
			t.Errorf("matches[%d].Index = %d, want %d", i, m.Index, i)
		}
		if m.Score != 0 {
			t.Errorf("matches[%d].Score = %f, want 0", i, m.Score)
		}
	}
}

func TestTrigramMatcher_Search_SingleWord(t *testing.T) {
	items := []Item{
		testItem{filter: "apple pie", display: "Apple Pie"},
		testItem{filter: "banana bread", display: "Banana Bread"},
		testItem{filter: "apple cider", display: "Apple Cider"},
	}

	matcher := NewTrigramMatcher(items)
	matches := matcher.Search("apple")

	if len(matches) != 2 {
		t.Fatalf("Search(\"apple\") returned %d matches, want 2", len(matches))
	}

	// Both apple items should match
	indices := make(map[int]bool)
	for _, m := range matches {
		indices[m.Index] = true
	}

	if !indices[0] || !indices[2] {
		t.Error("expected indices 0 and 2 to match")
	}
}

func TestTrigramMatcher_Search_MultiWord(t *testing.T) {
	items := []Item{
		testItem{filter: "apple pie", display: "Apple Pie"},
		testItem{filter: "banana bread", display: "Banana Bread"},
		testItem{filter: "apple cider", display: "Apple Cider"},
	}

	matcher := NewTrigramMatcher(items)
	matches := matcher.Search("apple pie")

	if len(matches) != 1 {
		t.Fatalf("Search(\"apple pie\") returned %d matches, want 1", len(matches))
	}

	if matches[0].Index != 0 {
		t.Errorf("expected index 0, got %d", matches[0].Index)
	}
}

func TestTrigramMatcher_Search_CaseInsensitive(t *testing.T) {
	items := []Item{
		testItem{filter: "Apple Pie", display: "Apple Pie"},
	}

	matcher := NewTrigramMatcher(items)

	tests := []string{"apple", "APPLE", "ApPlE", "apple pie", "APPLE PIE"}

	for _, query := range tests {
		matches := matcher.Search(query)
		if len(matches) == 0 {
			t.Errorf("Search(%q) returned no matches, expected 1", query)
		}
	}
}

func TestTrigramMatcher_Search_ShortQuery(t *testing.T) {
	items := []Item{
		testItem{filter: "apple", display: "Apple"},
		testItem{filter: "apricot", display: "Apricot"},
		testItem{filter: "banana", display: "Banana"},
	}

	matcher := NewTrigramMatcher(items)

	// Short queries (1-2 chars) use substring match
	matches := matcher.Search("ap")
	if len(matches) != 2 {
		t.Errorf("Search(\"ap\") returned %d matches, want 2", len(matches))
	}

	matches = matcher.Search("a")
	if len(matches) != 3 {
		t.Errorf("Search(\"a\") returned %d matches, want 3 (all contain 'a')", len(matches))
	}
}

func TestTrigramMatcher_Search_NoMatch(t *testing.T) {
	items := []Item{
		testItem{filter: "apple", display: "Apple"},
		testItem{filter: "banana", display: "Banana"},
	}

	matcher := NewTrigramMatcher(items)
	matches := matcher.Search("xyz")

	if len(matches) != 0 {
		t.Errorf("Search(\"xyz\") returned %d matches, want 0", len(matches))
	}
}

func TestTrigramMatcher_Search_SortedByScore(t *testing.T) {
	items := []Item{
		testItem{filter: "something else", display: "Something Else"},
		testItem{filter: "test", display: "Test"},                // Exact match
		testItem{filter: "testing longer", display: "Testing"},   // Partial match
		testItem{filter: "unrelated word", display: "Unrelated"}, // No match
		testItem{filter: "a test here", display: "A Test Here"},  // Contains test
	}

	matcher := NewTrigramMatcher(items)
	matches := matcher.Search("test")

	// Should match items containing "test", sorted by score
	if len(matches) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(matches))
	}

	// Verify scores are in descending order
	for i := 1; i < len(matches); i++ {
		if matches[i].Score > matches[i-1].Score {
			t.Errorf("matches not sorted by score: [%d].Score=%f > [%d].Score=%f",
				i, matches[i].Score, i-1, matches[i-1].Score)
		}
	}
}

func TestModel_New(t *testing.T) {
	m := New()

	if m.query != "" {
		t.Errorf("new model query = %q, want empty", m.query)
	}
	if m.cursor != 0 {
		t.Errorf("new model cursor = %d, want 0", m.cursor)
	}
	if m.items != nil {
		t.Error("new model items should be nil")
	}
}

func TestModel_SetItems(t *testing.T) {
	m := New()
	items := []Item{
		testItem{filter: "item1", display: "Item 1"},
		testItem{filter: "item2", display: "Item 2"},
	}

	m.SetItems(items)

	if len(m.items) != 2 {
		t.Errorf("items count = %d, want 2", len(m.items))
	}
	if m.matcher == nil {
		t.Error("matcher should be set")
	}
	if m.searchFunc != nil {
		t.Error("searchFunc should be nil after SetItems")
	}
}

func TestModel_SetSearchFunc(t *testing.T) {
	m := New()
	called := false
	fn := func(_ string) ([]Item, error) {
		called = true
		return []Item{testItem{filter: "result", display: "Result"}}, nil
	}

	m.SetSearchFunc(fn)

	if m.searchFunc == nil {
		t.Error("searchFunc should be set")
	}
	if m.matcher != nil {
		t.Error("matcher should be nil after SetSearchFunc")
	}
	if !called {
		t.Error("searchFunc should be called during SetSearchFunc")
	}
}

func TestModel_Reset(t *testing.T) {
	m := New()
	m.SetItems([]Item{testItem{filter: "test", display: "Test"}})
	m.query = "test"
	m.cursor = 5
	m.loading = true

	m.Reset()

	if m.query != "" {
		t.Errorf("after reset, query = %q, want empty", m.query)
	}
	if m.cursor != 0 {
		t.Errorf("after reset, cursor = %d, want 0", m.cursor)
	}
	if m.items != nil {
		t.Error("after reset, items should be nil")
	}
	if m.matcher != nil {
		t.Error("after reset, matcher should be nil")
	}
	if m.loading {
		t.Error("after reset, loading should be false")
	}
}

func TestModel_Update_Escape(t *testing.T) {
	m := New()
	m.SetItems([]Item{testItem{filter: "test", display: "Test"}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("escape should return a command")
	}

	msg := cmd()
	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}

	result, ok := actionMsg.Action.(Result)
	if !ok {
		t.Fatalf("expected Result, got %T", actionMsg.Action)
	}

	if !result.Canceled {
		t.Error("escape should set Canceled=true")
	}
}

func TestModel_Update_Enter(t *testing.T) {
	m := New()
	items := []Item{
		testItem{filter: "first", display: "First"},
		testItem{filter: "second", display: "Second"},
	}
	m.SetItems(items)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("enter should return a command")
	}

	msg := cmd()
	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}

	result, ok := actionMsg.Action.(Result)
	if !ok {
		t.Fatalf("expected Result, got %T", actionMsg.Action)
	}

	if result.Canceled {
		t.Error("enter should not set Canceled")
	}

	if result.Item == nil {
		t.Fatal("enter should return selected item")
	}

	if result.Item.FilterValue() != "first" {
		t.Errorf("expected first item, got %q", result.Item.FilterValue())
	}
}

func TestModel_Update_Navigation(t *testing.T) {
	m := New()
	items := []Item{
		testItem{filter: "first", display: "First"},
		testItem{filter: "second", display: "Second"},
		testItem{filter: "third", display: "Third"},
	}
	m.SetItems(items)

	// Initial cursor at 0
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}

	// Move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("after down, cursor = %d, want 1", m.cursor)
	}

	// Move down again
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("after second down, cursor = %d, want 2", m.cursor)
	}

	// Move down at end (should stay at 2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("after down at end, cursor = %d, want 2", m.cursor)
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("after up, cursor = %d, want 1", m.cursor)
	}

	// Move up to top
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("after second up, cursor = %d, want 0", m.cursor)
	}

	// Move up at top (should stay at 0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("after up at top, cursor = %d, want 0", m.cursor)
	}
}

func TestModel_Update_Typing(t *testing.T) {
	m := New()
	items := []Item{
		testItem{filter: "apple", display: "Apple"},
		testItem{filter: "banana", display: "Banana"},
	}
	m.SetItems(items)

	// Type 'a'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.query != "a" {
		t.Errorf("after typing 'a', query = %q, want \"a\"", m.query)
	}

	// Type 'p'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.query != "ap" {
		t.Errorf("after typing 'p', query = %q, want \"ap\"", m.query)
	}

	// Should filter to only apple
	if len(m.matches) != 1 {
		t.Errorf("expected 1 match for 'ap', got %d", len(m.matches))
	}
}

func TestModel_Update_Backspace(t *testing.T) {
	m := New()
	items := []Item{
		testItem{filter: "apple", display: "Apple"},
		testItem{filter: "banana", display: "Banana"},
	}
	m.SetItems(items)

	// Type "app"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if m.query != "app" {
		t.Errorf("query = %q, want \"app\"", m.query)
	}

	// Backspace
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.query != "ap" {
		t.Errorf("after backspace, query = %q, want \"ap\"", m.query)
	}

	// Backspace on empty query should be no-op
	m.query = ""
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.query != "" {
		t.Errorf("backspace on empty should be no-op, got %q", m.query)
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	m := New()

	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestModel_CursorResetOnQueryChange(t *testing.T) {
	m := New()
	items := []Item{
		testItem{filter: "apple", display: "Apple"},
		testItem{filter: "apricot", display: "Apricot"},
		testItem{filter: "avocado", display: "Avocado"},
	}
	m.SetItems(items)

	// Move cursor down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}

	// Type something - cursor should reset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.cursor != 0 {
		t.Errorf("after typing, cursor = %d, want 0", m.cursor)
	}
}

func TestModel_CursorBoundsOnFilterChange(t *testing.T) {
	m := New()
	items := []Item{
		testItem{filter: "apple", display: "Apple"},
		testItem{filter: "banana", display: "Banana"},
		testItem{filter: "cherry", display: "Cherry"},
	}
	m.SetItems(items)

	// Type something that filters to fewer items
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	// Should only have banana
	if len(m.matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(m.matches))
	}

	// Cursor should be within bounds
	if m.cursor >= len(m.matches) {
		t.Errorf("cursor %d out of bounds for %d matches", m.cursor, len(m.matches))
	}
}

func TestModel_Init(t *testing.T) {
	m := New()
	cmd := m.Init()

	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestModel_SetLoading(t *testing.T) {
	m := New()

	if m.loading {
		t.Error("initial loading should be false")
	}

	m.SetLoading(true)
	if !m.loading {
		t.Error("after SetLoading(true), loading should be true")
	}

	m.SetLoading(false)
	if m.loading {
		t.Error("after SetLoading(false), loading should be false")
	}
}

func TestModel_SearchFuncError(t *testing.T) {
	m := New()
	fn := func(_ string) ([]Item, error) {
		return nil, nil // Return error via nil items
	}

	m.SetSearchFunc(fn)

	// With nil items returned, matches should be empty
	if len(m.matches) != 0 {
		t.Errorf("expected 0 matches for nil items, got %d", len(m.matches))
	}
}

func TestNewTrigramMatcher_Empty(t *testing.T) {
	matcher := NewTrigramMatcher(nil)

	matches := matcher.Search("")
	if len(matches) != 0 {
		t.Errorf("search on empty matcher should return 0 matches, got %d", len(matches))
	}

	matches = matcher.Search("test")
	if len(matches) != 0 {
		t.Errorf("search on empty matcher should return 0 matches, got %d", len(matches))
	}
}

func TestResult_ActionType(t *testing.T) {
	r := Result{}
	if r.ActionType() != "search.result" {
		t.Errorf("ActionType() = %q, want \"search.result\"", r.ActionType())
	}
}
