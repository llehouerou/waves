package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/navigator"
)

type model struct {
	navigator navigator.Model[navigator.FileNode]
}

func initialModel() (model, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return model{}, err
	}

	source, err := navigator.NewFileSource(cwd)
	if err != nil {
		return model{}, err
	}

	nav, err := navigator.New(source)
	if err != nil {
		return model{}, err
	}

	return model{navigator: nav}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.navigator, cmd = m.navigator.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.navigator.View()
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
