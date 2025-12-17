# Download Popup

Orchestrates album downloads from Soulseek via MusicBrainz metadata.

## Flow Overview

1. User searches for an artist (MusicBrainz)
2. User selects release group (album)
3. User selects specific release (for track count)
4. System searches Soulseek for matching directories
5. User selects source to download from

## State Machine

```
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

## Filters (Phase 3)

- `f` - Cycle format filter: Both → Lossless → Lossy
- `s` - Toggle no-slot filter (users with free upload slots only)
- `t` - Toggle track count filter (match MB track count)
- `a` - Toggle albums-only filter (Phase 2, release groups)
- `d` - Toggle release deduplication (Phase 2, releases)

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
├── update_search.go        # Search phase handlers
├── update_releasegroup.go  # Release group phase handlers
├── update_release.go       # Release phase handlers
├── update_slskd.go         # Slskd phase handlers
│
├── view.go                 # Main View() dispatch
├── view_helpers.go         # Shared rendering helpers
├── view_search.go          # Search phase rendering
├── view_releasegroup.go    # Release group rendering
├── view_release.go         # Release rendering
└── view_slskd.go           # Slskd results rendering
```
