---
status: accepted
---

# An embedded core shared by the TUI and a Gio desktop GUI

waves gets a desktop GUI next to the TUI. All domain state and background
logic moves out of the Bubble Tea root model (`internal/app`) into a core: a
Go library that each client binary embeds and drives through commands,
queries and events. The GUI is written in Go on Gio. There is no daemon: one
client runs at a time, and the music stops when it closes, as it does today.

## Considered Options

- **A core daemon with clients over a protocol (the herdr / mpd model).** The
  only gain left was allowing a GUI in another language: music outliving its
  window is more confusing than useful in a music player. It costs a
  versioned protocol, a daemon lifecycle, and every library read going
  through the wire. Rejected.
- **GPUI (Zed's Rust framework).** The fluidity and look we wanted, but it
  forces Rust, so the daemon above. Unusable from Go: no C API.
- **Gio.** Same approach as GPUI in Go (GPU rendering, immediate mode),
  mature, two funded maintainers. A throwaway prototype — one Zed-styled
  screen over the real 50k-track library, branch `proto/gio-feel` — was
  judged a good base. Chosen.
- **gogpu/ui** (one year old, a single main author), **Cogent Core** (an
  imposed Material look), **Fyne**, **a webview (Wails)**, **a framework of
  our own** (years of platform work: Wayland, IME, accessibility). Rejected.

## Consequences

- Background logic driven today by the TUI's tick (the scrobble threshold,
  radio fill, the downloads poll, scan progress) moves into the core: the
  core must stay correct with no client ticking it.
- The core's API exposes data only, never internal handles: no
  `playback.Service.Player()`, no `state.Interface.DB()`. Its events and
  results stay plain data, so a daemon remains possible if the GUI ever
  needs another language.
- Navigation, popups, layout, keymap and cover rendering stay in each client.
- Gio draws glyphs as vector paths: no hinting, grayscale antialiasing, no
  text gamma. Small text at scale 1.0 is softer than in GPUI; if it matters,
  the fix is a contribution to Gio (a glyph atlas and text gamma).
