package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/state"
)

var playerBarStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("240"))

type tickMsg time.Time

type model struct {
	navigator navigator.Model[navigator.FileNode]
	player    *player.Player
	stateMgr  *state.Manager
	width     int
	height    int
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
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

const playerBarHeight = 3 // top border + content + bottom border

func (m model) navigatorHeight() int {
	height := m.height
	if m.player.State() != player.Stopped {
		// Navigator outputs height-2 visual lines, so compensate
		height -= playerBarHeight - 2
	}
	return height
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		msg.Height = m.navigatorHeight()

	case navigator.NavigationChangedMsg:
		m.stateMgr.SaveNavigation(state.NavigationState{
			CurrentPath:  msg.CurrentPath,
			SelectedName: msg.SelectedName,
		})
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.player.Stop()
			m.stateMgr.Close()
			return m, tea.Quit
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
			m.player.Toggle()
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

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) View() string {
	view := m.navigator.View()

	if m.player.State() != player.Stopped {
		info := m.player.TrackInfo()
		pos := m.player.Position()
		dur := m.player.Duration()

		status := "▶"
		if m.player.State() == player.Paused {
			status = "⏸"
		}

		// Right side: position/duration
		right := fmt.Sprintf("%s / %s ", formatDuration(pos), formatDuration(dur))
		rightLen := lipgloss.Width(right)

		// Calculate available width (subtract border width of 2)
		innerWidth := m.width - 2
		if innerWidth < 0 {
			innerWidth = 0
		}

		// Build track info (always shown)
		trackInfo := info.Title
		if info.Track > 0 {
			trackInfo = fmt.Sprintf("%02d - %s", info.Track, info.Title)
		}

		// Build album info
		albumInfo := info.Album
		if albumInfo != "" && info.Year > 0 {
			albumInfo = fmt.Sprintf("%s (%d)", albumInfo, info.Year)
		}

		artistInfo := info.Artist

		// Build combined artist/album: "Artist - Album (Year)"
		var artistAlbumFull, artistOnly string
		if artistInfo != "" {
			artistOnly = artistInfo
			if albumInfo != "" {
				artistAlbumFull = fmt.Sprintf("%s - %s", artistInfo, albumInfo)
			} else {
				artistAlbumFull = artistInfo
			}
		}

		// Calculate minimum width needed: " ▶  trackInfo  right"
		minGap := 2 // minimum gap between sections
		statusPart := " " + status + "  "
		statusLen := lipgloss.Width(statusPart)

		availableForContent := innerWidth - statusLen - rightLen - minGap
		trackLen := lipgloss.Width(trackInfo)
		artistAlbumFullLen := lipgloss.Width(artistAlbumFull)
		artistOnlyLen := lipgloss.Width(artistOnly)

		// Determine what fits: priority is track > artist > artist+album
		var artistPart string
		if artistAlbumFull != "" && artistAlbumFullLen+minGap+trackLen <= availableForContent {
			artistPart = artistAlbumFull
		} else if artistOnly != "" && artistOnlyLen+minGap+trackLen <= availableForContent {
			artistPart = artistOnly
		}

		// Build left content
		var leftParts []string
		if artistPart != "" {
			leftParts = append(leftParts, artistPart)
		}
		leftParts = append(leftParts, trackInfo)

		// Calculate total content width and distribute extra space
		contentWidth := 0
		for _, p := range leftParts {
			contentWidth += lipgloss.Width(p)
		}
		gaps := len(leftParts) // gaps between parts + gap before right

		extraSpace := availableForContent - contentWidth
		if extraSpace < 0 {
			extraSpace = 0
		}
		gapSize := minGap
		if gaps > 0 && extraSpace > 0 {
			gapSize = (extraSpace / gaps) + minGap
		}

		left := statusPart + strings.Join(leftParts, strings.Repeat(" ", gapSize))
		leftLen := lipgloss.Width(left)

		// Final padding to right-align the timer
		padding := innerWidth - leftLen - rightLen
		if padding < 0 {
			padding = 0
		}

		content := left + strings.Repeat(" ", padding) + right
		playerBar := playerBarStyle.Width(innerWidth).Render(content)

		view += playerBar
	}

	return view
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
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
