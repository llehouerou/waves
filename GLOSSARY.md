# CONTEXT

Domain vocabulary for waves. Use these words in code, comments, commits and
issues; reach for one of them before inventing a new one.

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
then checks completed files on disk. One polling loop runs it for the whole
session; entering the downloads view or queueing runs it once more.

**Delete** — cancel a download's slskd transfers, remove its download folders
with everything in them, and drop its row. Only on the user's explicit request,
in the downloads view.

**Finish import** — what follows a successful import, which moved the
download's tracks (and a cover, if any) into the library: drop its slskd
transfer records (a stale one would match a later download of the same files)
and its row, and remove each of its download folders that is now empty.
Leftover extras (`.nfo`, `.cue`) keep theirs. Importing a partly imported
download again counts a file gone from its folder whose track is in the library
as already imported, so that import can be a success.

Clearing completed downloads drops their rows and transfer records but keeps
their files.
