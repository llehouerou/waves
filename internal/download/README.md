# Download Popup

Orchestrates album downloads from Soulseek via MusicBrainz metadata.

## Flow Overview

Two entry points share one model:

- `f d` starts at phase 1 (artist search).
- `f n` starts at **phase 0**, the new releases list (`internal/releases` cache): `enter`
  there synthesises the MusicBrainz context from the cached row and jumps straight to
  `StateReleaseLoading`, skipping phases 1 and 2. `fromReleases` then sends `backspace`
  back to the list, and queueing returns to it instead of closing the popup.

1. User searches for an artist (MusicBrainz)
2. User selects release group (album)
3. User selects specific release (for track count)
4. System searches Soulseek for matching directories
5. User selects source to download from

## State Machine

```
┌─────────────────────────────────────────────────────────────────┐
│              PHASE 0: NEW RELEASES LIST (entered with f n)      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  StateReleasesLoading ──[ReleasesLoadedMsg]──► StateReleasesResults
│                                          │ [enter] jumps to     │
│                                          ▼ StateReleaseLoading  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     PHASE 1: ARTIST SEARCH                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  StateSearch ───[enter]───► StateArtistSearching                │
│                                     │                           │
│                            [ArtistSearchResultMsg]              │
│                                     ▼                           │
│                             StateArtistResults                  │
│                                     │                           │
└─────────────────────────────────────┼───────────────────────────┘
                                      │ [enter]
┌─────────────────────────────────────▼───────────────────────────┐
│                  PHASE 2: MUSICBRAINZ SELECTION                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  StateReleaseGroupLoading ────► StateReleaseGroupResults        │
│                                          │                      │
│                                     [enter]                     │
│                                          ▼                      │
│  StateReleaseLoading ─────────► StateReleaseResults             │
│                                          │                      │
│                                     [enter]                     │
│                                          ▼                      │
│                            StateReleaseDetailsLoading           │
│                                          │                      │
└──────────────────────────────────────────┼──────────────────────┘
                                           │ [ReleaseDetailsResultMsg]
┌──────────────────────────────────────────▼──────────────────────┐
│                   PHASE 3: SLSKD SOURCE SELECTION               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  StateSlskdSearching ───[poll]───► StateSlskdResults            │
│                                          │                      │
│                                     [enter]                     │
│                                          ▼                      │
│                                  StateDownloading               │
│                                          │                      │
│                            [SlskdDownloadQueuedMsg]             │
│                                          ▼                      │
│                                     (closes)                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Navigation

- `enter` - Confirm selection and proceed to next step
- `backspace` - Go back to previous step
- `esc` - Close popup
- `↑/↓` or `j/k` - Navigate lists
- In phase 0, `backspace` closes the popup (the list is the first step)

## Filters (Phase 3)

- `f` - Cycle format filter: Both → Lossless → Lossy
- `s` - Toggle no-slot filter (users with free upload slots only)
- `t` - Toggle track count filter (match MB track count)
- `a` - Toggle albums-only filter (Phase 2, release groups)
- `d` - Toggle release deduplication (Phase 2, releases)

## Keys (Phase 0, new releases list)

- `tab` / `h` / `l` / `←` / `→` - Switch Recent ⇄ Upcoming (own cursor per tab)
- `f` - Cycle filter: all → library → discoveries
- `r` - Force a ListenBrainz refresh (ignores the TTL, not the one-in-flight guard)
- `enter` - Jump into the download flow (refuses when slskd is unconfigured)

## File Organization

```
internal/download/
├── README.md               # This file
├── states.go               # State enum and phase helpers
├── model.go                # Model struct and configuration
├── commands.go             # Async commands (MB, slskd API calls)
├── scoring.go              # Result filtering and scoring
│
├── update.go               # Main Update() routing
├── update_releases.go      # New releases list handlers (phase 0)
├── update_search.go        # Search phase handlers
├── update_releasegroup.go  # Release group phase handlers
├── update_release.go       # Release phase handlers
├── update_slskd.go         # Slskd phase handlers
│
├── view.go                 # Main View() dispatch
├── view_helpers.go         # Shared rendering helpers
├── view_releases.go        # New releases list rendering (phase 0)
├── view_search.go          # Search phase rendering
├── view_releasegroup.go    # Release group rendering
├── view_release.go         # Release rendering
└── view_slskd.go           # Slskd results rendering
```
