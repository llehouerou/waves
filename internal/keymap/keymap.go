package keymap

// Binding describes a single key binding linking an action to keys and documentation.
type Binding struct {
	Action      Action
	Keys        []string
	Description string
	Context     string // one of the Context* constants below
}

// Binding contexts.
const (
	ContextGlobal        = "global"
	ContextNavigator     = "navigator"
	ContextQueue         = "queue"
	ContextPlayback      = "playback"
	ContextPlaylist      = "playlist"
	ContextPlaylistTrack = "playlist-track"
	ContextLibrary       = "library"
	ContextFileBrowser   = "filebrowser"
	ContextAlbumView     = "albumview"
	ContextDownloads     = "downloads"
)

// Keys and descriptions shared by several bindings; goconst wants them named.
const (
	keyCtrlD = "ctrl+d"
	keyCtrlU = "ctrl+u"
	keyDown  = "down"
	keyEnter = "enter"

	descMoveDown     = "Move down"
	descMoveUp       = "Move up"
	descFirstItem    = "First item"
	descLastItem     = "Last item"
	descHalfPageDown = "Half page down"
	descHalfPageUp   = "Half page up"
)

// Bindings contains all key bindings - the single source of truth.
var Bindings = []Binding{
	// Global
	{ActionQuit, []string{"q", "ctrl+c"}, "Quit application", ContextGlobal},
	{ActionSwitchFocus, []string{"tab"}, "Switch focus", ContextGlobal},
	{ActionToggleQueue, []string{"p"}, "Toggle queue panel", ContextGlobal},
	{ActionHelp, []string{"?"}, "Show help", ContextGlobal},

	// View switching
	{ActionViewLibrary, []string{"f1"}, "Library view", ContextGlobal},
	{ActionViewFileBrowser, []string{"f2"}, "File browser view", ContextGlobal},
	{ActionViewPlaylists, []string{"f3"}, "Playlists view", ContextGlobal},
	{ActionViewDownloads, []string{"f4"}, "Downloads view", ContextGlobal},

	// F-sequence prefix and actions
	{ActionFPrefix, []string{"f"}, "Function prefix", ContextGlobal},
	{ActionDeepSearch, []string{"f f"}, "Deep search", ContextGlobal},
	{ActionRefreshLibrary, []string{"f r"}, "Refresh library", ContextGlobal},
	{ActionFullRescan, []string{"f R"}, "Full rescan library", ContextGlobal},
	{ActionLibrarySources, []string{"f p"}, "Library sources", ContextGlobal},
	{ActionDownloadSoulseek, []string{"f d"}, "Download from Soulseek", ContextGlobal},
	{ActionLastfmSettings, []string{"f l"}, "Last.fm settings", ContextGlobal},
	{ActionNewReleases, []string{"f n"}, "New releases", ContextGlobal},

	// Playback
	{ActionPlayPause, []string{" "}, "Play/pause", ContextPlayback},
	{ActionStop, []string{"s"}, "Stop", ContextPlayback},
	{ActionNextTrack, []string{"pgdown"}, "Next track", ContextPlayback},
	{ActionPrevTrack, []string{"pgup"}, "Previous track", ContextPlayback},
	{ActionFirstTrack, []string{"home"}, "First track", ContextPlayback},
	{ActionLastTrack, []string{"end"}, "Last track", ContextPlayback},
	{ActionSeekBack, []string{"shift+left"}, "Seek -5s", ContextPlayback},
	{ActionSeekForward, []string{"shift+right"}, "Seek +5s", ContextPlayback},
	{ActionSeekBackLong, []string{"alt+shift+left"}, "Seek -15s", ContextPlayback},
	{ActionSeekForwardLong, []string{"alt+shift+right"}, "Seek +15s", ContextPlayback},
	{ActionTogglePlayerDisplay, []string{"v"}, "Toggle player display", ContextPlayback},
	{ActionCycleRepeat, []string{"R"}, "Cycle repeat (off/all/one/radio)", ContextPlayback},
	{ActionToggleShuffle, []string{"S"}, "Toggle shuffle", ContextPlayback},
	{ActionShowLyrics, []string{"f y"}, "Show lyrics", ContextGlobal},

	// Volume
	{ActionVolumeUp, []string{"+"}, "Volume +10%", ContextPlayback},
	{ActionVolumeDown, []string{"-"}, "Volume -10%", ContextPlayback},
	{ActionToggleMute, []string{"M"}, "Toggle mute", ContextPlayback},

	// Navigator
	{ActionMoveLeft, []string{"h", "left"}, "Parent/collapse", ContextNavigator},
	{ActionMoveRight, []string{"l", "right"}, "Enter/expand", ContextNavigator},
	{ActionMoveDown, []string{"j", keyDown}, descMoveDown, ContextNavigator},
	{ActionMoveUp, []string{"k", "up"}, descMoveUp, ContextNavigator},
	{ActionSelect, []string{keyEnter}, "Play (replace queue)", ContextNavigator},
	{ActionAdd, []string{"a"}, "Add to queue", ContextNavigator},
	{ActionJumpStart, []string{"g"}, descFirstItem, ContextNavigator},
	{ActionJumpEnd, []string{"G"}, descLastItem, ContextNavigator},
	{ActionPageDown, []string{keyCtrlD}, descHalfPageDown, ContextNavigator},
	{ActionPageUp, []string{keyCtrlU}, descHalfPageUp, ContextNavigator},
	{ActionSearch, []string{"/"}, "Search", ContextNavigator},

	// Downloads view. Resolved by the view against this context only; listed
	// before Library so the global resolver keeps "i" -> similar artists.
	// The view is no navigator: its cursor keys are its own.
	{ActionSelect, []string{keyEnter}, "Expand/collapse files", ContextDownloads},
	{ActionDelete, []string{"d"}, "Delete download", ContextDownloads},
	{ActionImportDownload, []string{"i"}, "Import download", ContextDownloads},
	{ActionRetryDownload, []string{"ctrl+r"}, "Retry failed files", ContextDownloads},
	{ActionClearCompleted, []string{"D"}, "Clear completed", ContextDownloads},
	{ActionRefreshDownloads, []string{"r"}, "Refresh from slskd", ContextDownloads},
	{ActionMoveDown, []string{"j", keyDown}, descMoveDown, ContextDownloads},
	{ActionMoveUp, []string{"k", "up"}, descMoveUp, ContextDownloads},
	{ActionJumpStart, []string{"g"}, descFirstItem, ContextDownloads},
	{ActionJumpEnd, []string{"G"}, descLastItem, ContextDownloads},
	{ActionPageDown, []string{keyCtrlD}, descHalfPageDown, ContextDownloads},
	{ActionPageUp, []string{keyCtrlU}, descHalfPageUp, ContextDownloads},

	// Library-specific
	{ActionDelete, []string{"d"}, "Delete track", ContextLibrary},
	{ActionToggleFavorite, []string{"F"}, "Toggle favorite", ContextLibrary},
	{ActionToggleAlbumView, []string{"V"}, "Toggle album view", ContextLibrary},
	{ActionAddToPlaylist, []string{"ctrl+a"}, "Add to playlist", ContextLibrary},
	{ActionRetag, []string{"t"}, "Retag album", ContextLibrary},
	{ActionExport, []string{"e"}, "Export to a device or a folder", ContextLibrary},
	{ActionSimilarArtists, []string{"i"}, "Similar artists", ContextLibrary},

	// Album view options (o-sequence)
	{ActionOPrefix, []string{"o"}, "Options prefix", ContextAlbumView},
	{ActionAlbumGrouping, []string{"o g"}, "Album grouping", ContextAlbumView},
	{ActionAlbumSorting, []string{"o s"}, "Album sorting", ContextAlbumView},
	{ActionAlbumPresets, []string{"o p"}, "Album presets", ContextAlbumView},

	// File browser
	{ActionDelete, []string{"d"}, "Delete file/folder", ContextFileBrowser},

	// Queue panel
	{ActionToggleSelect, []string{"x"}, "Toggle selection", ContextQueue},
	{ActionDelete, []string{"d", "delete"}, "Delete selected", ContextQueue},
	{ActionKeepSelected, []string{"D"}, "Keep only selected", ContextQueue},
	{ActionClear, []string{"c"}, "Clear except playing", ContextQueue},
	{ActionMoveItemDown, []string{"J", "shift+down"}, "Move selected down", ContextQueue},
	{ActionMoveItemUp, []string{"K", "shift+up"}, "Move selected up", ContextQueue},
	{ActionSelect, []string{keyEnter}, "Play track", ContextQueue},
	{ActionClearSelect, []string{"esc"}, "Clear selection, or back to navigator", ContextQueue},
	{ActionToggleFavorite, []string{"F"}, "Toggle favorite", ContextQueue},
	{ActionAddToPlaylist, []string{"ctrl+a"}, "Add to playlist", ContextQueue},
	{ActionLocate, []string{"L"}, "Locate in navigator", ContextQueue},
	{ActionExport, []string{"e"}, "Export to a device or a folder", ContextQueue},
	{ActionSearch, []string{"/"}, "Search", ContextQueue},
	{ActionMoveDown, []string{"j", keyDown}, descMoveDown, ContextQueue},
	{ActionMoveUp, []string{"k", "up"}, descMoveUp, ContextQueue},
	{ActionJumpStart, []string{"g"}, descFirstItem, ContextQueue},
	{ActionJumpEnd, []string{"G"}, descLastItem, ContextQueue},
	{ActionPageDown, []string{keyCtrlD}, descHalfPageDown, ContextQueue},
	{ActionPageUp, []string{keyCtrlU}, descHalfPageUp, ContextQueue},

	// Queue history (global when queue available)
	{ActionUndo, []string{"ctrl+z"}, "Undo", ContextGlobal},
	{ActionRedo, []string{"ctrl+y"}, "Redo", ContextGlobal},

	// Playlist management
	{ActionNewPlaylist, []string{"n"}, "New playlist", ContextPlaylist},
	{ActionNewFolder, []string{"N"}, "New folder", ContextPlaylist},
	{ActionRename, []string{"ctrl+r"}, "Rename", ContextPlaylist},
	{ActionDelete, []string{keyCtrlD}, "Delete", ContextPlaylist},

	// Playlist track editing
	{ActionDelete, []string{"d"}, "Remove track", ContextPlaylistTrack},
	{ActionMoveItemDown, []string{"J"}, "Move track down", ContextPlaylistTrack},
	{ActionMoveItemUp, []string{"K"}, "Move track up", ContextPlaylistTrack},
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
