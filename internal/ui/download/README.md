# Download Popup

Orchestrates album downloads from Soulseek via MusicBrainz metadata.

## Flow Overview

Two entry points share one model:

- `f d` starts at phase 1 (artist search).
- `f n` starts at **phase 0**, the new releases list (`internal/releases` cache): `enter`
  there takes the row's artist name and loads the releases of its release group, jumping
  straight to `StateReleaseLoading` and skipping phases 1 and 2. The header shows the row
  until the release details arrive. `fromReleases` then sends `backspace` back to the
  list, and queueing returns to it instead of closing the popup.

On both routes, the release group of the download is the one MusicBrainz returns with the
release details (`ReleaseDetails.ReleaseGroup`): it replaces the selected release group
when the details arrive, and release details without one stop on the release list with
an error.

Each request goes through a `request.Pending` (`internal/ui/request`): one MusicBrainz
request at a time (`waiting`), the read of the new releases (`relRead`) and the queueing
of the download picked (`queueing`). The popup takes an answer only when it carries the
ID of the request it waits for, once, and drops any other, successful or failed, such as
the answer to a request made by a popup closed since. Backspace while queueing stops the
wait: the download is queued all the same. A refused queueing is told by the app, from
the message, whether a popup waits for it or not.

The artists come from a MusicBrainz search, its 25 most relevant results. The release
groups of an artist and the releases of a release group are read in full, a page of 100
after the other, up to 2000 (`maxBrowsePages` of the MusicBrainz client): an artist's
catalogue past that bound is cut. A cut list says so above its rows, with how many were
read and how many MusicBrainz has ("2000 of 6235 release groups read from MusicBrainz");
the line stays while the list is shown, whatever the albums-only filter or the
deduplication leave of it, and goes with the list on backspace.

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
│  StateSlskdSearching ─[core.SearchFinished]─► StateSlskdResults │
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
internal/ui/download/
├── README.md               # This file
├── states.go               # State enum and phase helpers
├── model.go                # Model struct and configuration
├── commands.go             # Async command (queueing a download)
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
