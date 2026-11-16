package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/diag"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/stderr"
	"github.com/llehouerou/waves/internal/ui/styles"
	"github.com/llehouerou/waves/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("waves", version.Version)
		os.Exit(0)
	}

	os.Exit(run())
}

func run() int {
	// shkh/lastfm-go builds a &http.Client{} with no timeout per request and
	// offers no injection point, so bound the default transport instead.
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.ResponseHeaderTimeout = 15 * time.Second
	}

	// Capture stderr from C libraries (ALSA, minimp3) to prevent TUI corruption
	_ = stderr.Start()
	defer stderr.Stop()

	// Opt-in profiling for issue #28 diagnostics. WAVES_PPROF=host:port
	// (e.g. localhost:6060) starts net/http/pprof; unset => nothing listens.
	// MaybeStartPprof is a no-op for an empty address by contract.
	if ok, actual, perr := diag.MaybeStartPprof(os.Getenv("WAVES_PPROF")); perr != nil {
		stderr.WriteOriginal(fmt.Sprintf("WAVES_PPROF: failed to start pprof: %v\n", perr))
	} else if ok {
		stderr.WriteOriginal(fmt.Sprintf("pprof listening on http://%s/debug/pprof/\n", actual))
	}

	cfg, err := config.Load()
	if err != nil {
		stderr.WriteOriginal(fmt.Sprintf("Error loading config: %v\n", err))
		return 1
	}

	if err := styles.InitTheme(cfg.Theme); err != nil {
		stderr.WriteOriginal(fmt.Sprintf("Error loading theme: %v\n", err))
		return 1
	}

	icons.Init(cfg.Icons)

	m, err := app.New(cfg)
	if err != nil {
		stderr.WriteOriginal(fmt.Sprintf("Error initializing: %v\n", err))
		return 1
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Run closes the core however the program ends, not only on the quit key.
	if err := app.Run(p, m.Core); err != nil {
		stderr.WriteOriginal(runErrorLine(err))
		return 1
	}

	return 0
}

// runErrorLine is what stderr says when Run fails: a failed initialization of
// the TUI is told the way a failed opening of the database is, on a line of
// its own, and each other error of Run on the next lines.
func runErrorLine(err error) string {
	errs := []error{err}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		errs = joined.Unwrap()
	}
	var b strings.Builder
	for _, e := range errs {
		var startupErr *app.StartupError
		if errors.As(e, &startupErr) {
			fmt.Fprintf(&b, "Error initializing: %v\n", startupErr.Err)
		} else {
			fmt.Fprintf(&b, "Error: %v\n", e)
		}
	}
	return b.String()
}
