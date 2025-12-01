package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/state"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	icons.Init(cfg.Icons)

	stateMgr, err := state.Open()
	if err != nil {
		fmt.Printf("Error opening state: %v\n", err)
		os.Exit(1)
	}

	m, err := app.New(cfg, stateMgr)
	if err != nil {
		stateMgr.Close()
		fmt.Printf("Error initializing: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
