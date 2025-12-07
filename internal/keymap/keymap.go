// Package keymap defines key bindings for the application.
package keymap

// Binding describes a single key binding for documentation.
type Binding struct {
	Keys        []string
	Description string
	Context     string // "global", "navigator", "queue", "playback", "playlist", "playlist-track"
}

// All contains all key bindings for help generation.
var All = []Binding{
	// Global
	{[]string{"q", "ctrl+c"}, "Quit application", "global"},
	{[]string{"tab"}, "Switch focus", "global"},
	{[]string{"F1"}, "Library view", "global"},
	{[]string{"F2"}, "File browser view", "global"},
	{[]string{"F3"}, "Playlists view", "global"},
	{[]string{"p"}, "Toggle queue panel", "global"},
	{[]string{"/"}, "Search", "global"},
	{[]string{"g f"}, "Deep search", "global"},
	{[]string{"g r"}, "Refresh library", "global"},
	{[]string{"g R"}, "Full rescan library", "global"},
	{[]string{"g p"}, "Library sources", "global"},
	{[]string{"?"}, "Show help", "global"},

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
	{[]string{"ctrl+a"}, "Add to playlist", "navigator"},
	{[]string{"d"}, "Delete track", "library"},

	// Queue panel
	{[]string{"x"}, "Toggle selection", "queue"},
	{[]string{"d", "delete"}, "Delete selected", "queue"},
	{[]string{"c"}, "Clear except playing", "queue"},
	{[]string{"shift+j"}, "Move down", "queue"},
	{[]string{"shift+k"}, "Move up", "queue"},
	{[]string{"enter"}, "Play track", "queue"},
	{[]string{"esc"}, "Clear selection", "queue"},
	{[]string{"g"}, "First item", "queue"},
	{[]string{"G"}, "Last item", "queue"},

	// Playlist management
	{[]string{"n"}, "New playlist", "playlist"},
	{[]string{"N"}, "New folder", "playlist"},
	{[]string{"ctrl+r"}, "Rename", "playlist"},
	{[]string{"ctrl+d"}, "Delete", "playlist"},

	// Playlist track editing
	{[]string{"d"}, "Remove track", "playlist-track"},
	{[]string{"J"}, "Move track down", "playlist-track"},
	{[]string{"K"}, "Move track up", "playlist-track"},
}

// ByContext returns key bindings filtered by context.
func ByContext(context string) []Binding {
	var result []Binding
	for _, kb := range All {
		if kb.Context == context {
			result = append(result, kb)
		}
	}
	return result
}
