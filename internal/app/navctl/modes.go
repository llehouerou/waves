// Package navctl provides navigation control types and utilities.
package navctl

// ViewMode represents the current navigator view type.
type ViewMode string

const (
	// ViewLibrary shows the music library browser.
	ViewLibrary ViewMode = "library"
	// ViewFileBrowser shows the filesystem browser.
	ViewFileBrowser ViewMode = "file"
	// ViewPlaylists shows the playlists browser.
	ViewPlaylists ViewMode = "playlists"
	// ViewDownloads shows the downloads monitor.
	ViewDownloads ViewMode = "downloads"
)

// HasSearch reports whether the view mode has a search, local (/) and deep
// (f f). The Downloads view has none.
func (v ViewMode) HasSearch() bool {
	return v != ViewDownloads
}

// FocusTarget represents which UI component has focus.
type FocusTarget int

const (
	// FocusNavigator indicates the navigator panel has focus.
	FocusNavigator FocusTarget = iota
	// FocusQueue indicates the queue panel has focus.
	FocusQueue
)

// LibrarySubMode represents the library view sub-mode.
type LibrarySubMode int

const (
	LibraryModeMiller  LibrarySubMode = iota // Miller columns
	LibraryModeAlbum                         // Album view
	LibraryModeBrowser                       // Browser view (3 columns)
)
