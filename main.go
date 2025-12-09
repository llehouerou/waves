package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/stderr"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("waves", version)
		os.Exit(0)
	}

	os.Exit(run())
}

func run() int {
	// Capture stderr from C libraries (ALSA, minimp3) to prevent TUI corruption
	_ = stderr.Start()
	defer stderr.Stop()

	cfg, err := config.Load()
	if err != nil {
		stderr.WriteOriginal(fmt.Sprintf("Error loading config: %v\n", err))
		return 1
	}

	icons.Init(cfg.Icons)

	stateMgr, err := state.Open()
	if err != nil {
		stderr.WriteOriginal(fmt.Sprintf("Error opening state: %v\n", err))
		return 1
	}

	m, err := app.New(cfg, stateMgr)
	if err != nil {
		stateMgr.Close()
		stderr.WriteOriginal(fmt.Sprintf("Error initializing: %v\n", err))
		return 1
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		stderr.WriteOriginal(fmt.Sprintf("Error running program: %v\n", err))
		return 1
	}

	return 0
}
