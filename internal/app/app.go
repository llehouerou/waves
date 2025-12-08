// internal/app/app.go
package app

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// loadingPhase represents the current state of the loading screen.
type loadingPhase int

const (
	// loadingWaiting means init is running but loading screen not yet visible.
	loadingWaiting loadingPhase = iota
	// loadingShowing means loading screen is visible.
	loadingShowing
	// loadingDone means loading is complete, show normal UI.
	loadingDone
)

// Model is the root application model containing all state.
type Model struct {
	Navigation        NavigationManager
	Library           *library.Library
	Playlists         *playlists.Playlists
	Popups            PopupManager
	Input             InputManager
	Layout            LayoutManager
	Playback          PlaybackManager
	LibraryScanCh     <-chan library.ScanProgress
	LibraryScanJob    *jobbar.Job
	HasLibrarySources bool
	StateMgr          state.Interface
	LastSeekTime      time.Time
	PendingTrackIdx   int
	TrackSkipVersion  int

	// Loading state
	loadingState       loadingPhase // Current loading phase
	loadingInitDone    bool         // True when InitResult received
	loadingShowTime    time.Time    // When loading screen became visible
	loadingFirstLaunch bool         // True if this is first app launch
	LoadingStatus      string
	LoadingFrame       int         // Animation frame counter
	initConfig         *initConfig // Stored config for deferred initialization
}

// initConfig holds configuration for deferred initialization.
type initConfig struct {
	cfg      *config.Config
	stateMgr *state.Manager
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.loadingState == loadingWaiting && m.initConfig != nil {
		return tea.Batch(
			m.startInitialization(),
			ShowLoadingAfterDelayCmd(), // Show loading screen after 400ms if init not done
		)
	}
	return m.WatchTrackFinished()
}

// New creates a new application model with deferred initialization.
// The actual loading happens asynchronously after the UI starts.
func New(cfg *config.Config, stateMgr *state.Manager) (Model, error) {
	lib := library.New(stateMgr.DB())
	pls := playlists.New(stateMgr.DB(), lib)
	queue := playlist.NewQueue()
	p := player.New()

	return Model{
		Navigation:    NewNavigationManager(),
		Library:       lib,
		Playlists:     pls,
		Popups:        NewPopupManager(),
		Input:         NewInputManager(),
		Layout:        NewLayoutManager(queuepanel.New(queue)),
		Playback:      NewPlaybackManager(p, queue),
		StateMgr:      stateMgr,
		loadingState:  loadingWaiting,
		LoadingStatus: "Loading navigators...",
		initConfig:    &initConfig{cfg: cfg, stateMgr: stateMgr},
	}, nil
}

// startInitialization returns a command that performs async initialization.
func (m Model) startInitialization() tea.Cmd {
	cfg := m.initConfig.cfg
	stateMgr := m.initConfig.stateMgr

	return func() tea.Msg {
		result := InitResult{SavedView: ViewLibrary}

		// Load saved navigation state
		startPath := cfg.DefaultFolder
		var savedFileSelection string
		var savedLibrarySelection string
		var savedPlaylistsSelection string

		navState, err := stateMgr.GetNavigation()
		if err == nil && navState != nil {
			if _, statErr := os.Stat(navState.CurrentPath); statErr == nil {
				startPath = navState.CurrentPath
				savedFileSelection = navState.SelectedName
			}
			if navState.ViewMode != "" {
				result.SavedView = ViewMode(navState.ViewMode)
			}
			savedLibrarySelection = navState.LibrarySelectedID
			savedPlaylistsSelection = navState.PlaylistsSelectedID
		} else if err == nil && navState == nil {
			// No saved navigation state - this is first launch
			result.IsFirstLaunch = true
		}

		if startPath == "" {
			var err error
			startPath, err = os.Getwd()
			if err != nil {
				result.Error = err
				return result
			}
		}

		// Initialize file navigator
		source, err := navigator.NewFileSource(startPath)
		if err != nil {
			result.Error = err
			return result
		}

		fileNav, err := navigator.New(source)
		if err != nil {
			result.Error = err
			return result
		}

		if savedFileSelection != "" {
			fileNav.FocusByName(savedFileSelection)
		}
		fileNav.SetFocused(true)
		result.FileNav = fileNav

		// Initialize library navigator
		lib := library.New(stateMgr.DB())

		// Migrate library sources from config to DB if needed
		if err := lib.MigrateSources(cfg.LibrarySources); err != nil {
			result.Error = err
			return result
		}

		libSource := library.NewSource(lib)
		libNav, err := navigator.New(libSource)
		if err != nil {
			result.Error = err
			return result
		}

		if savedLibrarySelection != "" {
			libNav.FocusByID(savedLibrarySelection)
		}
		libNav.SetFocused(true)
		result.LibNav = libNav

		// Initialize playlists navigator
		pls := playlists.New(stateMgr.DB(), lib)
		plsSource := playlists.NewSource(pls)
		plsNav, err := navigator.New(plsSource)
		if err != nil {
			result.Error = err
			return result
		}

		if savedPlaylistsSelection != "" {
			plsNav.FocusByID(savedPlaylistsSelection)
		}
		plsNav.SetFocused(true)
		result.PlsNav = plsNav

		// Restore queue state
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
		result.Queue = queue
		result.QueuePanel = queuepanel.New(queue)

		return result
	}
}
