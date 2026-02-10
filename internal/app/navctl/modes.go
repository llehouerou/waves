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

// SupportsDeepSearch returns true if the view mode supports deep search (g f).
func (v ViewMode) SupportsDeepSearch() bool {
	return v == ViewFileBrowser || v == ViewLibrary
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
