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

Run `make install-hooks` after cloning. Pre-commit runs `make check` before each commit.

## Git Workflow

Always wait for user confirmation before committing or pushing changes.

## Architecture

### Stack

- **Bubble Tea**: TUI framework (Elm architecture)
- **Beep**: Audio playback (MP3/FLAC)
- **SQLite**: Persistence (library index, queue, playlists, FTS5 search)
- **Miller columns**: Three-panel navigator layout

### Package Structure

```
internal/
├── app/           # Root model, update, view, managers, controllers
├── navigator/     # Generic Miller columns with sourceutil helpers
├── library/       # Music library with SQLite storage
├── playlists/     # Playlist management with folders
├── player/        # Audio playback engine
├── playlist/      # Queue and track management
├── search/        # SQLite FTS5 search
├── download/      # Download orchestration (slskd + MusicBrainz)
├── downloads/     # Download state tracking
├── importer/      # File import with tagging and renaming
├── slskd/         # Soulseek client API
├── musicbrainz/   # MusicBrainz API client
├── rename/        # Picard-compatible file renaming
├── state/         # Persistent navigation state
├── config/        # Configuration loading
├── db/            # Database utilities
├── icons/         # Icon rendering (nerd/unicode/none)
├── keymap/        # Key binding definitions
├── stderr/        # C library stderr capture
└── ui/            # UI components
    ├── queuepanel/     # Queue display with selection
    ├── playerbar/      # Playback status
    ├── headerbar/      # Navigation breadcrumbs
    ├── downloads/      # Download progress UI
    ├── popup/          # Generic popup container
    ├── confirm/        # Confirmation dialogs
    ├── textinput/      # Text input popup
    ├── helpbindings/   # Keybinding help
    ├── librarysources/ # Library source manager
    ├── scanreport/     # Scan results display
    ├── jobbar/         # Background job status
    ├── styles/         # Shared lipgloss styles
    └── render/         # Rendering utilities
```

### Key Patterns

**Elm Architecture (MVU)**
- All state changes flow through `Update()` - never mutate elsewhere
- `View()` is pure - only renders, never modifies state
- Commands handle side effects - async operations return messages

**Message Routing**
- INTERCEPT: Root handles completely (`q`, `ctrl+c`, `tab`)
- BROADCAST: All children need it (`tea.WindowSizeMsg`)
- DELEGATE: Only focused child handles (`hjkl` navigation)
- TARGET: Route by message type (`NavigationChangedMsg`)

**State Ownership**
- Domain state (player, queue, library): Owned by root, accessed via pointers
- UI state (cursor, selection): Owned by each component
- Components can READ shared state but MUST emit messages to WRITE

**Dependencies**
```
main.go → internal/app → internal/ui/* + domain packages
domain packages → NO ui imports
```

**Command Pattern**
Commands are async functions that return `tea.Cmd`. Follow these conventions:
- **Naming**: Use `xxxCmd` suffix (e.g., `searchArtistsCmd`, `RefreshLibraryCmd`)
- **Parameters**: Use a params struct for 3+ parameters (e.g., `slskdPollParams`, `ImportFileParams`)
- **Results**: All result messages use `Err error` field (not `Error`)
- **Structure**:
  ```go
  // For complex commands, define a params struct
  type fooParams struct {
      client *Client
      id     string
      // ...
  }

  // Command function returns tea.Cmd
  func fooCmd(params fooParams) tea.Cmd {
      return func() tea.Msg {
          result, err := params.client.DoSomething(params.id)
          return FooResultMsg{Result: result, Err: err}
      }
  }

  // Result message with Err field
  type FooResultMsg struct {
      Result SomeType
      Err    error
  }
  ```

**Error Handling**
- All result messages use `Err error` field (not `Error`)
- Never swallow errors silently - either:
  1. Propagate to caller via return value or message
  2. Display to user via `Popups.ShowError(errmsg.Format(op, err))`
  3. Log with context (for non-critical background operations)
- Use `internal/errmsg` package for consistent formatting:
  ```go
  import "github.com/llehouerou/waves/internal/errmsg"

  // Simple error
  m.Popups.ShowError(errmsg.Format(errmsg.OpDownloadDelete, err))

  // Error with context
  m.Popups.ShowError(errmsg.FormatWith(errmsg.OpFileDelete, filename, err))
  ```
- Intentionally ignored errors must use `//nolint:nilerr` or `//nolint:errcheck` with comment

### Anti-Patterns

- Mutating state in `View()` or commands
- Blocking I/O in `Update()` or `View()`
- Components directly mutating shared state without emitting messages
