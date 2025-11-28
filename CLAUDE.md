# Waves - Terminal Music Player

## Dev Workflow

```bash
make fmt      # Format code (goimports-reviser)
make lint     # Run golangci-lint
make build    # Verify compilation (no binary output)
make run      # Run with go run
```

Git pre-commit hook automatically runs `make fmt` and `make lint` before each commit.

## Current State

- Basic Bubble Tea app with Miller columns file navigator
- 3 columns: parent (1/6) | current (1/6) | preview (2/3)
- Navigation: arrows or hjkl
- Header shows current path with separator

## Goal

Terminal music player with file browser for music selection.
