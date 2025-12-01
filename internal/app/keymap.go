// internal/app/keymap.go
package app

// KeyBinding describes a single key binding for documentation.
type KeyBinding struct {
	Keys        []string
	Description string
	Context     string // "global", "navigator", "queue", "playback"
}

// KeyMap contains all key bindings for help generation.
var KeyMap = []KeyBinding{
	// Global
	{[]string{"q", "ctrl+c"}, "Quit application", "global"},
	{[]string{"tab"}, "Switch focus", "global"},
	{[]string{"F1"}, "Library view", "global"},
	{[]string{"F2"}, "File browser view", "global"},
	{[]string{"p"}, "Toggle queue panel", "global"},
	{[]string{"/"}, "Search", "global"},
	{[]string{"space ff"}, "Deep search (files)", "global"},
	{[]string{"space lr"}, "Refresh library", "global"},

	// Playback
	{[]string{"space"}, "Play/pause", "playback"},
	{[]string{"s"}, "Stop", "playback"},
	{[]string{"pgdown"}, "Next track", "playback"},
	{[]string{"pgup"}, "Previous track", "playback"},
	{[]string{"home"}, "First track", "playback"},
	{[]string{"end"}, "Last track", "playback"},
	{[]string{"shift+left"}, "Seek -5s", "playback"},
	{[]string{"shift+right"}, "Seek +5s", "playback"},
	{[]string{"v"}, "Toggle player display", "playback"},
	{[]string{"R"}, "Cycle repeat mode", "playback"},
	{[]string{"S"}, "Toggle shuffle", "playback"},

	// Navigator
	{[]string{"h", "left"}, "Parent/collapse", "navigator"},
	{[]string{"l", "right"}, "Enter/expand", "navigator"},
	{[]string{"j", "down"}, "Move down", "navigator"},
	{[]string{"k", "up"}, "Move up", "navigator"},
	{[]string{"enter"}, "Add and play", "navigator"},
	{[]string{"a"}, "Add to queue", "navigator"},
	{[]string{"r"}, "Replace queue", "navigator"},
	{[]string{"alt+enter"}, "Add album, play track", "navigator"},

	// Queue
	{[]string{"x"}, "Toggle selection", "queue"},
	{[]string{"d", "delete"}, "Delete selected", "queue"},
	{[]string{"shift+j"}, "Move down", "queue"},
	{[]string{"shift+k"}, "Move up", "queue"},
	{[]string{"enter"}, "Play track", "queue"},
	{[]string{"esc"}, "Clear selection", "queue"},
	{[]string{"g"}, "First track", "queue"},
	{[]string{"G"}, "Last track", "queue"},
}

// KeysByContext returns key bindings filtered by context.
func KeysByContext(context string) []KeyBinding {
	var result []KeyBinding
	for _, kb := range KeyMap {
		if kb.Context == context {
			result = append(result, kb)
		}
	}
	return result
}
