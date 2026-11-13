// internal/app/interfaces.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/ui/librarynav"
	"github.com/llehouerou/waves/internal/ui/librarysources"
	"github.com/llehouerou/waves/internal/ui/playlistnav"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
	"github.com/llehouerou/waves/internal/ui/search"
)

// Compile-time assertions that managers satisfy their interfaces.
var (
	_ PopupController      = (*popupctl.Manager)(nil)
	_ InputController      = (*InputManager)(nil)
	_ LayoutController     = (*LayoutManager)(nil)
	_ NavigationController = (*navctl.Manager)(nil)
)

// PopupController manages modal popups and overlays.
type PopupController interface {
	// SetSize updates dimensions for popup rendering.
	SetSize(width, height int)

	// Visibility management
	ActivePopup() popupctl.Type
	IsVisible(t popupctl.Type) bool
	Hide(t popupctl.Type)

	// Show methods (type-specific parameters)
	ShowHelp(contexts []string) tea.Cmd
	ShowConfirm(title, message string, context any) tea.Cmd
	ShowConfirmWithOptions(title, message string, options []string, context any) tea.Cmd
	ShowTextInput(mode popupctl.InputMode, title, value string, context any) tea.Cmd
	ShowLibrarySources(sources []string) tea.Cmd
	ShowScanReport(stats *core.ScanStats) tea.Cmd
	ShowError(msg string)

	// Type-specific accessors
	LibrarySources() *librarysources.Model
	InputMode() popupctl.InputMode
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

// NavigationController manages view modes, focus state, and navigators.
type NavigationController interface {
	// View mode
	ViewMode() navctl.ViewMode
	SetViewMode(mode navctl.ViewMode)

	// Focus
	Focus() navctl.FocusTarget
	SetFocus(target navctl.FocusTarget)
	IsNavigatorFocused() bool
	IsQueueFocused() bool

	// Navigator accessors
	FileNav() *navigator.Model[navigator.FileNode]
	LibraryNav() *navigator.Model[librarynav.Node]
	PlaylistNav() *navigator.Model[playlistnav.Node]
	SetFileNav(nav navigator.Model[navigator.FileNode])
	SetLibraryNav(nav navigator.Model[librarynav.Node])
	SetPlaylistNav(nav navigator.Model[playlistnav.Node])

	// Navigation helpers
	CurrentNavigator() navigator.Node
	UpdateActiveNavigator(msg tea.Msg) tea.Cmd
	ResizeNavigators(msg tea.WindowSizeMsg)
	RefreshLibrary(preserveSelection bool)
	RefreshPlaylists(preserveSelection bool)

	// View rendering
	RenderActiveNavigator() string
}
