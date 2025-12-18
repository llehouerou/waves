// Package keymap defines key bindings and action dispatch for the application.
package keymap

// Action represents a user-triggerable action.
type Action string

const (
	// Global actions
	ActionQuit        Action = "quit"
	ActionSwitchFocus Action = "switch_focus"
	ActionToggleQueue Action = "toggle_queue"
	ActionSearch      Action = "search"
	ActionHelp        Action = "help"

	// View switching
	ActionViewLibrary     Action = "view_library"
	ActionViewFileBrowser Action = "view_file_browser"
	ActionViewPlaylists   Action = "view_playlists"
	ActionViewDownloads   Action = "view_downloads"

	// Key sequence prefixes
	ActionFPrefix Action = "f_prefix"
	ActionOPrefix Action = "o_prefix"

	// F-sequence actions (f + key)
	ActionDeepSearch       Action = "deep_search"
	ActionRefreshLibrary   Action = "refresh_library"
	ActionFullRescan       Action = "full_rescan"
	ActionLibrarySources   Action = "library_sources"
	ActionDownloadSoulseek Action = "download_soulseek"
	ActionLastfmSettings   Action = "lastfm_settings"

	// O-sequence actions (o + key) - album view options
	ActionAlbumGrouping Action = "album_grouping"
	ActionAlbumSorting  Action = "album_sorting"
	ActionAlbumPresets  Action = "album_presets"

	// Playback actions
	ActionPlayPause           Action = "play_pause"
	ActionStop                Action = "stop"
	ActionNextTrack           Action = "next_track"
	ActionPrevTrack           Action = "prev_track"
	ActionFirstTrack          Action = "first_track"
	ActionLastTrack           Action = "last_track"
	ActionSeekForward         Action = "seek_forward"
	ActionSeekBack            Action = "seek_back"
	ActionSeekForwardLong     Action = "seek_forward_long"
	ActionSeekBackLong        Action = "seek_back_long"
	ActionTogglePlayerDisplay Action = "toggle_player_display"
	ActionCycleRepeat         Action = "cycle_repeat"
	ActionToggleShuffle       Action = "toggle_shuffle"

	// Navigation actions
	ActionMoveUp    Action = "move_up"
	ActionMoveDown  Action = "move_down"
	ActionMoveLeft  Action = "move_left"
	ActionMoveRight Action = "move_right"
	ActionJumpStart Action = "jump_start"
	ActionJumpEnd   Action = "jump_end"
	ActionPageUp    Action = "page_up"
	ActionPageDown  Action = "page_down"

	// Selection/activation actions
	ActionSelect        Action = "select"          // enter - play/activate
	ActionAdd           Action = "add"             // a - add to queue
	ActionAddToPlaylist Action = "add_to_playlist" // ctrl+a

	// Generic contextual actions
	ActionDelete       Action = "delete"        // d/delete - context determines what
	ActionToggleSelect Action = "toggle_select" // x - toggle selection
	ActionClearSelect  Action = "clear_select"  // esc - clear selection
	ActionClear        Action = "clear"         // c - clear queue except playing

	// Queue-specific actions
	ActionMoveItemUp   Action = "move_item_up"   // shift+k
	ActionMoveItemDown Action = "move_item_down" // shift+j
	ActionUndo         Action = "undo"           // ctrl+z
	ActionRedo         Action = "redo"           // ctrl+shift+z

	// Library-specific actions
	ActionToggleFavorite  Action = "toggle_favorite"   // F
	ActionToggleAlbumView Action = "toggle_album_view" // V
	ActionRetag           Action = "retag"             // t

	// Playlist management actions
	ActionNewPlaylist Action = "new_playlist" // n
	ActionNewFolder   Action = "new_folder"   // N
	ActionRename      Action = "rename"       // ctrl+r
)
