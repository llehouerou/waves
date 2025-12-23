// internal/app/app.go
package app

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/radio"
	"github.com/llehouerou/waves/internal/state"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
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
	Downloads         *downloads.Manager
	DownloadsView     dlview.Model
	Popups            PopupManager
	Input             InputManager
	Layout            LayoutManager
	Playback          PlaybackManager
	Keys              *keymap.Resolver
	LibraryScanCh     <-chan library.ScanProgress
	LibraryScanJob    *jobbar.Job
	HasLibrarySources bool
	HasSlskdConfig    bool                     // True if slskd integration is configured
	Slskd             config.SlskdConfig       // slskd configuration
	MusicBrainz       config.MusicBrainzConfig // MusicBrainz configuration
	StateMgr          state.Interface
	LastSeekTime      time.Time
	PendingTrackIdx   int
	TrackSkipVersion  int
	Favorites         map[int64]bool // Track IDs that are favorited

	// Last.fm scrobbling
	Lastfm          *lastfm.Client       // nil if not configured
	LastfmSession   *state.LastfmSession // nil if not linked
	ScrobbleState   *lastfm.ScrobbleState
	HasLastfmConfig bool
	lastfmAuthToken string // Token awaiting authorization (desktop auth flow)

	// Radio mode
	Radio              *radio.Radio // nil if Last.fm not configured
	RadioConfig        config.RadioConfig
	RadioFillTriggered bool // True if radio fill was triggered for current track

	// Export
	ExportRepo   *export.TargetRepository
	ExportJobs   map[string]*export.Job
	ExportParams map[string]export.Params // Active export params by job ID

	// Notifications (temporary messages with independent timeouts)
	Notifications      []Notification
	nextNotificationID int64

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
			WatchStderr(),              // Watch for stderr output from C libraries
		)
	}
	return tea.Batch(m.WatchTrackFinished(), WatchStderr())
}

// New creates a new application model with deferred initialization.
// The actual loading happens asynchronously after the UI starts.
func New(cfg *config.Config, stateMgr *state.Manager) (Model, error) {
	lib := library.New(stateMgr.DB())
	pls := playlists.New(stateMgr.DB(), lib)
	dl := downloads.New(stateMgr.DB())
	queue := playlist.NewQueue()
	p := player.New()

	// Initialize Last.fm client if configured
	var lfmClient *lastfm.Client
	var lfmSession *state.LastfmSession
	var radioInstance *radio.Radio
	radioConfig := cfg.GetRadioConfig()
	hasLastfmConfig := cfg.HasLastfmConfig()
	if hasLastfmConfig {
		lfmClient = lastfm.New(cfg.Lastfm.APIKey, cfg.Lastfm.APISecret)
		// Load saved session
		if sess, err := stateMgr.GetLastfmSession(); err == nil && sess != nil {
			lfmSession = sess
			lfmClient.SetSessionKey(sess.SessionKey)
		}
		// Initialize radio instance
		radioInstance = radio.New(stateMgr.DB(), lfmClient, lib, radioConfig)
	}

	// Initialize downloads view with config status
	downloadsView := dlview.New()
	downloadsView.SetConfigured(cfg.HasSlskdConfig())

	return Model{
		Navigation:      NewNavigationManager(),
		Library:         lib,
		Playlists:       pls,
		Downloads:       dl,
		DownloadsView:   downloadsView,
		Popups:          NewPopupManager(),
		Input:           NewInputManager(),
		Layout:          NewLayoutManager(queuepanel.New(queue)),
		Playback:        NewPlaybackManager(p, queue),
		Keys:            keymap.NewResolver(keymap.Bindings),
		StateMgr:        stateMgr,
		HasSlskdConfig:  cfg.HasSlskdConfig(),
		Slskd:           cfg.Slskd,
		MusicBrainz:     cfg.MusicBrainz,
		Lastfm:          lfmClient,
		LastfmSession:   lfmSession,
		HasLastfmConfig: hasLastfmConfig,
		Radio:           radioInstance,
		RadioConfig:     radioConfig,
		ExportRepo:      export.NewTargetRepository(stateMgr.DB()),
		ExportJobs:      make(map[string]*export.Job),
		ExportParams:    make(map[string]export.Params),
		loadingState:    loadingWaiting,
		LoadingStatus:   "Loading navigators...",
		initConfig:      &initConfig{cfg: cfg, stateMgr: stateMgr},
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
			result.SavedLibrarySubMode = navState.LibrarySubMode
			result.SavedAlbumSelectedID = navState.AlbumSelectedID
			result.SavedAlbumGroupFields = navState.AlbumGroupFields
			result.SavedAlbumSortCriteria = navState.AlbumSortCriteria
		} else if err == nil && navState == nil {
			// No saved navigation state - this is first launch
			result.IsFirstLaunch = true
		}

		if startPath == "" {
			var err error
			startPath, err = os.Getwd()
			if err != nil {
				result.Err = err
				return result
			}
		}

		// Initialize file navigator
		source, err := navigator.NewFileSource(startPath)
		if err != nil {
			result.Err = err
			return result
		}

		fileNav, err := navigator.New(source)
		if err != nil {
			result.Err = err
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
			result.Err = err
			return result
		}

		libSource := library.NewSource(lib)
		libNav, err := navigator.New(libSource)
		if err != nil {
			result.Err = err
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
			result.Err = err
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
				queue.AddWithoutHistory(playlist.Track{
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
			queue.SaveToHistory() // Save loaded state as initial history entry
		}
		result.Queue = queue
		result.QueuePanel = queuepanel.New(queue)

		return result
	}
}

// isLastfmLinked returns true if Last.fm is configured and authenticated.
func (m *Model) isLastfmLinked() bool {
	return m.HasLastfmConfig && m.Lastfm != nil && m.Lastfm.IsAuthenticated()
}
