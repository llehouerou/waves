# Waves

A terminal music player with a file browser for music selection.

## Current State

- Miller columns file navigator (parent | current | preview)
- Keyboard navigation with arrows or hjkl
- Music playback (MP3, FLAC) with play/pause/stop
- Styled player bar with track info and progress
- Configuration via TOML files

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

| Key | Action |
|-----|--------|
| `h` / `←` | Go to parent directory |
| `l` / `→` / `Enter` | Enter directory / Play file |
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Space` | Toggle play/pause |
| `s` | Stop playback |
| `q` | Quit |

## Configuration

Copy `config.example.toml` to `~/.config/waves/config.toml` or `./config.toml`.

## License

GPL-3.0 - See [LICENSE](LICENSE) for details.
