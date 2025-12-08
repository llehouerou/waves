// internal/app/interfaces.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
	"github.com/llehouerou/waves/internal/ui/scanreport"
)

// Compile-time assertions that managers satisfy their interfaces.
var (
	_ PopupController  = (*PopupManager)(nil)
	_ InputController  = (*InputManager)(nil)
	_ LayoutController = (*LayoutManager)(nil)
)

// PopupController manages modal popups and overlays.
type PopupController interface {
	// SetSize updates dimensions for popup rendering.
	SetSize(width, height int)

	// ActivePopup returns which popup is currently active (if any).
	ActivePopup() PopupType

	// Help popup
	ShowHelp(contexts []string)
	HideHelp()
	IsHelpVisible() bool
	Help() *helpbindings.Model

	// Confirm popup
	ShowConfirm(title, message string, context any)
	ShowConfirmWithOptions(title, message string, options []string, context any)
	HideConfirm()
	IsConfirmVisible() bool

	// Text input popup
	ShowTextInput(mode InputMode, title, value string, context any)
	HideTextInput()
	IsTextInputVisible() bool
	InputMode() InputMode

	// Library sources popup
	ShowLibrarySources(sources []string)
	HideLibrarySources()
	IsLibrarySourcesVisible() bool

	// Scan report popup
	ShowScanReport(report scanreport.Model)
	HideScanReport()
	IsScanReportVisible() bool

	// Error popup
	ShowError(msg string)
	HideError()
	IsErrorVisible() bool
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
