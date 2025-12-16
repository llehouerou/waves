<p align="center">
  <img src="assets/screenshot.png" alt="Waves" width="800">
</p>

> **Note:** This project is in early development. Features may change and bugs are expected.

A terminal music player with library browsing and queue management.

## Features

- **Library Browser**: Browse music by Artist > Album > Track hierarchy
- **File Browser**: Navigate filesystem with file/folder deletion
- **Playlists**: Create, organize, and manage playlists with folder hierarchy
- **Favorites**: Quick-access playlist with heart icon display
- **Playing Queue**: Persistent queue with multi-selection, reordering, and undo/redo
- **Audio Playback**: MP3 and FLAC support with seeking
- **Full-Text Search**: SQLite FTS5 search across library, files, and playlists
- **Download Manager**: Search and download from Soulseek via slskd integration
- **Import System**: MusicBrainz tagging, file renaming, and library integration
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
| `g` `d` | Download manager (requires slskd) |
| `?` | Show keybinding help |
| `q` | Quit |

### Playback

| Key | Action |
|-----|--------|
| `Enter` | Play (replaces queue with folder/album, plays selected) |
| `a` | Add to queue (keep playing) |
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
| `Ctrl+Z` | Undo |
| `Ctrl+Shift+Z` | Redo |

### File Browser (F2 view)

| Key | Action |
|-----|--------|
| `Ctrl+D` | Delete file/folder |

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

# slskd integration for downloading music from Soulseek
[slskd]
url = "http://localhost:5030"
apikey = "your-api-key-here"

[slskd.filters]
format = "both"       # "both", "lossless", or "lossy"
no_slot = true        # Filter out users with no free upload slot
track_count = true    # Filter results with fewer tracks than expected

[musicbrainz]
albums_only = true    # Filter to show only albums (not singles/EPs)
```

### Library Sources

Library sources are managed in-app using `g` `p` in the library view (F1). This opens a popup where you can add, remove, and view source paths. Sources are persisted in the database, not the config file.

### Download Manager

The download manager requires a running [slskd](https://github.com/slskd/slskd) instance. Configure the URL and API key in `config.toml`, then use `g` `d` to open the download popup. Search for artists/albums, select a release from MusicBrainz, and download matching results from Soulseek. Downloaded files can be imported with MusicBrainz tagging and Picard-compatible file renaming.

## License

GPL-3.0 - See [LICENSE](LICENSE) for details.
