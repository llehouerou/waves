# CONTEXT

Domain vocabulary for waves. Use these words in code, comments, commits and
issues; reach for one of them before inventing a new one.

## Architecture

**Core** — the Go library that owns all domain state and background logic
(playback, queue, library, playlists, downloads and import, scrobbling, radio,
exports, lyrics, MPRIS, desktop notifications) and is driven only through
commands, queries and events carrying plain data. Each client embeds it; see
`docs/adr/0001-embedded-core-and-gio-gui.md`.

**Client** — a user interface over the core: the Bubble Tea TUI or the Gio
GUI. It owns only presentation state (navigation, popups, layout, keymap) and
never touches a domain package directly. One client runs at a time.
_Avoid_: frontend, view.

**Client state** — what a client asks the core to keep between two runs: its
navigation state (the view the user was in, and what was selected in each) and
the album presets. The core stores it in its database and does not read it
(`Core.ClientState()`); a navigation state saved a moment before quitting is
written when the core closes.

**Database** — the one file the core keeps everything in: the queue, the
volume, the library, the playlists, the downloads, the Last.fm session, the
client state. A client gives the core its path (`core.Options.Database`); the
core opens it, and closes it last when it closes. A core that is given none
keeps nothing between runs and has no library.

## Playback

**Transition** — one move of the queue position and everything that must follow
it: the track being left is captured, the queue moves, what was played is
recorded, `TrackChange` is emitted, and the new track is started when the
transition asks for it. Four things start one: a track finishing, `Next`,
`Previous`, `JumpTo`. They differ only in their precondition, their queue move,
their start rule and what they do when the queue is exhausted; the rest lives
once in `serviceImpl.applyTransition`.

**Last-played** — the queue index and track path the playback service last
started. Not the same as the queue's current position: the queue can be moved
without playing (`QueueAdvance`, `QueueMoveTo`), and `Play` uses the difference
between the two to decide whether a track really changed.

**Gapless self-start** — the player moving to the next track by itself, without
being told. It makes a finished track ambiguous: the player is Playing either
way, and repeat-one or a duplicated queue entry start the same path again.
`player.TrackStarts()` is the only thing that can tell them apart, because it
only moves when the player begins a track.

**Exhausted** — a queue move that has nowhere to go: `Next` at the end with
repeat off. Distinct from a *precondition* failure, which is checked before the
move happens (`Previous` at index 0, `JumpTo` out of bounds).

**Queue edit** — a change to the queue's contents (add, replace, remove, move,
clear, undo, redo), made only through the playback service. One user gesture
is one queue edit: one undo step, the preloaded next track dropped, one
`QueueChange` emitted. Restoring the saved queue at startup is not a queue
edit.

## Last.fm

**Linking** — giving the core the Last.fm account it scrobbles as. Two
commands, because the user authorizes waves in a browser between them:
`BeginLink` returns a token and the URL to authorize it at, `CompleteLink`
exchanges the token for a session and saves it. The client opens the browser
and keeps the token meanwhile; the core scrobbles from then on, in this run
and the next ones, until `Unlink` forgets the session.

**Pending scrobble** — a scrobble Last.fm did not take when the track was
played, kept to be sent again. The core sends them again, oldest first, on
its own clock: at startup, right after linking, then every five minutes, while
an account is linked. A pass stops at the first one that fails. Failing to
reach Last.fm costs a scrobble nothing; one that Last.fm itself refused ten
times is dropped, and so is one older than two weeks, which Last.fm no longer
takes.
## Library

**Library source** — a directory the library is built from. A track belongs to
the library while its file is under a source; removing a source removes its
tracks.

**Scan** — one pass over library sources that adds new files, re-reads changed
ones and drops the tracks whose file is gone, then rebuilds the search index.
The core runs it, one at a time, and reports it through `ScanProgressed` and
`ScanFinished`; a client only starts it (`Refresh`, `FullRefresh`, `AddSource`).
Closing the core cuts a scan short: it ends after the files it is reading,
removes no track, rebuilds the search index and reports neither a failure nor
its end; the next scan
adds the files it did not reach and drops the tracks whose file is gone.

**Retag** — writing the tags of a MusicBrainz release on the files of a
library album, in place: each file as the track its track and disc numbers
name. The user picks the release; the core runs the retag (`Retag`), one at a
time, stops playback first when the track playing is one of the files,
reports it through `RetagProgressed` and `RetagFinished`, and reads the files
again into the library, where the album may then be under other names.

**Favorites** — the playlist with ID 1, which can be neither renamed nor
deleted. A favorite is a track in it.

## New releases

**New release** — an album or EP that ListenBrainz lists as released in the
last ninety days, or to be in the next ninety, by an artist of the library or
by a discovery. The core caches the list ListenBrainz gives and matches it
against the library each time it is read (`Releases().List`), saying of each
release whether the library already has it and whether it is being
downloaded.

**Discovery** — an artist the library has nothing by, whom Last.fm finds
similar to at least two artists of the library. It needs a Last.fm API key,
not an account.

**Refresh** (of the new releases) — downloading again the list ListenBrainz
gives, whole, when the cached one is a day old or the user asks for it. The
core runs it, one at a time, and reports its end through `ReleasesRefreshed`;
a failure is told with the list, which keeps what was cached, and the core
remembers it until a refresh works (`Releases().Status`).

**Warm-up** — asking Last.fm for the artists similar to each artist of the
library that has none fresh: the discoveries, and radio, come from them. The
core runs it, one at a time, and reports through `WarmupFinished` when it
changed them. An artist it failed for is taken again by the next one.

A client only asks for both, when it shows the list (`Releases().Refresh`).
## Downloads

**Download** — one MusicBrainz release fetched from one slskd user's folder,
including its disc subfolders (`Album\CD1`, `Album\CD2 - Live`): a row with
its files, tracked from queueing until it is imported, deleted or cleared.
`downloads.Manager` owns its whole lifecycle and is the only thing that talks
to slskd about it.

**Disc folder** — a folder whose name means a disc: `CD1`, `CD 2`, `CD02`,
`Disc 1`, `Disk 2`, optionally followed by a title after a separator
(`downloads.DiscNumber`). A user's disc folders under one parent are one
search result and one download; the import takes a file's disc from its disc
folder when its tags don't say.

**Download folder** — the folder slskd writes files to in the completed folder,
named after the last part of their slskd folder only (`downloads.BuildDiskPath`):
a download has one per slskd folder of its files, so one per disc folder. It
belongs to one download: queueing is refused when any of the download's
folders has the name of one a download waves still tracks, or already exists
on disk.

**Sync** — one pass that reads slskd's transfers into the downloads' states,
then checks completed files on disk. The core runs it, one pass at a time, on
its own clock for the whole session, and reports through `DownloadsChanged`
when the downloads are no longer what it last said; entering the downloads
view or queueing asks for one more pass (`Sync`).

**Slskd search** — one search of the Soulseek network for the folders holding
a release. The core runs it, one at a time (a new one replaces the one
running), and reports it through `SearchProgressed` and `SearchFinished`; a
client turns its responses into search results with `FilterAndScoreResults`.

**Delete** — cancel a download's slskd transfers, remove its download folders
with everything in them, and drop its row. Only on the user's explicit request,
in the downloads view.

**Import** — moving the files of a completed download into a library source
as the tracks of a MusicBrainz release: each file is tagged as its track,
named after it and moved, and the album folder gets the release's cover. A
file the release has no track for is not imported. The core first makes the
**plan** of an import (`PrepareImport`): the release the files are imported
as (their own when their tags name another than the one queued), its cover,
and the track and the destination of each file; a client shows it and the
user picks the library source. The core then runs it (`Import`), one at a
time, reports it through `ImportProgressed` and `ImportFinished`, adds the
tracks to the library and, when every file made it, ends with Finish import.

**Finish import** — what follows a successful import, which moved the
download's tracks (and a cover, if any) into the library: drop its slskd
transfer records (a stale one would match a later download of the same files)
and its row, and remove each of its download folders that is now empty.
Leftover extras (`.nfo`, `.cue`) keep theirs. Importing a partly imported
download again counts a file gone from its folder whose track is in the library
as already imported, so that import can be a success.

Clearing completed downloads drops their rows and transfer records but keeps
their files.

## Exports

**Export target** — a saved place to export to: a folder of a removable
device, known by the device's UUID so that it is found wherever the device is
mounted, or a folder of this machine. It also says how the files are laid out
under it: one folder per album, a folder per artist then per album, or a
single folder.

**Export** — copying library tracks under an export target, each file named
after its tags, a FLAC converted to MP3 when asked. A file already there is
left as it is, and a track that fails does not stop the others. The core runs
it (`Exports().Start`), several at once when asked, and reports it through
`ExportProgressed` and `ExportFinished`. It refuses a target whose device is
not plugged in, and stops an export whose device is unplugged on the way.

## Lyrics

**Lyrics** — the words of a track, line by line; synced when each line says
when it is sung, which lets a client follow playback. The core takes them from
the `.lrc` file next to the track, else from those it fetched earlier, else
from lrclib, whose answer it keeps (`Lyrics().Fetch`).

## Desktop

**MPRIS** — the D-Bus interface through which desktop widgets and media keys
show and control a media player. The core serves it over its playback when a
client asks for it (`core.Options.MPRIS`), and leaves the session bus first
when it closes.

**Desktop notification** — a message the core sends to the notification
daemon of the desktop, when the config file asks for it: the track that starts
("now playing", each one replacing the one before) and an import that made it
("download complete"). A track skipped while another is being sent is not
told, and nothing waits for the daemon to answer.
_Avoid_: notification alone, which is also the TUI's message bar
(`Model.Notifications`): that one is the client's.
