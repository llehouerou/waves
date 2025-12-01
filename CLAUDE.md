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
- `D`: Keep only selected items
- `Enter`: Jump to and play track
- `Esc`: Clear selection

### Architecture

- **Bubble Tea**: TUI framework
- **Beep**: Audio playback
- **SQLite**: State persistence (library index, queue, navigation)
- **Miller columns**: Three-panel navigator layout

### Key Packages

- `internal/app`: Root model, update logic, view composition
- `internal/navigator`: Generic Miller columns navigator
- `internal/library`: Music library with SQLite storage
- `internal/player`: Audio playback (MP3/FLAC)
- `internal/playlist`: Queue and track management
- `internal/state`: Persistent state (navigation, queue)
- `internal/ui/queuepanel`: Queue display with selection
- `internal/ui/playerbar`: Playback status display

## Architecture Principles

See `docs/ARCHITECTURE.md` for detailed patterns. Key rules:

### Elm Architecture (MVU)

- **All state changes flow through `Update()`** - Never mutate state elsewhere
- **`View()` is pure** - Only renders, never modifies state
- **Commands handle side effects** - Async operations return messages
- **Never block in Update() or View()** - Use commands for I/O

### File Organization (`internal/app/`)

| File | Responsibility |
|------|----------------|
| `app.go` | Model struct, `New()` constructor, `Init()` |
| `update.go` | `Update()` method, message routing, key handlers |
| `view.go` | `View()` method, rendering helpers |
| `commands.go` | Command factories (tick, timeouts) |
| `messages.go` | Message type definitions, enums |
| `layout.go` | Dimension calculations |
| `persistence.go` | State save methods |
| `playback.go` | Playback control methods |
| `queue.go` | Queue action methods |
| `components.go` | Component resize/focus methods |

### Message Routing Strategies

| Strategy | When | Example |
|----------|------|---------|
| **INTERCEPT** | Root handles completely | `q`, `ctrl+c`, `tab` |
| **BROADCAST** | All children need it | `tea.WindowSizeMsg` |
| **DELEGATE** | Only focused child handles | `hjkl` keys |
| **TARGET** | Route by message type | `NavigationChangedMsg` |

### State Ownership

- **Domain state** (player, queue, library): Owned by root, accessed via pointers
- **UI state** (cursor, selection): Owned by each component
- **Golden rule**: Components can READ shared state directly but MUST emit messages to WRITE

### Package Dependencies

```
main.go → internal/app → internal/ui/* + domain packages
internal/ui/* → internal/playlist (Track type) + ui utilities
domain packages (player, playlist, library, state) → NO ui imports
```

### Stateless Views

Use pure render functions (not `tea.Model`) when a component has no local state:
- `playerbar.Render(state, width)` - stateless, all data passed in
- Easier to test, no boilerplate Init/Update

### Anti-Patterns

- ❌ Mutating state in `View()` or commands
- ❌ Blocking I/O in `Update()` or `View()`
- ❌ Monolithic 500+ line switch statements
- ❌ Components directly mutating shared state without emitting messages
