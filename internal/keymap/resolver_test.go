//nolint:goconst // test cases intentionally repeat strings for readability
package keymap

import (
	"slices"
	"testing"
)

func TestNewResolver(t *testing.T) {
	bindings := []Binding{
		{ActionQuit, []string{"q", "ctrl+c"}, "Quit", "global"},
		{ActionPlayPause, []string{" "}, "Play/pause", "playback"},
		{ActionMoveUp, []string{"k", "up"}, "Move up", "navigator"},
	}

	r := NewResolver(bindings)

	if r == nil {
		t.Fatal("NewResolver returned nil")
	}

	// Verify bindings map is populated
	if r.bindings == nil {
		t.Error("bindings map is nil")
	}

	// Verify byAction map is populated
	if r.byAction == nil {
		t.Error("byAction map is nil")
	}
}

func TestResolver_Resolve(t *testing.T) {
	bindings := []Binding{
		{ActionQuit, []string{"q", "ctrl+c"}, "Quit", "global"},
		{ActionPlayPause, []string{" "}, "Play/pause", "playback"},
		{ActionMoveUp, []string{"k", "up"}, "Move up", "navigator"},
		{ActionMoveDown, []string{"j", "down"}, "Move down", "navigator"},
	}

	r := NewResolver(bindings)

	tests := []struct {
		key      string
		expected Action
	}{
		{"q", ActionQuit},
		{"ctrl+c", ActionQuit},
		{" ", ActionPlayPause},
		{"k", ActionMoveUp},
		{"up", ActionMoveUp},
		{"j", ActionMoveDown},
		{"down", ActionMoveDown},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := r.Resolve(tt.key)
			if result != tt.expected {
				t.Errorf("Resolve(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestResolver_KeysFor(t *testing.T) {
	bindings := []Binding{
		{ActionQuit, []string{"q", "ctrl+c"}, "Quit", "global"},
		{ActionPlayPause, []string{" "}, "Play/pause", "playback"},
		{ActionMoveUp, []string{"k", "up"}, "Move up", "navigator"},
	}

	r := NewResolver(bindings)

	tests := []struct {
		action   Action
		expected []string
	}{
		{ActionQuit, []string{"q", "ctrl+c"}},
		{ActionPlayPause, []string{" "}},
		{ActionMoveUp, []string{"k", "up"}},
		{Action("unknown"), nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			result := r.KeysFor(tt.action)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("KeysFor(%q) = %v, want nil", tt.action, result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("KeysFor(%q) = %v, want %v", tt.action, result, tt.expected)
				return
			}

			for _, key := range tt.expected {
				if !slices.Contains(result, key) {
					t.Errorf("KeysFor(%q) missing key %q, got %v", tt.action, key, result)
				}
			}
		})
	}
}

func TestResolver_DeduplicatesKeys(t *testing.T) {
	// Same action defined in multiple contexts with overlapping keys
	bindings := []Binding{
		{ActionDelete, []string{"d", "delete"}, "Delete", "queue"},
		{ActionDelete, []string{"d"}, "Delete", "library"},
		{ActionDelete, []string{"d"}, "Delete", "filebrowser"},
	}

	r := NewResolver(bindings)

	keys := r.KeysFor(ActionDelete)

	// Count occurrences of "d"
	count := 0
	for _, k := range keys {
		if k == "d" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("expected 'd' to appear once after deduplication, got %d times in %v", count, keys)
	}
}

func TestResolver_WithGlobalBindings(t *testing.T) {
	// Test with the actual global Bindings
	r := NewResolver(Bindings)

	// Verify some known bindings work
	if action := r.Resolve("q"); action != ActionQuit {
		t.Errorf("Resolve('q') = %q, want %q", action, ActionQuit)
	}

	if action := r.Resolve("tab"); action != ActionSwitchFocus {
		t.Errorf("Resolve('tab') = %q, want %q", action, ActionSwitchFocus)
	}

	if action := r.Resolve(" "); action != ActionPlayPause {
		t.Errorf("Resolve(' ') = %q, want %q", action, ActionPlayPause)
	}

	// Verify KeysFor returns expected keys
	quitKeys := r.KeysFor(ActionQuit)
	if !slices.Contains(quitKeys, "q") || !slices.Contains(quitKeys, "ctrl+c") {
		t.Errorf("KeysFor(ActionQuit) = %v, expected to contain 'q' and 'ctrl+c'", quitKeys)
	}
}

func TestDedupe(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "all duplicates",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"x"},
			expected: []string{"x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupe(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("dedupe(%v) = %v, want %v", tt.input, result, tt.expected)
				return
			}

			// Check that all expected elements are present and in order
			for i, v := range tt.expected {
				if result[i] != v {
					t.Errorf("dedupe(%v)[%d] = %q, want %q", tt.input, i, result[i], v)
				}
			}
		})
	}
}

func TestResolver_EmptyBindings(t *testing.T) {
	r := NewResolver([]Binding{})

	if action := r.Resolve("q"); action != "" {
		t.Errorf("Resolve on empty resolver should return empty, got %q", action)
	}

	if keys := r.KeysFor(ActionQuit); keys != nil {
		t.Errorf("KeysFor on empty resolver should return nil, got %v", keys)
	}
}
