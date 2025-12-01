# Waves

> **Note:** This project is in early development. Features may change and bugs are expected.

A terminal music player with library browsing and queue management.

## Features

- **Library Browser**: Browse music by Artist > Album > Track
- **File Browser**: Navigate filesystem to find music files
- **Playing Queue**: Persistent queue with multi-selection and reordering
- **Audio Playback**: MP3 and FLAC support with seeking
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

### Navigation

| Key | Action |
|-----|--------|
| `h` `j` `k` `l` / Arrows | Navigate |
| `F1` | Library view |
| `F2` | File browser view |
| `Tab` | Switch focus (navigator / queue) |
| `p` | Toggle queue panel |
| `/` | Search current items |
| `Space` `f` `f` | Deep search (file browser) |
| `Space` `l` `r` | Refresh library |
| `q` | Quit |

### Playback

| Key | Action |
|-----|--------|
| `Enter` | Add to queue and play |
| `a` | Add to queue (keep playing) |
| `r` | Replace queue and play |
| `Alt+Enter` | Play album from selected track |
| `Space` | Play/pause |
| `s` | Stop |
| `v` | Toggle player display mode |
| `Shift+Left/Right` | Seek -/+5 seconds |
| `PgDown` / `PgUp` | Next/previous track |
| `Home` / `End` | First/last track |

### Queue Panel

| Key | Action |
|-----|--------|
| `x` | Toggle selection |
| `Shift+J/K` | Move selected items |
| `d` | Delete selected |
| `Enter` | Jump to track |
| `Esc` | Clear selection |

## Configuration

Copy `config.example.toml` to `~/.config/waves/config.toml` or `./config.toml`.

```toml
# Library sources to scan
library_sources = ["~/Music"]

# Icon style: "nerd" (requires Nerd Font) or "ascii"
icons = "nerd"
```

## License

GPL-3.0 - See [LICENSE](LICENSE) for details.
