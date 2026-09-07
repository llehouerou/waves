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
