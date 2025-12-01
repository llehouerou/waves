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

Terminal music player with library browser and queue management.

### Features

- **Library Browser**: Browse music by Artist > Album > Track hierarchy
- **File Browser**: Navigate filesystem to find music files
- **Playing Queue**: Persistent queue with multi-selection and reordering
- **Audio Playback**: MP3 and FLAC support with seeking
- **State Persistence**: Queue and navigation state saved between sessions

### Key Bindings

#### Navigation
- `hjkl` / arrows: Navigate
- `F1`: Library view
- `F2`: File browser view
- `Tab`: Switch focus between navigator and queue panel
- `p`: Toggle queue panel visibility
- `/`: Search current items
- `space ff`: Deep search (file browser)
- `space lr`: Refresh library

#### Playback
- `Enter`: Add to queue and play
- `a`: Add to queue (keep playing)
- `r`: Replace queue and play
- `Alt+Enter`: Replace queue with album, play from selected track
- `Space`: Play/pause (starts queue playback when stopped)
- `s`: Stop playback
- `v`: Toggle player display mode (compact/expanded)
- `Shift+Left/Right`: Seek -/+5 seconds

#### Queue Navigation
- `PgDown/PgUp`: Next/previous track
- `Home/End`: First/last track

#### Queue Panel (when focused)
- `x`: Toggle selection on item
- `Shift+J/K`: Move selected items up/down
- `d`: Delete selected items
- `Enter`: Jump to and play track
- `Esc`: Clear selection

### Architecture

- **Bubble Tea**: TUI framework
- **Beep**: Audio playback
- **SQLite**: State persistence (library index, queue, navigation)
- **Miller columns**: Three-panel navigator layout

### Key Packages

- `internal/navigator`: Generic Miller columns navigator
- `internal/library`: Music library with SQLite storage
- `internal/player`: Audio playback (MP3/FLAC)
- `internal/playlist`: Queue and track management
- `internal/state`: Persistent state (navigation, queue)
- `internal/ui/queuepanel`: Queue display with selection
- `internal/ui/playerbar`: Playback status display
