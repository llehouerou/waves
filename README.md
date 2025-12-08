# Waves

<p align="center">
  <img src="assets/screenshot.png" alt="Waves" width="800">
</p>

> **Note:** This project is in early development. Features may change and bugs are expected.

A terminal music player with library browsing and queue management.

## Features

- **Library Browser**: Browse music by Artist > Album > Track hierarchy
- **File Browser**: Navigate filesystem to find music files
- **Playlists**: Create, organize, and manage playlists with folder hierarchy
- **Playing Queue**: Persistent queue with multi-selection and reordering
- **Audio Playback**: MP3 and FLAC support with seeking
- **Trigram Search**: Fast fuzzy search across library, files, and playlists
- **Mouse Support**: Click to navigate, select tracks, and control playback
- **State Persistence**: Queue and navigation saved between sessions

## Installation

```bash
go install github.com/llehouerou/waves@latest
```

## Development

```bash
git clone https://github.com/llehouerou/waves.git
cd waves
make install-hooks  # Install git pre-commit hook
make run            # Run the app
```

## Controls

Press `?` at any time to show the keybinding help popup.

### Navigation

| Key | Action |
|-----|--------|
| `h` `j` `k` `l` / Arrows | Navigate |
| `F1` | Library view |
| `F2` | File browser view |
| `F3` | Playlists view |
| `Tab` | Switch focus (navigator / queue) |
| `p` | Toggle queue panel |
| `/` | Search current items |
| `g` `f` | Deep search (library, file browser, or playlists) |
| `g` `r` | Refresh library (incremental) |
| `g` `R` | Full rescan library (re-reads all metadata) |
| `g` `p` | Library sources manager (library view) |
| `?` | Show keybinding help |
| `q` | Quit |

### Playback

| Key | Action |
|-----|--------|
| `Enter` | Add to queue and play |
| `a` | Add to queue (keep playing) |
| `r` | Replace queue and play |
| `Alt+Enter` | Play album from selected track |
| `Ctrl+A` | Add to playlist (library view) |
| `d` | Delete track (library view, track level) |
| `Space` | Play/pause |
| `s` | Stop |
| `v` | Toggle player display mode |
| `Shift+Left/Right` | Seek -/+5 seconds |
| `Alt+Shift+Left/Right` | Seek -/+15 seconds |
| `PgDown` / `PgUp` | Next/previous track |
| `Home` / `End` | First/last track |

### Queue Panel

| Key | Action |
|-----|--------|
| `x` | Toggle selection |
| `Shift+J/K` | Move selected items |
| `d` | Delete selected |
| `D` | Keep only selected |
| `c` | Clear queue except playing track |
| `Enter` | Jump to track |
| `Esc` | Clear selection |

### Playlists (F3 view)

| Key | Action |
|-----|--------|
| `n` | Create new playlist |
| `N` | Create new folder |
| `Ctrl+R` | Rename playlist/folder |
| `Ctrl+D` | Delete playlist/folder |
| `d` | Remove track from playlist |
| `J` / `K` | Move track down/up |

## Configuration

Copy `config.example.toml` to `~/.config/waves/config.toml` or `./config.toml`.

```toml
# Default folder to open on startup (supports ~)
default_folder = "~/Music"

# Icon style: "nerd" (Nerd Font), "unicode" (emoji), "none" (text)
icons = "nerd"
```

### Library Sources

Library sources are managed in-app using `g` `p` in the library view (F1). This opens a popup where you can add, remove, and view source paths. Sources are persisted in the database, not the config file.

## License

GPL-3.0 - See [LICENSE](LICENSE) for details.
