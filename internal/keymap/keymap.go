package keymap

// Binding describes a single key binding linking an action to keys and documentation.
type Binding struct {
	Action      Action
	Keys        []string
	Description string
	Context     string // "global", "navigator", "queue", "playback", "playlist", "playlist-track", "library", "filebrowser", "albumview", "downloads"
}

// Bindings contains all key bindings - the single source of truth.
var Bindings = []Binding{
	// Global
	{ActionQuit, []string{"q", "ctrl+c"}, "Quit application", "global"},
	{ActionSwitchFocus, []string{"tab"}, "Switch focus", "global"},
	{ActionToggleQueue, []string{"p"}, "Toggle queue panel", "global"},
	{ActionSearch, []string{"/"}, "Search", "global"},
	{ActionHelp, []string{"?"}, "Show help", "global"},

	// View switching
	{ActionViewLibrary, []string{"f1"}, "Library view", "global"},
	{ActionViewFileBrowser, []string{"f2"}, "File browser view", "global"},
	{ActionViewPlaylists, []string{"f3"}, "Playlists view", "global"},
	{ActionViewDownloads, []string{"f4"}, "Downloads view", "global"},

	// F-sequence prefix and actions
	{ActionFPrefix, []string{"f"}, "Function prefix", "global"},
	{ActionDeepSearch, []string{"f f"}, "Deep search", "global"},
	{ActionRefreshLibrary, []string{"f r"}, "Refresh library", "global"},
	{ActionFullRescan, []string{"f R"}, "Full rescan library", "global"},
	{ActionLibrarySources, []string{"f p"}, "Library sources", "global"},
	{ActionDownloadSoulseek, []string{"f d"}, "Download from Soulseek", "global"},
	{ActionLastfmSettings, []string{"f l"}, "Last.fm settings", "global"},

	// Playback
	{ActionPlayPause, []string{" "}, "Play/pause", "playback"},
	{ActionStop, []string{"s"}, "Stop", "playback"},
	{ActionNextTrack, []string{"pgdown"}, "Next track", "playback"},
	{ActionPrevTrack, []string{"pgup"}, "Previous track", "playback"},
	{ActionFirstTrack, []string{"home"}, "First track", "playback"},
	{ActionLastTrack, []string{"end"}, "Last track", "playback"},
	{ActionSeekBack, []string{"shift+left"}, "Seek -5s", "playback"},
	{ActionSeekForward, []string{"shift+right"}, "Seek +5s", "playback"},
	{ActionSeekBackLong, []string{"alt+shift+left"}, "Seek -15s", "playback"},
	{ActionSeekForwardLong, []string{"alt+shift+right"}, "Seek +15s", "playback"},
	{ActionTogglePlayerDisplay, []string{"v"}, "Toggle player display", "playback"},
	{ActionCycleRepeat, []string{"R"}, "Cycle repeat (off/all/one/radio)", "playback"},
	{ActionToggleShuffle, []string{"S"}, "Toggle shuffle", "playback"},
	{ActionShowLyrics, []string{"f y"}, "Show lyrics", "global"},

	// Volume
	{ActionVolumeUp, []string{"+"}, "Volume +10%", "playback"},
	{ActionVolumeDown, []string{"-"}, "Volume -10%", "playback"},
	{ActionVolumeUpFine, []string{"shift++"}, "Volume +1%", "playback"},
	{ActionVolumeDownFine, []string{"shift+-"}, "Volume -1%", "playback"},
	{ActionToggleMute, []string{"M"}, "Toggle mute", "playback"},

	// Navigator
	{ActionMoveLeft, []string{"h", "left"}, "Parent/collapse", "navigator"},
	{ActionMoveRight, []string{"l", "right"}, "Enter/expand", "navigator"},
	{ActionMoveDown, []string{"j", "down"}, "Move down", "navigator"},
	{ActionMoveUp, []string{"k", "up"}, "Move up", "navigator"},
	{ActionSelect, []string{"enter"}, "Play (replace queue)", "navigator"},
	{ActionAdd, []string{"a"}, "Add to queue", "navigator"},
	{ActionAddToPlaylist, []string{"ctrl+a"}, "Add to playlist", "navigator"},
	{ActionJumpStart, []string{"g"}, "First item", "navigator"},
	{ActionJumpEnd, []string{"G"}, "Last item", "navigator"},
	{ActionPageDown, []string{"ctrl+d"}, "Half page down", "navigator"},
	{ActionPageUp, []string{"ctrl+u"}, "Half page up", "navigator"},

	// Library-specific
	{ActionDelete, []string{"d"}, "Delete track", "library"},
	{ActionToggleFavorite, []string{"F"}, "Toggle favorite", "library"},
	{ActionToggleAlbumView, []string{"V"}, "Toggle album view", "library"},
	{ActionRetag, []string{"t"}, "Retag album", "library"},
	{ActionExport, []string{"e"}, "Export to USB", "library"},

	// Album view options (o-sequence)
	{ActionOPrefix, []string{"o"}, "Options prefix", "albumview"},
	{ActionAlbumGrouping, []string{"o g"}, "Album grouping", "albumview"},
	{ActionAlbumSorting, []string{"o s"}, "Album sorting", "albumview"},
	{ActionAlbumPresets, []string{"o p"}, "Album presets", "albumview"},

	// File browser
	{ActionDelete, []string{"d"}, "Delete file/folder", "filebrowser"},

	// Queue panel
	{ActionToggleSelect, []string{"x"}, "Toggle selection", "queue"},
	{ActionDelete, []string{"d", "delete"}, "Delete selected", "queue"},
	{ActionClear, []string{"c"}, "Clear except playing", "queue"},
	{ActionMoveItemDown, []string{"shift+j"}, "Move down", "queue"},
	{ActionMoveItemUp, []string{"shift+k"}, "Move up", "queue"},
	{ActionSelect, []string{"enter"}, "Play track", "queue"},
	{ActionClearSelect, []string{"esc"}, "Clear selection", "queue"},
	{ActionToggleFavorite, []string{"F"}, "Toggle favorite", "queue"},
	{ActionAddToPlaylist, []string{"ctrl+a"}, "Add to playlist", "queue"},
	{ActionLocate, []string{"L"}, "Locate in navigator", "queue"},
	{ActionExport, []string{"e"}, "Export to USB", "queue"},
	{ActionJumpStart, []string{"g"}, "First item", "queue"},
	{ActionJumpEnd, []string{"G"}, "Last item", "queue"},
	{ActionPageDown, []string{"ctrl+d"}, "Half page down", "queue"},
	{ActionPageUp, []string{"ctrl+u"}, "Half page up", "queue"},

	// Queue history (global when queue available)
	{ActionUndo, []string{"ctrl+z"}, "Undo", "global"},
	{ActionRedo, []string{"ctrl+shift+z"}, "Redo", "global"},

	// Playlist management
	{ActionNewPlaylist, []string{"n"}, "New playlist", "playlist"},
	{ActionNewFolder, []string{"N"}, "New folder", "playlist"},
	{ActionRename, []string{"ctrl+r"}, "Rename", "playlist"},
	{ActionDelete, []string{"ctrl+d"}, "Delete", "playlist"},

	// Playlist track editing
	{ActionDelete, []string{"d"}, "Remove track", "playlist-track"},
	{ActionMoveItemDown, []string{"J"}, "Move track down", "playlist-track"},
	{ActionMoveItemUp, []string{"K"}, "Move track up", "playlist-track"},
}

// ByContext returns key bindings filtered by context.
func ByContext(context string) []Binding {
	var result []Binding
	for _, kb := range Bindings {
		if kb.Context == context {
			result = append(result, kb)
		}
	}
	return result
}
