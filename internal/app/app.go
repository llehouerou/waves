// internal/app/app.go
package app

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/config"
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
	Player            player.Interface
	Queue             *playlist.PlayingQueue
	QueuePanel        queuepanel.Model
	QueueVisible      bool
	Focus             FocusTarget
	StateMgr          state.Interface
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

// New creates a new application model from configuration.
func New(cfg *config.Config, stateMgr *state.Manager) (Model, error) {
	// Determine start path: saved state > config default > cwd
	startPath := cfg.DefaultFolder
	var savedFileSelection string
	savedViewMode := ViewLibrary
	var savedLibrarySelection string

	if navState, err := stateMgr.GetNavigation(); err == nil && navState != nil {
		if _, statErr := os.Stat(navState.CurrentPath); statErr == nil {
			startPath = navState.CurrentPath
			savedFileSelection = navState.SelectedName
		}
		if navState.ViewMode != "" {
			savedViewMode = ViewMode(navState.ViewMode)
		}
		savedLibrarySelection = navState.LibrarySelectedID
	}

	if startPath == "" {
		var err error
		startPath, err = os.Getwd()
		if err != nil {
			return Model{}, err
		}
	}

	source, err := navigator.NewFileSource(startPath)
	if err != nil {
		return Model{}, err
	}

	fileNav, err := navigator.New(source)
	if err != nil {
		return Model{}, err
	}

	if savedFileSelection != "" {
		fileNav.FocusByName(savedFileSelection)
	}

	lib := library.New(stateMgr.DB())
	libSource := library.NewSource(lib)
	libNav, err := navigator.New(libSource)
	if err != nil {
		return Model{}, err
	}

	if savedLibrarySelection != "" {
		libNav.FocusByID(savedLibrarySelection)
	}

	queue := playlist.NewQueue()
	if queueState, err := stateMgr.GetQueue(); err == nil && queueState != nil {
		for _, t := range queueState.Tracks {
			queue.Add(playlist.Track{
				ID:          t.TrackID,
				Path:        t.Path,
				Title:       t.Title,
				Artist:      t.Artist,
				Album:       t.Album,
				TrackNumber: t.TrackNumber,
			})
		}
		if queueState.CurrentIndex >= 0 && queueState.CurrentIndex < queue.Len() {
			queue.JumpTo(queueState.CurrentIndex)
		}
		queue.SetRepeatMode(playlist.RepeatMode(queueState.RepeatMode))
		queue.SetShuffle(queueState.Shuffle)
	}
	queuePanel := queuepanel.New(queue)

	fileNav.SetFocused(true)
	libNav.SetFocused(true)

	return Model{
		ViewMode:          savedViewMode,
		FileNavigator:     fileNav,
		LibraryNavigator:  libNav,
		Library:           lib,
		LibrarySources:    cfg.LibrarySources,
		Player:            player.New(),
		Queue:             queue,
		QueuePanel:        queuePanel,
		QueueVisible:      true,
		Focus:             FocusNavigator,
		StateMgr:          stateMgr,
		Search:            search.New(),
		PlayerDisplayMode: playerbar.ModeExpanded,
	}, nil
}
