//nolint:goconst // test cases intentionally repeat strings for readability
package keymap

import (
	"testing"
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
