//nolint:goconst // test cases intentionally repeat strings for readability
package keymap

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func TestByContext(t *testing.T) {
	tests := []struct {
		name            string
		context         string
		expectNonEmpty  bool
		expectMinLength int
	}{
		{"global context", "global", true, 5},
		{"playback context", "playback", true, 5},
		{"navigator context", "navigator", true, 5},
		{"queue context", "queue", true, 5},
		{"playlist context", "playlist", true, 1},
		{"playlist-track context", "playlist-track", true, 1},
		{"library context", "library", true, 1},
		{"filebrowser context", "filebrowser", true, 1},
		{"albumview context", "albumview", true, 1},
		{"unknown context returns empty", "unknown", false, 0},
		{"empty context returns empty", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ByContext(tt.context)

			if tt.expectNonEmpty && len(result) == 0 {
				t.Errorf("ByContext(%q) returned empty, expected non-empty", tt.context)
			}

			if !tt.expectNonEmpty && len(result) != 0 {
				t.Errorf("ByContext(%q) returned %d items, expected empty", tt.context, len(result))
			}

			if len(result) < tt.expectMinLength {
				t.Errorf("ByContext(%q) returned %d items, expected at least %d", tt.context, len(result), tt.expectMinLength)
			}

			// Verify all returned bindings have the correct context
			for _, binding := range result {
				if binding.Context != tt.context {
					t.Errorf("binding context = %q, want %q", binding.Context, tt.context)
				}
			}
		})
	}
}

func TestByContextGlobalBindings(t *testing.T) {
	globalBindings := ByContext("global")

	// Check that essential global bindings exist
	expectedActions := []Action{
		ActionQuit,
		ActionSwitchFocus,
		ActionToggleQueue,
		ActionSearch,
		ActionHelp,
	}

	for _, action := range expectedActions {
		found := false
		for _, b := range globalBindings {
			if b.Action == action {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected action %q in global bindings", action)
		}
	}
}

func TestByContextPlaybackBindings(t *testing.T) {
	playbackBindings := ByContext("playback")

	// Check that essential playback bindings exist
	expectedActions := []Action{
		ActionPlayPause,
		ActionStop,
		ActionNextTrack,
		ActionPrevTrack,
	}

	for _, action := range expectedActions {
		found := false
		for _, b := range playbackBindings {
			if b.Action == action {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected action %q in playback bindings", action)
		}
	}
}

func TestBindingsHaveRequiredFields(t *testing.T) {
	for i, b := range Bindings {
		if b.Action == "" {
			t.Errorf("binding[%d] has empty Action", i)
		}
		if len(b.Keys) == 0 {
			t.Errorf("binding[%d] (%s) has no Keys", i, b.Action)
		}
		if b.Description == "" {
			t.Errorf("binding[%d] (%s) has empty Description", i, b.Action)
		}
		if b.Context == "" {
			t.Errorf("binding[%d] (%s) has empty Context", i, b.Action)
		}
	}
}

// The downloads view resolves keys against its own context; the global
// resolver must keep the library meaning of keys shared with it.
func TestGlobalResolverKeepsLibraryKeys(t *testing.T) {
	r := NewResolver(Bindings)
	if got := r.Resolve("i"); got != ActionSimilarArtists {
		t.Errorf("Resolve(i) = %q, want %q (downloads bindings must precede library ones)", got, ActionSimilarArtists)
	}
	if got := NewResolver(ByContext(ContextDownloads)).Resolve("i"); got != ActionImportDownload {
		t.Errorf("downloads Resolve(i) = %q, want %q", got, ActionImportDownload)
	}
}

func TestBindingsHaveValidContexts(t *testing.T) {
	validContexts := map[string]bool{
		"global":         true,
		"navigator":      true,
		"queue":          true,
		"playback":       true,
		"playlist":       true,
		"playlist-track": true,
		"library":        true,
		"filebrowser":    true,
		"albumview":      true,
		"downloads":      true,
	}

	for i, b := range Bindings {
		if !validContexts[b.Context] {
			t.Errorf("binding[%d] (%s) has invalid context: %q", i, b.Action, b.Context)
		}
	}
}

// producibleKeyNames returns every key name Bubble Tea v1 gives a non-rune
// key: the String() of a tea.KeyMsg for each KeyType, with or without Alt.
func producibleKeyNames() map[string]bool {
	names := make(map[string]bool)
	// Bubble Tea v1 key types run from about -90 to 127 (keyDEL): the sweep
	// over-approximates on purpose, unknown types have no name and are skipped.
	for kt := tea.KeyType(-1000); kt <= 1000; kt++ {
		if kt == tea.KeyRunes {
			continue
		}
		for _, alt := range []bool{false, true} {
			if s := (tea.KeyMsg{Type: kt, Alt: alt}).String(); s != "" {
				names[s] = true
			}
		}
	}
	return names
}

// A binding Bubble Tea can never produce is a key the user cannot press.
func TestBindingsKeysAreProducibleByBubbleTea(t *testing.T) {
	names := producibleKeyNames()
	for _, b := range Bindings {
		for _, key := range b.Keys {
			parts := []string{key}
			if utf8.RuneCountInString(key) > 1 {
				parts = strings.Split(key, " ")
			}
			for _, part := range parts {
				if utf8.RuneCountInString(part) != 1 && !names[part] {
					t.Errorf("%s: key %q has part %q, which Bubble Tea never produces", b.Action, key, part)
				}
			}
		}
	}
}

// D keeps only the selection in the queue and clears the completed downloads
// in the downloads view: each view resolves it against its own context.
func TestQueueResolverKeepsSelectedOnD(t *testing.T) {
	if got := NewResolver(ByContext(ContextQueue)).Resolve("D"); got != ActionKeepSelected {
		t.Errorf("queue Resolve(D) = %q, want %q", got, ActionKeepSelected)
	}
	if got := NewResolver(ByContext(ContextDownloads)).Resolve("D"); got != ActionClearCompleted {
		t.Errorf("downloads Resolve(D) = %q, want %q", got, ActionClearCompleted)
	}
}
