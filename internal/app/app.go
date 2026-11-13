// internal/app/app.go
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/ui/albumart"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/librarynav"
	"github.com/llehouerou/waves/internal/ui/playlistnav"
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
	Navigation           *navctl.Manager
	DownloadsView        dlview.Model
	Popups               *popupctl.Manager
	Input                InputManager
	Layout               LayoutManager
	Core                 *core.Core // playback, queue, library, playlists, downloads, scrobbling, radio, MPRIS, desktop notifications
	Keys                 *keymap.Resolver
	LibraryScanJob       *jobbar.Job
	HasLibrarySources    bool
	Slskd                config.SlskdConfig       // slskd configuration
	MusicBrainz          config.MusicBrainzConfig // MusicBrainz configuration
	LastSeekTime         time.Time
	LastVolumeScrollTime time.Time // Debounce for mouse wheel volume control
	PendingTrackIdx      int
	TrackSkipVersion     int
	// Tick lifecycle (issue #28): tickGen is the only currently-valid tick
	// generation; tickRunning is true while a chain is alive. Together they
	// guarantee at most one live 1s tick chain. Mutate only via tick.go.
	tickGen     int
	tickRunning bool
	// panels is the header + navigator + queue block, rendered by Update for
	// every message but the playback tick (issue #54). View only reads it.
	panels string

	// Linking a Last.fm account: the core links it, the user authorizes the
	// token in a browser meanwhile.
	lastfmAuthToken string                 // the token awaiting authorization
	openBrowser     func(url string) error // nil: no browser is opened. Tests leave it so.

	// The exports the core runs, each as it was last reported, in the order
	// they were started: what the job bar shows of them.
	exports []core.ExportProgressed

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

	// Album art rendering
	AlbumArt                *albumart.Renderer
	albumArtPendingTransmit string // Transmission command to include in next View()
}

// initConfig holds configuration for deferred initialization.
type initConfig struct {
	cfg *config.Config
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
	return tea.Batch(m.WatchServiceEvents(), WatchStderr())
}

// New creates a new application model with deferred initialization, on a
// core it starts. The actual loading happens asynchronously after the UI starts.
func New(cfg *config.Config) (Model, error) {
	// Initialize downloads view with config status
	downloadsView := dlview.New()
	downloadsView.SetConfigured(cfg.HasSlskdConfig(), cfg.Slskd.CompletedPath)

	// Lyrics fetched from lrclib are kept in the user's cache; with nowhere
	// to keep them, they are fetched again each time.
	lyricsCache := ""
	if dir, err := os.UserCacheDir(); err == nil {
		lyricsCache = filepath.Join(dir, "waves", "lyrics")
	}

	database, err := core.DefaultDatabase()
	if err != nil {
		return Model{}, fmt.Errorf("find the database: %w", err)
	}

	// The core opens the database, restores the saved queue and volume, opens
	// the library, the playlists and the downloads, and runs on its own: it
	// also serves MPRIS and sends the desktop notifications.
	c, err := core.New(core.Options{
		Database:       database,
		Lastfm:         cfg.Lastfm,
		Radio:          cfg.GetRadioConfig(),
		LibrarySources: cfg.LibrarySources,
		Slskd:          cfg.Slskd,
		Rename:         cfg.Rename,
		LyricsCache:    lyricsCache,
		Notifications:  cfg.GetNotificationsConfig(),
		MPRIS:          true,
	})
	if err != nil {
		return Model{}, fmt.Errorf("open the database %s: %w", database, err)
	}

	return Model{
		Navigation:    navctl.New(),
		DownloadsView: downloadsView,
		Popups:        popupctl.New(),
		Input:         NewInputManager(),
		Layout:        NewLayoutManager(queuepanel.New(c.Playback())),
		Core:          c,
		Keys:          keymap.NewResolver(keymap.Bindings),
		Slskd:         cfg.Slskd,
		MusicBrainz:   cfg.MusicBrainz,
		openBrowser:   openInBrowser,
		loadingState:  loadingWaiting,
		LoadingStatus: "Loading navigators...",
		initConfig:    &initConfig{cfg: cfg},
		AlbumArt:      newAlbumArt(),
	}, nil
}

// newAlbumArt creates the album art renderer. Detection always yields a
// renderer (unicode blocks when the terminal has no graphics protocol), except
// when the user disables art with WAVES_IMAGE_PROTOCOL=none.
func newAlbumArt() *albumart.Renderer {
	protocol := albumart.Detect()
	if protocol == nil {
		return nil
	}
	return albumart.New(protocol)
}

// startInitialization returns a command that performs async initialization.
func (m Model) startInitialization() tea.Cmd {
	cfg := m.initConfig.cfg
	c := m.Core

	return func() tea.Msg {
		result := InitResult{SavedView: navctl.ViewLibrary}

		// Load saved navigation state
		startPath := cfg.DefaultFolder
		var savedFileSelection string
		var savedLibrarySelection string
		var savedPlaylistsSelection string

		navState, err := c.ClientState().GetNavigation()
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
			result.SavedBrowserState = navState.BrowserSelectedState
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
		libSource := librarynav.NewSource(c.Library())
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

		// Initialize library browser
		browser := librarybrowser.New(c.Library())
		_ = browser.Refresh()
		result.LibraryBrowser = browser

		// Initialize playlists navigator
		plsSource := playlistnav.NewSource(c.Playlists())
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

		return result
	}
}
