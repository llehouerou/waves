package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/popup"
)

type tickMsg time.Time

type scanResultMsg navigator.ScanResult

type keySequenceTimeoutMsg struct{}

type libraryScanProgressMsg library.ScanProgress

type libraryScanCompleteMsg struct{}

type ViewMode string

const (
	ViewLibrary     ViewMode = "library"
	ViewFileBrowser ViewMode = "file"
)

type model struct {
	viewMode          ViewMode
	fileNavigator     navigator.Model[navigator.FileNode]
	libraryNavigator  navigator.Model[library.Node]
	library           *library.Library
	librarySources    []string
	libraryScanCh     <-chan library.ScanProgress
	libraryScanMsg    string // current scan status message
	player            *player.Player
	stateMgr          *state.Manager
	search            search.Model
	searchMode        bool
	playerDisplayMode playerbar.DisplayMode
	scanChan          <-chan navigator.ScanResult
	cancelScan        context.CancelFunc
	pendingKeys       string // buffered keys for sequences like "space ff"
	errorMsg          string // error message to display in overlay
	width             int
	height            int
}

func initialModel() (model, error) {
	cfg, err := config.Load()
	if err != nil {
		return model{}, err
	}

	// Initialize icons based on config
	icons.Init(cfg.Icons)

	// Open state manager
	stateMgr, err := state.Open()
	if err != nil {
		return model{}, err
	}

	// Determine start path: saved state > config default > cwd
	startPath := cfg.DefaultFolder
	var savedFileSelection string
	savedViewMode := ViewLibrary
	var savedLibrarySelection string

	if navState, err := stateMgr.GetNavigation(); err == nil && navState != nil {
		// Check if saved path still exists
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
		startPath, err = os.Getwd()
		if err != nil {
			stateMgr.Close()
			return model{}, err
		}
	}

	source, err := navigator.NewFileSource(startPath)
	if err != nil {
		stateMgr.Close()
		return model{}, err
	}

	fileNav, err := navigator.New(source)
	if err != nil {
		stateMgr.Close()
		return model{}, err
	}

	// Restore file browser selection if we have one
	if savedFileSelection != "" {
		fileNav.FocusByName(savedFileSelection)
	}

	// Initialize library
	lib := library.New(stateMgr.DB())
	libSource := library.NewSource(lib)
	libNav, err := navigator.New(libSource)
	if err != nil {
		stateMgr.Close()
		return model{}, err
	}

	// Restore library selection if we have one
	if savedLibrarySelection != "" {
		libNav.FocusByID(savedLibrarySelection)
	}

	return model{
		viewMode:         savedViewMode,
		fileNavigator:    fileNav,
		libraryNavigator: libNav,
		library:          lib,
		librarySources:   cfg.LibrarySources,
		player:           player.New(),
		stateMgr:         stateMgr,
		search:           search.New(),
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) navigatorHeight() int {
	height := m.height
	if m.player.State() != player.Stopped {
		height -= playerbar.Height(m.playerDisplayMode) - 2
	}
	return height
}

func (m *model) saveState() {
	m.stateMgr.SaveNavigation(state.NavigationState{
		CurrentPath:       m.fileNavigator.CurrentPath(),
		SelectedName:      m.fileNavigator.SelectedName(),
		ViewMode:          string(m.viewMode),
		LibrarySelectedID: m.libraryNavigator.SelectedID(),
	})
}

func (m *model) handleEnter() tea.Cmd {
	var path string

	switch m.viewMode {
	case ViewFileBrowser:
		selected := m.fileNavigator.Selected()
		if selected == nil || selected.IsContainer() {
			return nil
		}
		path = selected.ID()
	case ViewLibrary:
		selected := m.libraryNavigator.Selected()
		if selected == nil || selected.IsContainer() {
			return nil
		}
		path = selected.Path()
	}

	if path == "" || !player.IsMusicFile(path) {
		return nil
	}

	if err := m.player.Play(path); err != nil {
		m.errorMsg = err.Error()
		return nil
	}

	// Resize navigator for player bar
	sizeMsg := tea.WindowSizeMsg{Width: m.width, Height: m.navigatorHeight()}
	if m.viewMode == ViewFileBrowser {
		m.fileNavigator, _ = m.fileNavigator.Update(sizeMsg)
	} else {
		m.libraryNavigator, _ = m.libraryNavigator.Update(sizeMsg)
	}
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update search model size
		m.search, _ = m.search.Update(msg)

		// Update both navigators with new size
		navSizeMsg := tea.WindowSizeMsg{Width: msg.Width, Height: m.navigatorHeight()}
		m.fileNavigator, _ = m.fileNavigator.Update(navSizeMsg)
		m.libraryNavigator, _ = m.libraryNavigator.Update(navSizeMsg)
		return m, nil

	case navigator.NavigationChangedMsg:
		m.saveState()
		return m, nil

	case scanResultMsg:
		m.search.SetItems(msg.Items)
		m.search.SetLoading(!msg.Done)
		if !msg.Done {
			return m, m.waitForScan()
		}
		return m, nil

	case search.ResultMsg:
		m.searchMode = false
		m.scanChan = nil
		// Cancel any ongoing scan
		if m.cancelScan != nil {
			m.cancelScan()
			m.cancelScan = nil
		}
		if !msg.Canceled && msg.Item != nil {
			switch item := msg.Item.(type) {
			case navigator.FileItem:
				m.fileNavigator.NavigateTo(item.Path)
			case library.SearchItem:
				m.handleLibrarySearchResult(item.Result)
			}
		}
		m.search.Reset()
		return m, nil

	case libraryScanProgressMsg:
		switch msg.Phase {
		case "scanning":
			m.libraryScanMsg = fmt.Sprintf("Scanning... %d files found", msg.Current)
		case "processing":
			m.libraryScanMsg = fmt.Sprintf("Processing %d/%d: %s", msg.Current, msg.Total, msg.CurrentFile)
		case "cleaning":
			m.libraryScanMsg = "Cleaning up..."
		case "done":
			m.libraryScanMsg = ""
			m.libraryScanCh = nil
			// Refresh library navigator to show new data
			libSource := library.NewSource(m.library)
			if newNav, err := navigator.New(libSource); err == nil {
				m.libraryNavigator = newNav
				// Set size on new navigator
				m.libraryNavigator, _ = m.libraryNavigator.Update(tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.navigatorHeight(),
				})
			}
			return m, nil
		}
		return m, m.waitForLibraryScan()

	case libraryScanCompleteMsg:
		m.libraryScanMsg = ""
		m.libraryScanCh = nil
		return m, nil

	case keySequenceTimeoutMsg:
		// Timeout occurred, execute buffered space action
		if m.pendingKeys == " " {
			m.pendingKeys = ""
			m.player.Toggle()
		}
		return m, nil

	case tea.KeyMsg:
		// Handle error overlay - any key dismisses it
		if m.errorMsg != "" {
			m.errorMsg = ""
			return m, nil
		}

		// Handle search mode
		if m.searchMode {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
		}

		key := msg.String()

		// Handle key sequences starting with space
		if m.pendingKeys != "" {
			m.pendingKeys += key
			switch {
			case m.pendingKeys == " ff" && m.viewMode == ViewFileBrowser:
				// Deep search: recursive scan (file browser only)
				m.pendingKeys = ""
				m.searchMode = true
				m.search.SetLoading(true)
				ctx, cancel := context.WithCancel(context.Background())
				m.cancelScan = cancel
				m.scanChan = navigator.ScanDir(ctx, m.fileNavigator.CurrentPath())
				return m, m.waitForScan()
			case m.pendingKeys == " lr" && m.viewMode == ViewLibrary:
				// Library refresh (library view only)
				m.pendingKeys = ""
				if len(m.librarySources) > 0 && m.libraryScanCh == nil {
					ch := make(chan library.ScanProgress)
					m.libraryScanCh = ch
					go func() {
						_ = m.library.Refresh(m.librarySources, ch)
					}()
					return m, m.waitForLibraryScan()
				}
				return m, nil
			case len(m.pendingKeys) >= 3 || !isValidSequencePrefix(m.pendingKeys):
				// Invalid sequence, execute buffered space action and reset
				m.pendingKeys = ""
				m.player.Toggle()
			}
			return m, nil
		}

		switch key {
		case "q", "ctrl+c":
			m.player.Stop()
			m.stateMgr.Close()
			return m, tea.Quit
		case "f1":
			m.viewMode = ViewLibrary
			m.saveState()
			return m, nil
		case "f2":
			m.viewMode = ViewFileBrowser
			m.saveState()
			return m, nil
		case "/":
			// Shallow search: current items only
			m.searchMode = true
			if m.viewMode == ViewFileBrowser {
				m.search.SetItems(m.currentDirSearchItems())
			} else {
				m.search.SetItems(m.currentLibrarySearchItems())
			}
			m.search.SetLoading(false)
			return m, nil
		case "enter":
			if cmd := m.handleEnter(); cmd != nil {
				return m, cmd
			}
		case " ":
			// Start buffering for potential key sequence with timeout
			m.pendingKeys = " "
			return m, keySequenceTimeoutCmd()
		case "s":
			m.player.Stop()
			// Resize navigator when player stops
			if m.viewMode == ViewFileBrowser {
				m.fileNavigator, _ = m.fileNavigator.Update(tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.navigatorHeight(),
				})
			} else {
				m.libraryNavigator, _ = m.libraryNavigator.Update(tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.navigatorHeight(),
				})
			}
		case "shift+left":
			m.player.Seek(-5 * time.Second)
		case "shift+right":
			m.player.Seek(5 * time.Second)
		}

	case tickMsg:
		if m.player.State() == player.Playing {
			return m, tickCmd()
		}
	}

	// Route message to active navigator
	var cmd tea.Cmd
	if m.viewMode == ViewFileBrowser {
		m.fileNavigator, cmd = m.fileNavigator.Update(msg)
	} else {
		m.libraryNavigator, cmd = m.libraryNavigator.Update(msg)
	}
	return m, cmd
}

func (m model) waitForScan() tea.Cmd {
	ch := m.scanChan
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		result, ok := <-ch
		if !ok {
			return scanResultMsg{Done: true}
		}
		return scanResultMsg(result)
	}
}

func (m model) waitForLibraryScan() tea.Cmd {
	ch := m.libraryScanCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return libraryScanCompleteMsg{}
		}
		return libraryScanProgressMsg(progress)
	}
}

// isValidSequencePrefix checks if the pending keys could lead to a valid key sequence.
func isValidSequencePrefix(pending string) bool {
	validSequences := []string{" ff", " lr"}
	for _, seq := range validSequences {
		if len(pending) <= len(seq) && seq[:len(pending)] == pending {
			return true
		}
	}
	return false
}

// handleLibrarySearchResult navigates to the selected search result in the library.
func (m *model) handleLibrarySearchResult(result library.SearchResult) {
	switch result.Type {
	case library.ResultArtist:
		// Navigate to artist (focus on artist in root view)
		id := "library:artist:" + result.Artist
		m.libraryNavigator.FocusByID(id)
	case library.ResultAlbum:
		// Navigate to album (focus on album in artist view)
		id := "library:album:" + result.Artist + ":" + result.Album
		m.libraryNavigator.FocusByID(id)
	case library.ResultTrack:
		// Play the track directly
		if result.Path != "" && player.IsMusicFile(result.Path) {
			if err := m.player.Play(result.Path); err != nil {
				m.errorMsg = err.Error()
			}
		}
	}
}

// currentDirSearchItems returns the current directory items as search items.
func (m model) currentDirSearchItems() []search.Item {
	nodes := m.fileNavigator.CurrentItems()
	items := make([]search.Item, len(nodes))
	for i, node := range nodes {
		items[i] = navigator.FileItem{
			Path:    node.ID(),
			RelPath: node.DisplayName(),
			IsDir:   node.IsContainer(),
		}
	}
	return items
}

// currentLibrarySearchItems returns all library items for global search.
func (m model) currentLibrarySearchItems() []search.Item {
	results, err := m.library.AllSearchItems()
	if err != nil {
		return nil
	}
	items := make([]search.Item, len(results))
	for i, r := range results {
		items[i] = library.SearchItem{Result: r}
	}
	return items
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func keySequenceTimeoutCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
		return keySequenceTimeoutMsg{}
	})
}

func (m model) View() string {
	// Render active navigator
	var view string
	if m.viewMode == ViewFileBrowser {
		view = m.fileNavigator.View()
	} else {
		view = m.libraryNavigator.View()
	}

	// Show library scan progress in header area if scanning
	if m.libraryScanMsg != "" {
		// Replace the first line with scan status
		lines := splitLines(view)
		if len(lines) > 0 {
			lines[0] = m.libraryScanMsg
			view = joinLines(lines)
		}
	}

	if m.player.State() != player.Stopped {
		info := m.player.TrackInfo()
		barState := playerbar.State{
			Playing:     m.player.State() == player.Playing,
			Paused:      m.player.State() == player.Paused,
			Track:       info.Track,
			Title:       info.Title,
			Artist:      info.Artist,
			Album:       info.Album,
			Year:        info.Year,
			Position:    m.player.Position(),
			Duration:    m.player.Duration(),
			DisplayMode: m.playerDisplayMode,
		}
		view += playerbar.Render(barState, m.width)
	}

	// Overlay search popup if active
	if m.searchMode {
		searchView := m.search.View()
		view = popup.Compose(view, searchView, m.width, m.height)
	}

	// Overlay error popup if present
	if m.errorMsg != "" {
		errorView := m.renderError()
		view = popup.Compose(view, errorView, m.width, m.height)
	}

	return view
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(lines[0])
	for i := 1; i < len(lines); i++ {
		sb.WriteByte('\n')
		sb.WriteString(lines[i])
	}
	return sb.String()
}

func (m model) renderError() string {
	p := popup.New()
	p.Title = "Error"
	p.Content = m.errorMsg
	p.Footer = "Press any key to dismiss"
	return p.Render(m.width, m.height)
}

func main() {
	m, err := initialModel()
	if err != nil {
		fmt.Printf("Error initializing: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
