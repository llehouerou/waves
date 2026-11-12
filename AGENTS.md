# Waves - Terminal Music Player

## Dev Workflow

```bash
make fmt           # Format code (goimports-reviser)
make lint          # Run golangci-lint
make check         # Verify format (no rewrite) + lint + test
make build         # Verify compilation (no binary output)
make run           # Run with go run
make install-hooks # Install git pre-commit hook
```

Run `make install-hooks` after cloning. Pre-commit runs `make fmt check` before each commit.

## Git Workflow

Always wait for user confirmation before committing or pushing changes.
Autonomous agents (no human at the keyboard) follow "Autonomous Runs" below instead.

## Conventions

- English everywhere: code, comments, docs, commit messages, issues, PR titles.
- Commits: Conventional Commits, `type(scope): summary`, lowercase, the summary
  states the resulting behaviour (`fix(import): never overwrite a track already in the library`).
  Types in use: `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `build`, `ci`, `chore`.
- Branches: `type/issue-N-slug` (`refactor/issue-72-download-lifecycle`), `type/slug` without an issue.

## Autonomous Runs

- Commit on your own branch without asking; never push, tag or release.
- Do not touch `.github/workflows/`, `.goreleaser.yaml`, `aur/` or `flake.lock`.
- A fresh clone has no pre-commit hook: run `make check` yourself before finishing,
  and never bypass it (`--no-verify`, bare `//nolint`).
- If `go.mod`/`go.sum` changed, run `go mod tidy` then `make update-vendor-hash`
  (what the hook does), and commit `default.nix` with them.
- Never launch `waves` / `make run` to check behaviour: it needs a TTY and an
  audio device. Verify through tests (next section).

## Testing Without a Terminal

Bubble Tea models are plain values, so the TUI is tested without a terminal:

- Build the model, feed `tea.KeyMsg` or domain messages to `Update()`, then assert
  on the returned model, on the returned `tea.Cmd` (call it to get its message),
  and on `View()` passed through `testutil.StripANSI`.
  Root model: `internal/app/integration_test.go` (`newIntegrationTestModel`, `keyMsg`).
  Popups: `testutil.PopupHarness` in `internal/ui/testutil`.
- No teatest, no golden files: assert on lines or substrings of the rendered view
  (`testutil.ContainsLine`, `testutil.FindLine`).
- Audio: build the core on `player.NewMock()` (`core.Options.Player`; `newTestCore`
  and `testPlayer` in `internal/app/app_test.go`); never open the real speaker (`speaker.Init`).
- Library, playlists and downloads: `newTestCore` runs on `state.NewMock()`, which has no
  database, so its core has none of them. A test that reads them builds its model with
  `newLibraryTestModel` (`internal/app/library_test.go`): a real database in `t.TempDir()`,
  seeded before the core opens it. Feed it the core's events with `pumpUntil`.
- Remote services (slskd, MusicBrainz, Last.fm, ListenBrainz): fake them with
  `httptest.NewServer`. slskd has a shared fake, `slskdtest.New` (`internal/slskd/slskdtest`):
  `newDownloadsCore` in `internal/core/downloads_test.go` builds a core on it,
  `newDownloadsTestModel` in `internal/app/downloads_test.go` a model.
  Last.fm's client only yields to a swapped `http.DefaultTransport`: `lastfmtest.Serve`
  (`internal/lastfm/lastfmtest`) does it, so its tests take no `t.Parallel`. A core is
  linked by a session saved in its state before it opens (`linkedState` in
  `internal/core/core_test.go`); a model built by hand opens no browser (`Model.openBrowser`
  is nil), `newLastfmTestModel` in `internal/app/lastfm_test.go` records what it would open.
  MusicBrainz is replaced in the core's own tests only (`Options.musicbrainz`, set by
  `newTestCore`). From another package, import a release with no ID: `PrepareImport` then
  asks MusicBrainz nothing (`internal/app/import_test.go`).
  Tests must pass offline.
- Files: `t.TempDir()` for databases and fixtures. Never read or write the real
  `~/.config/waves` or `~/.local/share/waves`: `t.Setenv("HOME", t.TempDir())`
  redirects `os.UserHomeDir`, but `adrg/xdg` caches its dirs at init, so code
  using it must take the path as a parameter to be tested.
- Audio fixtures: `tagstest.WriteMP3` (`internal/tags/tagstest`) writes a tagged MP3 with
  no ffmpeg, for the tests that import, retag or scan files. A test about a format
  generates its file with ffmpeg and skips when it is missing (`internal/tags/read_test.go`).
  D-Bus tests skip without a session bus.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for `llehouerou/waves`, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical labels, unchanged: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root (created lazily when needed). See `docs/agents/domain.md`.

## Architecture

### Stack

- **Bubble Tea**: TUI framework (Elm architecture)
- **Beep**: Audio playback (MP3/FLAC)
- **SQLite**: Persistence (library index, queue, playlists, FTS5 search)
- **Miller columns**: Three-panel navigator layout

### Package Structure

```
internal/
├── app/           # TUI client: root model, update, view, managers, controllers
├── core/          # Embedded core: domain state and background logic (ADR 0001)
├── navigator/     # Generic Miller columns with sourceutil helpers
├── library/       # Music library with SQLite storage
├── playlists/     # Playlist management with folders
├── player/        # Audio playback engine
├── playlist/      # Queue and track management
├── search/        # SQLite FTS5 search
├── downloads/     # Download lifecycle, Sync with slskd, scoring of search results
├── importer/      # Import of a download and retag of an album: plan, track matching, per-file loop
├── slskd/         # Soulseek client API and its search
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
    ├── download/       # Download popup (MusicBrainz, slskd search), over the core
    ├── importpopup/    # Import popup (plan preview, library source, progress), over the core
    ├── retag/          # Retag popup (release lookup, tag preview, progress), over the core
    ├── mbworkflow/     # MusicBrainz requests as Bubble Tea commands
    ├── popup/          # Generic popup container
    ├── confirm/        # Confirmation dialogs
    ├── textinput/      # Text input popup
    ├── helpbindings/   # Keybinding help
    ├── librarysources/ # Library source manager
    ├── librarynav/     # Library nodes and navigator source, over the core
    ├── playlistnav/    # Playlist nodes and navigator source, over the core
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
- Playback (queue, volume, scrobbling, radio), library (sources, scan, retag, search index),
  playlists (folders, favorites), downloads (Sync, slskd search, import, MusicBrainz) and
  Last.fm (linking, pending scrobbles, similar artists): owned by the core. The TUI sends
  commands to `m.Core.Playback()`, `m.Core.Library()`, `m.Core.Playlists()`,
  `m.Core.Downloads()`, `m.Core.MusicBrainz()` and `m.Core.Lastfm()`, reads their data,
  and reacts to `m.Core.Events()`; `internal/app/architecture_test.go` lists the packages
  only the core may import
- A command the client waits for returns its error and emits nothing: the client refreshes
  its own views. Events report background work only (a scan, a Sync, a slskd search, an
  import, a retag, a radio fill, a failure)
- Other domain state (releases, exports): owned by root, accessed via pointers,
  until its step of issue #88 moves it into the core
- UI state (cursor, selection): Owned by each component
- Components can READ shared state but MUST emit messages to WRITE

**Dependencies**
```
main.go → internal/app → internal/core + internal/ui/* + domain packages
internal/core → domain packages
domain packages the core owns → NO ui, navigator, icons or Bubble Tea imports
```

**Command Pattern**
Commands are async functions that return `tea.Cmd`. Follow these conventions:
- **Naming**: Use `xxxCmd` suffix (e.g., `SearchArtistsCmd`, `LoadReleasesCmd`)
- **Parameters**: Use a params struct for 3+ parameters (e.g., `LoadReleasesParams`, `RetryPendingParams`)
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

**Adding Async Commands to UI Components**
When adding new `tea.Cmd` functions that return messages to popups/components:
1. Define the message type in the component's `commands.go`
2. Handle the message in the component's `Update()`
3. **Register the message in `internal/app/update.go`** pass-through cases so it routes to the component

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
