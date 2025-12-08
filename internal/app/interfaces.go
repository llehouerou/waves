// internal/app/interfaces.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/librarysources"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
	"github.com/llehouerou/waves/internal/ui/scanreport"
)

// Compile-time assertions that managers satisfy their interfaces.
var (
	_ PopupController      = (*PopupManager)(nil)
	_ InputController      = (*InputManager)(nil)
	_ LayoutController     = (*LayoutManager)(nil)
	_ PlaybackController   = (*PlaybackManager)(nil)
	_ NavigationController = (*NavigationManager)(nil)
)

// PopupController manages modal popups and overlays.
type PopupController interface {
	// SetSize updates dimensions for popup rendering.
	SetSize(width, height int)

	// Visibility management
	ActivePopup() PopupType
	IsVisible(t PopupType) bool
	Hide(t PopupType)

	// Show methods (type-specific parameters)
	ShowHelp(contexts []string)
	ShowConfirm(title, message string, context any)
	ShowConfirmWithOptions(title, message string, options []string, context any)
	ShowTextInput(mode InputMode, title, value string, context any)
	ShowLibrarySources(sources []string)
	ShowScanReport(report scanreport.Model)
	ShowError(msg string)

	// Type-specific accessors
	Help() *helpbindings.Model
	LibrarySources() *librarysources.Model
	InputMode() InputMode
	ErrorMsg() string

	// Key handling
	HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd)

	// Rendering
	RenderOverlay(base string) string
}

// InputController manages search mode, key sequences, and input state.
type InputController interface {
	// Search mode
	SearchMode() SearchMode
	IsSearchActive() bool
	IsLocalSearch() bool
	IsDeepSearch() bool
	IsAddToPlaylistSearch() bool

	// Starting search modes
	StartLocalSearch(items []search.Item)
	EndSearch()

	// Key sequences
	PendingKeys() string
	HasPendingKeys() bool
	StartKeySequence(key string)
	ClearKeySequence()
	IsKeySequence(seq string) bool

	// Normal mode
	IsNormalMode() bool

	// Search component access
	Search() *search.Model
	UpdateSearch(msg tea.Msg) tea.Cmd
	SearchView() string

	// Size
	SetSize(msg tea.WindowSizeMsg)
}

// LayoutController manages window dimensions and panel visibility.
type LayoutController interface {
	// Dimensions
	SetSize(width, height int)
	Width() int
	Height() int
	Dimensions() (width, height int)

	// Queue visibility
	ToggleQueue()
	ShowQueue()
	HideQueue()
	IsQueueVisible() bool

	// Queue panel
	QueuePanel() *queuepanel.Model
	SetQueuePanel(panel queuepanel.Model)

	// Layout calculations
	NavigatorWidth() int
	QueueWidth() int
	ResizeQueuePanel(height int)
}

// PlaybackController manages audio playback, queue, and display mode.
type PlaybackController interface {
	// Player access
	Player() player.Interface
	SetPlayer(p player.Interface)

	// Queue access
	Queue() *playlist.PlayingQueue
	SetQueue(q *playlist.PlayingQueue)

	// Player state
	State() player.State
	IsPlaying() bool
	IsPaused() bool
	IsStopped() bool

	// Playback controls
	Play(path string) error
	Pause()
	Resume()
	Toggle()
	Stop()
	Seek(delta time.Duration)

	// Position and duration
	Position() time.Duration
	Duration() time.Duration

	// Current track
	CurrentTrack() *playlist.Track

	// Display mode
	DisplayMode() playerbar.DisplayMode
	SetDisplayMode(mode playerbar.DisplayMode)
	ToggleDisplayMode()

	// Finished channel for track completion
	FinishedChan() <-chan struct{}
}

// NavigationController manages view modes, focus state, and navigators.
type NavigationController interface {
	// View mode
	ViewMode() ViewMode
	SetViewMode(mode ViewMode)

	// Focus
	Focus() FocusTarget
	SetFocus(target FocusTarget)
	IsNavigatorFocused() bool
	IsQueueFocused() bool

	// Navigator accessors
	FileNav() *navigator.Model[navigator.FileNode]
	LibraryNav() *navigator.Model[library.Node]
	PlaylistNav() *navigator.Model[playlists.Node]
	SetFileNav(nav navigator.Model[navigator.FileNode])
	SetLibraryNav(nav navigator.Model[library.Node])
	SetPlaylistNav(nav navigator.Model[playlists.Node])

	// Navigation helpers
	CurrentNavigator() navigator.Node
	UpdateActiveNavigator(msg tea.Msg) tea.Cmd
	ResizeNavigators(msg tea.WindowSizeMsg)
	RefreshLibrary(preserveSelection bool)
	RefreshPlaylists(preserveSelection bool)

	// View rendering
	RenderActiveNavigator() string
}
