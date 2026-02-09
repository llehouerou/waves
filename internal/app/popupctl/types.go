// internal/app/popupctl/types.go
package popupctl

// Type identifies which popup is currently active.
type Type int

const (
	None Type = iota
	Help
	Confirm
	TextInput
	LibrarySources
	ScanReport
	Error
	Download
	Import
	Retag
	AlbumGrouping
	AlbumSorting
	AlbumPresets
	LastfmAuth
	Export
	Lyrics
	SimilarArtists
)

// Priority defines which popup takes precedence (highest priority first).
var Priority = []Type{
	Error,
	ScanReport,
	Help,
	Confirm,
	TextInput,
	LibrarySources,
	AlbumGrouping,
	AlbumSorting,
	AlbumPresets,
	LastfmAuth,
	Export,
	Lyrics,
	SimilarArtists,
	Download,
	Import,
	Retag,
}

// RenderOrder defines the order popups are rendered (bottom to top).
var RenderOrder = []Type{
	Retag,
	Import,
	Download,
	SimilarArtists,
	Lyrics,
	Export,
	LastfmAuth,
	AlbumPresets,
	AlbumSorting,
	AlbumGrouping,
	LibrarySources,
	TextInput,
	Confirm,
	ScanReport,
	Help,
	Error,
}

// InputMode represents the type of text input being collected.
type InputMode int

const (
	// InputNone indicates no text input is active.
	InputNone InputMode = iota
	// InputNewPlaylist indicates creating a new playlist.
	InputNewPlaylist
	// InputNewFolder indicates creating a new folder.
	InputNewFolder
	// InputRename indicates renaming a playlist or folder.
	InputRename
)
