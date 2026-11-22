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
transition asks for it. Five things start one: a track finishing, `Next`,
`Previous`, `JumpTo`, and the resume of a queue that *ran out* when a radio fill
lands (`ResumeAfterRunOut`). They differ only in their precondition, their queue move,
their start rule, what they do when the queue is exhausted and the state a
failed start is reported from (Playing for a track finishing, the state before
the start otherwise); the rest lives once in `serviceImpl.applyTransition`.
`Play`, or `Toggle` from stopped, on a queue with tracks and no current track
also moves the queue, to its first track, but is not a transition: it is a
start the caller waits for, so a track that fails to open is returned to it,
not emitted as an error, and is not recorded as last played.

**Last-played** — the queue entry the playback service last started: its
index and its track path. Not the same as the queue's current position: the queue can be moved
without playing (`QueueAdvance`, `QueueMoveTo`), and `Play` uses the difference
between the two to decide whether a track really changed.

**Gapless self-start** — the player moving to the next track by itself, without
being told. It makes a finished track ambiguous: the player is Playing either
way, and repeat-one or a duplicated queue entry start the same path again.
`player.TrackStarts()` is the only thing that can tell them apart, because it
only moves when the player begins a track.

**Track start** — a track beginning to play: the service started the player on
it (`Play`, `Toggle` from stopped, a transition that starts its track) and the
open succeeded, or the player started it by a gapless self-start. A track that
starts again (repeat-one, a duplicated queue entry) is a new track start; a
resume from pause or a move of the queue that starts nothing is none. The
service reports each one as a `TrackStart`, and the core begins a play on it
alone: the scrobble's start, the "now playing" notification, the Last.fm
"now playing" update, the radio fill on the last track.

**Stale finish** — a finish that no longer belongs to the track the service
considers playing. A finish carries the player's start count of the track that
ended (`player.Interface.FinishedChan()`); once the service has begun starting a
track, or has stopped the player, the finish of any track started before is
stale, and the service ignores it: no queue move, no player call, no event. A
finish that arrives while a command still opens its track is stale too.

**Exhausted** — a queue move that has nowhere to go: the end of the queue with
repeat off or radio, an empty queue, `Next` with no current track, or the
finish of a track removed from the queue. Distinct from a *precondition*
failure, which is checked before the move happens (`Previous` at index 0,
`JumpTo` out of bounds).

**Ran out** — the queue was *exhausted* while the player was active, and not
because the track that played was removed from it: a track finished with
nothing after it, or `Next` was asked with nothing to move to. The player
stops. In radio mode, a fill that lands afterwards resumes playback on the
track that follows (the first of the fill, when the queue was cleared), unless
a track start, a `Stop`, a move of the queue position or the removal of the
current track came in between. A player stopped for any other reason (a
`Stop`, a track that failed to open, the end of a track removed from the
queue) has not run out.

**Queue edit** — a change to the queue's contents (add, replace, remove, move,
clear, undo, redo), made only through the playback service. One user gesture
is one queue edit: one undo step, the preloaded next track dropped, one
`QueueChange` emitted. Restoring the saved queue at startup is not a queue
edit. The queue position stays on its queue entry through every edit (a track
queued twice is two entries), and *Last-played* follows its own the same way.
A client reads each entry's ID with the queue (`QueueEntries`): the ID stays
the same for as long as the entry exists, undo and redo included, and a
replace makes new entries. The queue panel's selection marks entries this way,
and drops a mark for good once its entry leaves the queue.
An edit that takes the current entry out of the queue (remove, clear, an undo
or a redo whose list does not hold it) leaves the queue with no current track
while the player goes on: the track playing is then removed from the queue,
or cleared with it when the queue is left empty. An undo or a redo that brings
that entry back makes it current again, unless the queue position was set to
another track in between.
The queue following the library is not a queue edit either: once a scan or a
retag has read files again into the library, the core gives each entry of a
library track what the library now has for it, as the restore does at startup,
in the queue and in the lists undo and redo walk. No undo step, every entry
keeps its ID and its place, playback is left alone, and one `QueueChange` is
emitted when an entry changed. An entry queued from the Files view (no library
ID), or whose track the library no longer has, stays as it is.

## Last.fm

**Linking** — giving the core the Last.fm account it scrobbles as. Two
commands, because the user authorizes waves in a browser between them:
`BeginLink` returns a token and the URL to authorize it at, `CompleteLink`
exchanges the token for a session and saves it. The client opens the browser
and keeps the token meanwhile; the core scrobbles from then on, in this run
and the next ones, until `Unlink` forgets the session. A session Last.fm no
longer accepts (the user revoked waves on last.fm) is forgotten the same way
when a pass over the pending scrobbles meets it, and the user is told to link
the account again.

**Now playing update** — telling Last.fm, at each track start while an
account is linked, the track that starts, so that the profile shows it from
its start rather than once it is scrobbled. It carries what the scrobble of
that play will carry, read from the file, with no timestamp. It is best
effort: a failure of any kind is not told to the user, not kept as pending,
not sent again, and unlinks nothing; the scrobble of the play is unaffected.
One update is sent at a time, in the order of the track starts: one still
waiting when another track starts gives way. At quit, the core does not wait
for it: the one in flight is abandoned, the one waiting is never sent.

**Pending scrobble** — a scrobble Last.fm did not take when the track was
played, kept to be sent again, as it was sent at play time, album artist
included. The core sends them again, oldest first, on
its own clock: at startup, right after linking, then every five minutes, while
an account is linked. A pass stops at the first one that fails. Failing to
reach Last.fm costs a scrobble nothing, and so does a Last.fm asking to try
again (down, unavailable, rate limiting) or refusing waves' credentials. A
refusal is Last.fm refusing the scrobble itself, by any other error code: one
refused ten times is dropped, and so is one older than two weeks, which
Last.fm no longer takes. A session Last.fm no longer accepts unlinks the
account: the scrobbles wait, with nothing counted, for it to be linked again.
At quit, the core waits for Last.fm to answer about the one being sent, and
about the scrobble of the track playing, three seconds at most for both:
past them the one being sent stays pending, as if it had not been sent, the
scrobble of the track playing is kept as pending, as if Last.fm had not
taken it, and the next run sends them. Last.fm may have taken either all
the same, and then gets it twice.
## Library

**Library source** — a directory the library is built from. A source is
stored as the absolute, clean path of its directory (no trailing separator,
no doubled separator, no `.` or `..` element; a relative path is taken from
the working directory), compared lexically: symbolic links are not resolved.
A directory is a source once, whatever spelling is typed: `/x/music/` or
`/x//music` is refused when `/x/music` is a source. A source holds
the paths at or under it, on a path boundary: `/x/music2/a.mp3` is not under
`/x/music`. A track belongs to the library while a source holds its file. A
source may lie inside another (`/x/music` and `/x/music/sub`); the innermost
source of a path is the longest of the sources that hold it. Removing a source
removes the tracks it holds that no other source holds: with `/x/music/sub`
removed, its tracks stay in the library through `/x/music`, with their IDs,
playlist entries and favorites.

**Scan** — one pass over library sources that adds new files, re-reads changed
ones and drops the tracks whose file is gone, then rebuilds the search index.
The core runs it, one at a time, and reports it through `ScanProgressed` and
`ScanFinished`; a client only starts it (`Refresh`, `FullRefresh`, `AddSource`).
Once it ends, the queue follows the library (see *Queue edit*).
It reads and reports each file once, under its innermost source among the
sources it scans: a scan of `/x/music` alone reports the files of
`/x/music/sub` under `/x/music`.
It walks a source that is a symbolic link to a directory as that directory,
its tracks at paths under the source's own path (`/x/link/a/one.mp3`, never
the path of the directory the link points to); a link whose target is missing
or not a directory is a source it could not read. Only the root of a source
is followed: a symbolic link to a folder inside a source is not.
A music file that is a symbolic link is the file it points to: the scan reads
it again when the modification time of that file changes, and takes a link
whose target is missing (a disk not mounted) for a file it could not read,
never for a file that is gone. A link to a folder is no music file, whatever
its name.
It adds or updates no track that no source of the library holds when it
writes it, and reports only the tracks it wrote: a source removed while a
scan runs has no track left when the scan ends, and its files still held by
another source (`/x/music/sub` removed, `/x/music` kept) are written.
It keeps the tracks of a source or folder it could not read, and of a source in
which it finds no music file while the library has tracks for it (an unmounted
mount point, empty or not), and lists those paths in its report instead of
failing. It keeps as it is the track of a file it reads again (changed, or any file
in a full scan) and cannot read, or that names no artist or no album, and lists
the file in its report, at every scan while the file stays that way; a new such file gets no track and is not
listed. A music file anywhere under a source counts for it, inside an inner
source or not.
It leaves out the hidden entries of a source, as the Files view does: a file
or folder whose name starts with a dot, below the root of its innermost source
of the library, the root taken whatever its name and the names of the folders
above it (`~/.music` and `~/.local/share/music` are scanned). A hidden music
file, and anything in a hidden folder, gives no track and is not listed in its
report; a source inside a hidden folder (`/x/music/.inner`) is no hidden
entry, its tracks are tracks also when the outer source is scanned alone. A
track at a hidden path is removed as a track whose file is gone is, its file
there or not, and listed as removed. A hidden music file still counts for its
source as a music file found: a source whose only music files are hidden is
no unmounted mount point. It ends at the first database write that fails (adding,
updating or removing a track): it reports a failure and no stats, removes no
track after a failed add or update, keeps what it wrote before, and the next
scan does what it left.
Closing the core cuts a scan short: it ends after the files it is reading,
removes no track, rebuilds the search index and reports neither a failure nor
its end; the next scan
adds the files it did not reach and drops the tracks whose file is gone.

**Files view** — the navigator over the file system. It lists the folders and
music files of a folder, a symbolic link as what it points to: a link to a
folder is a folder, a link to a music file is a music file, a link to
nothing is not listed. Whatever is found through a link is named under the
link (`~/music/a/one.mp3` for `~/music` pointing to `/mnt/nas/music`), never
under the path it points to. Queueing a folder and the deep search walk it
as the Files view lists it: a folder that is a link is walked as the folder
it points to when it is the one queued or searched, and no walk descends
into a link to a folder below it. The deep search finds such a link as a
folder, with nothing under it; queueing a folder takes no track from it.
An entry whose name starts with a dot is hidden, a link among them: it is
not listed, the deep search does not find it, queueing takes no track from
it and walks no hidden folder, and playing a file with its folder leaves out
the hidden files next to it, that file aside. The folder listed, searched or
queued is taken whatever its name and the names of the folders above it.

**Retag** — writing the tags of a MusicBrainz release on the files of a
library album, in place: each file as the track its track and disc numbers
name. The user picks the release; the core runs the retag (`Retag`), one at a
time, stops playback first when the track playing is one of the files,
reports it through `RetagProgressed` and `RetagFinished`, and reads the files
again into the library, where the album may then be under other names:
`RetagFinished` names it as the library then has it, and each of the three
modes of the Library view (the album view, the library browser, the Miller
columns) that was on the album follows it there. The queue follows it too: its
entries of the album's tracks take the names the library now has for them.
It adds or updates no track that no source of the library holds when it reads
them back: a source removed while the retag runs does not cut it short, every
file gets its tags, and the source has no track left in the library when the
retag ends. That is not a failure of the retag.

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
changed them. An artist it failed for is taken again by the next one. At
quit, the core does not wait for it: what Last.fm answers afterwards is written
nowhere, and the next warm-up asks about those artists again.

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
on disk with anything in it (an empty folder is not taken).

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
in the downloads view. The download the core is importing is refused, and kept
whole, until its import is over.

**Import** — moving the files of a completed download into a library source
as the tracks of a MusicBrainz release: each file is tagged as its track,
named after it and moved, and the album folder gets the release's cover. No
folder or file it writes is hidden or nameless: a name that would start with
a dot starts with `_` instead, and a name the cleaning leaves empty (an artist
`?`) is `_`, its tags unchanged. A file the release has no track for is
not imported. The core first makes the
**plan** of an import (`PrepareImport`): the release the files are imported
as (their own when their tags name another than the one queued) with its
own release group, its cover, and the track and the destination of each
file; a client shows it and the user picks the library source. The core
then runs it (`Import`), one at a
time and never on a download being deleted or cleared, reports it through `ImportProgressed` and `ImportFinished`, adds the
tracks to the library and, when every file made it, ends with Finish import.
It adds or updates no track that no source of the library holds when it
writes them. Its source removed while it runs (and held by no other source),
the import goes on and moves every file into the folder, but none is in the
library: the import has not made it, `ImportFinished` says the folder is no
longer a library source, and the download is kept. The user finishes it by
adding the source again, whose scan brings the tracks in, and importing the
download again: every file is then already imported.

**Finish import** — what follows a successful import, which moved the
download's tracks (and a cover, if any) into the library: drop its slskd
transfer records (a stale one would match a later download of the same files)
and its row, and remove each of its download folders that is now empty.
Leftover extras (`.nfo`, `.cue`) keep theirs. Importing a partly imported
download again counts a file gone from its folder whose track is in the library
as already imported, so that import can be a success.

Clearing completed downloads drops their rows and transfer records but keeps
their files. The download the core is importing is held back, and the user is
told why: Finish import drops it once its import succeeds.

**Credit word** — `feat` or `ft` (lowercase, a dot after it or not) standing
as a word of its own in a name: after whitespace or an opening bracket, and
followed by neither a letter nor a digit. With `remove_feat` on, the folder and
file names an import writes drop it and everything after it, and the whole
bracket when it is inside one (`removeFeatPatterns`); its tags keep it.
`Featuring`, `Feat.` and a word that only starts with those letters
(`feature`) are not credit words.

## Exports

**Export target** — a saved place to export to: a folder of a removable
device, known by the device's UUID so that it is found wherever the device is
mounted, or a folder of this machine. It also says how the files are laid out
under it: one folder per album, a folder per artist then per album, or a
single folder.

**Export** — copying library tracks under an export target, each file named
after its tags, a FLAC converted to MP3 when asked. No folder or file it
writes is hidden: a name that would start with a dot starts with `_` instead,
its tags unchanged, so that every file lands under the target. A file gets
its name only once it is whole: it is written under its name plus `.part`,
synced, then renamed, so that a write cut short (waves killed, the device
pulled out) is written again by the next export of the track. A file already
there is left as it is, and a track that fails does not stop the others. The
core runs it (`Exports().Start`), several at once when asked, and reports it through
`ExportProgressed` and `ExportFinished`. It refuses a target whose device is
not plugged in, and stops an export whose device is unplugged on the way.

## Lyrics

**Lyrics** — the words of a track, line by line; synced when each line says
when it is sung, which lets a client follow playback. The core takes them from
the `.lrc` file next to the track, else from those it fetched earlier for the
same artist, title and duration (in whole seconds, as lrclib is asked), else
from lrclib, whose answer it keeps (`Lyrics().Fetch`). Two recordings of a
title get each their own.

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
