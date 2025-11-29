package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/overlay"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

type tickMsg time.Time

type scanResultMsg navigator.ScanResult

type keySequenceTimeoutMsg struct{}

type model struct {
	navigator   navigator.Model[navigator.FileNode]
	player      *player.Player
	stateMgr    *state.Manager
	search      search.Model
	searchMode  bool
	scanChan    <-chan navigator.ScanResult
	cancelScan  context.CancelFunc
	pendingKeys string // buffered keys for sequences like "space ff"
	width       int
	height      int
}

func initialModel() (model, error) {
	cfg, err := config.Load()
	if err != nil {
		return model{}, err
	}

	// Open state manager
	stateMgr, err := state.Open()
	if err != nil {
		return model{}, err
	}

	// Determine start path: saved state > config default > cwd
	startPath := cfg.DefaultFolder
	var savedSelection string

	if navState, err := stateMgr.GetNavigation(); err == nil && navState != nil {
		// Check if saved path still exists
		if _, statErr := os.Stat(navState.CurrentPath); statErr == nil {
			startPath = navState.CurrentPath
			savedSelection = navState.SelectedName
		}
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

	nav, err := navigator.New(source)
	if err != nil {
		stateMgr.Close()
		return model{}, err
	}

	// Restore selection if we have one
	if savedSelection != "" {
		nav.FocusByName(savedSelection)
	}

	return model{
		navigator: nav,
		player:    player.New(),
		stateMgr:  stateMgr,
		search:    search.New(),
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) navigatorHeight() int {
	height := m.height
	if m.player.State() != player.Stopped {
		// Navigator outputs height-2 visual lines, so compensate
		height -= playerbar.Height - 2
	}
	return height
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update search model size
		m.search, _ = m.search.Update(msg)

		msg.Height = m.navigatorHeight()

	case navigator.NavigationChangedMsg:
		m.stateMgr.SaveNavigation(state.NavigationState{
			CurrentPath:  msg.CurrentPath,
			SelectedName: msg.SelectedName,
		})
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
			if fileItem, ok := msg.Item.(navigator.FileItem); ok {
				m.navigator.NavigateTo(fileItem.Path)
			}
		}
		m.search.Reset()
		return m, nil

	case keySequenceTimeoutMsg:
		// Timeout occurred, execute buffered space action
		if m.pendingKeys == " " {
			m.pendingKeys = ""
			m.player.Toggle()
		}
		return m, nil

	case tea.KeyMsg:
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
			if m.pendingKeys == " ff" {
				// Deep search: recursive scan
				m.pendingKeys = ""
				m.searchMode = true
				m.search.SetLoading(true)
				ctx, cancel := context.WithCancel(context.Background())
				m.cancelScan = cancel
				m.scanChan = navigator.ScanDir(ctx, m.navigator.CurrentPath())
				return m, m.waitForScan()
			} else if len(m.pendingKeys) >= 3 || !isValidSequencePrefix(m.pendingKeys) {
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
		case "/":
			// Shallow search: current directory items only
			m.searchMode = true
			items := m.currentDirSearchItems()
			m.search.SetItems(items)
			m.search.SetLoading(false)
			return m, nil
		case "enter":
			if selected := m.navigator.Selected(); selected != nil {
				if !selected.IsContainer() && player.IsMusicFile(selected.ID()) {
					if err := m.player.Play(selected.ID()); err == nil {
						// Resize navigator for player bar
						m.navigator, _ = m.navigator.Update(tea.WindowSizeMsg{
							Width:  m.width,
							Height: m.navigatorHeight(),
						})
						return m, tickCmd()
					}
				}
			}
		case " ":
			// Start buffering for potential key sequence with timeout
			m.pendingKeys = " "
			return m, keySequenceTimeoutCmd()
		case "s":
			m.player.Stop()
			// Resize navigator when player stops
			m.navigator, _ = m.navigator.Update(tea.WindowSizeMsg{
				Width:  m.width,
				Height: m.navigatorHeight(),
			})
		}

	case tickMsg:
		if m.player.State() == player.Playing {
			return m, tickCmd()
		}
	}

	var cmd tea.Cmd
	m.navigator, cmd = m.navigator.Update(msg)
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

// isValidSequencePrefix checks if the pending keys could lead to a valid key sequence.
func isValidSequencePrefix(pending string) bool {
	validSequences := []string{" ff"}
	for _, seq := range validSequences {
		if len(pending) <= len(seq) && seq[:len(pending)] == pending {
			return true
		}
	}
	return false
}

// currentDirSearchItems returns the current directory items as search items.
func (m model) currentDirSearchItems() []search.Item {
	nodes := m.navigator.CurrentItems()
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
	view := m.navigator.View()

	if m.player.State() != player.Stopped {
		info := m.player.TrackInfo()
		barState := playerbar.State{
			Playing:  m.player.State() == player.Playing,
			Paused:   m.player.State() == player.Paused,
			Track:    info.Track,
			Title:    info.Title,
			Artist:   info.Artist,
			Album:    info.Album,
			Year:     info.Year,
			Position: m.player.Position(),
			Duration: m.player.Duration(),
		}
		view += playerbar.Render(barState, m.width)
	}

	// Overlay search popup if active
	if m.searchMode {
		searchView := m.search.View()
		view = overlay.Compose(view, searchView, m.width, m.height)
	}

	return view
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
