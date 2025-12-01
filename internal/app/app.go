// internal/app/app.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// Model is the root application model containing all state.
type Model struct {
	ViewMode          ViewMode
	FileNavigator     navigator.Model[navigator.FileNode]
	LibraryNavigator  navigator.Model[library.Node]
	Library           *library.Library
	LibrarySources    []string
	LibraryScanCh     <-chan library.ScanProgress
	LibraryScanMsg    string
	Player            *player.Player
	Queue             *playlist.PlayingQueue
	QueuePanel        queuepanel.Model
	QueueVisible      bool
	Focus             FocusTarget
	StateMgr          *state.Manager
	Search            search.Model
	SearchMode        bool
	PlayerDisplayMode playerbar.DisplayMode
	ScanChan          <-chan navigator.ScanResult
	CancelScan        context.CancelFunc
	PendingKeys       string
	ErrorMsg          string
	LastSeekTime      time.Time
	PendingTrackIdx   int
	TrackSkipVersion  int
	Width             int
	Height            int
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}
