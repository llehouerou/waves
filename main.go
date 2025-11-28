package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/player"
)

type tickMsg time.Time

type model struct {
	navigator navigator.Model[navigator.FileNode]
	player    *player.Player
}

func initialModel() (model, error) {
	cfg, err := config.Load()
	if err != nil {
		return model{}, err
	}

	startPath := cfg.DefaultFolder
	if startPath == "" {
		startPath, err = os.Getwd()
		if err != nil {
			return model{}, err
		}
	}

	source, err := navigator.NewFileSource(startPath)
	if err != nil {
		return model{}, err
	}

	nav, err := navigator.New(source)
	if err != nil {
		return model{}, err
	}

	return model{
		navigator: nav,
		player:    player.New(),
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.player.Stop()
			return m, tea.Quit
		case "enter":
			if selected := m.navigator.Selected(); selected != nil {
				if !selected.IsContainer() && player.IsMusicFile(selected.ID()) {
					if err := m.player.Play(selected.ID()); err == nil {
						return m, tickCmd()
					}
				}
			}
		case " ":
			m.player.Toggle()
		case "s":
			m.player.Stop()
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

		view += fmt.Sprintf(
			"\n%s %s - %s [%s/%s]",
			status,
			info.Artist,
			info.Title,
			formatDuration(pos),
			formatDuration(dur),
		)
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
