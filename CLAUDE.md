# Waves - Terminal Music Player

## Dev Workflow

```bash
make fmt           # Format code (goimports-reviser)
make lint          # Run golangci-lint
make check         # Format + lint
make build         # Verify compilation (no binary output)
make run           # Run with go run
make install-hooks # Install git pre-commit hook
```

Run `make install-hooks` after cloning. Pre-commit hook runs `make check` before each commit.

## Git Workflow

Always wait for user confirmation before committing or pushing changes.

## Current State

- Basic Bubble Tea app with Miller columns file navigator
- 3 columns: parent (1/6) | current (1/6) | preview (2/3)
- Navigation: arrows or hjkl
- Header shows current path with separator

## Goal

Terminal music player with file browser for music selection.
