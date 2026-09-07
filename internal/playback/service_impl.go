// internal/playback/service_impl.go
package playback

import (
	"errors"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/tags"
)

// Errors returned by playback service methods.
var (
	ErrEmptyQueue     = errors.New("queue is empty")
	ErrNoCurrentTrack = errors.New("no current track")
	ErrInvalidIndex   = errors.New("invalid queue index")
)

// Verify serviceImpl implements Service at compile time.
var _ Service = (*serviceImpl)(nil)

type serviceImpl struct {
	mu sync.RWMutex

	player player.Interface
	queue  *playlist.PlayingQueue

	// lastPlayedIndex tracks the queue index when Play() was last called.
	// Used to detect track changes and emit TrackChange events.
	lastPlayedIndex int
	// lastPlayedPath tracks the path of the last played track.
	// Used alongside lastPlayedIndex to detect actual track changes.
	lastPlayedPath string
	// playPathMu serialises player commands that open a file, so they stay
	// ordered over the window where startPlayback releases s.mu.
	playPathMu sync.Mutex

	// startsAtPlay is the player's start count when this service last started a
	// track. If it has moved by the time a track finishes, the player started
	// something itself (a gapless transition) and there is nothing to launch.
	startsAtPlay uint64

	subs   []*Subscription
	subsMu sync.RWMutex

	done   chan struct{}
	closed bool
}

// New creates a new playback service.
func New(p player.Interface, q *playlist.PlayingQueue) Service {
	s := &serviceImpl{
		player:          p,
		queue:           q,
		lastPlayedIndex: -1, // No track played yet
		done:            make(chan struct{}),
	}
	go s.watchTrackFinished()
	return s
}

// State returns the current playback state.
func (s *serviceImpl) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.playerStateToState(s.player.State())
}

func (s *serviceImpl) playerStateToState(ps player.State) State {
	switch ps {
	case player.Playing:
		return StatePlaying
	case player.Paused:
		return StatePaused
	case player.Stopped:
		return StateStopped
	default:
		return StateStopped
	}
}

// IsPlaying returns true if currently playing.
func (s *serviceImpl) IsPlaying() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.State() == player.Playing
}

// IsStopped returns true if currently stopped.
func (s *serviceImpl) IsStopped() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.State() == player.Stopped
}

// IsPaused returns true if currently paused.
func (s *serviceImpl) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.State() == player.Paused
}

// Position returns the current playback position.
func (s *serviceImpl) Position() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.Position()
}

// Duration returns the current track duration.
func (s *serviceImpl) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.Duration()
}

// CurrentTrack returns the current track, or nil if none.
func (s *serviceImpl) CurrentTrack() *Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTrackLocked()
}

func (s *serviceImpl) currentTrackLocked() *Track {
	t := s.queue.Current()
	if t == nil {
		return nil
	}
	track := TrackFromPlaylist(*t)
	return &track
}

// TrackInfo returns metadata about the currently playing track.
func (s *serviceImpl) TrackInfo() *tags.FileInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.TrackInfo()
}

// Player returns the underlying player interface.
// This is used for UI rendering (e.g., playerbar.NewState).
func (s *serviceImpl) Player() player.Interface {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player
}

// QueueTracks returns a copy of all tracks in the queue.
func (s *serviceImpl) QueueTracks() []Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return TracksFromPlaylist(s.queue.Tracks())
}

// QueueCurrentIndex returns the current queue index (-1 if none).
func (s *serviceImpl) QueueCurrentIndex() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.CurrentIndex()
}

// QueueLen returns the number of tracks in the queue.
func (s *serviceImpl) QueueLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.Len()
}

// QueueIsEmpty returns true if the queue is empty.
func (s *serviceImpl) QueueIsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.IsEmpty()
}

// QueueHasNext returns true if there is a next track in the queue.
func (s *serviceImpl) QueueHasNext() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.HasNext()
}

// QueuePeekNext returns the next track without advancing the queue.
// Returns nil if there is no next track.
func (s *serviceImpl) QueuePeekNext() *Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	queueTrack := s.queue.PeekNext()
	if queueTrack == nil {
		return nil
	}
	track := TrackFromPlaylist(*queueTrack)
	return &track
}

// AddTracks adds tracks to the end of the queue.
func (s *serviceImpl) AddTracks(tracks ...Track) {
	s.mu.Lock()
	defer s.mu.Unlock()
	playlistTracks := TracksToPlaylist(tracks)
	s.queue.Add(playlistTracks...)
	s.emitQueueChange()
}

// ReplaceTracks replaces all tracks in the queue.
// Returns the track at index 0 or nil if empty.
func (s *serviceImpl) ReplaceTracks(tracks ...Track) *Track {
	s.mu.Lock()
	defer s.mu.Unlock()
	playlistTracks := TracksToPlaylist(tracks)
	first := s.queue.Replace(playlistTracks...)
	s.emitQueueChange()
	if first == nil {
		return nil
	}
	result := TrackFromPlaylist(*first)
	return &result
}

// ClearQueue removes all tracks from the queue.
func (s *serviceImpl) ClearQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue.Clear()
	s.emitQueueChange()
}

// Undo reverts the last queue modification.
func (s *serviceImpl) Undo() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue.Undo() {
		s.emitQueueChange()
		return true
	}
	return false
}

// Redo reapplies the last undone queue modification.
func (s *serviceImpl) Redo() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue.Redo() {
		s.emitQueueChange()
		return true
	}
	return false
}

// QueueAdvance advances the queue position (respecting repeat/shuffle modes)
// without starting playback. Returns the track at the new position, or nil.
// Does NOT emit TrackChange - that happens when Play() is called.
func (s *serviceImpl) QueueAdvance() *Track {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.queue.Next()
	if t == nil {
		return nil
	}

	track := TrackFromPlaylist(*t)
	return &track
}

// QueueMoveTo moves the queue position to the specified index
// without starting playback. Returns the track at that position, or nil.
// Does NOT emit TrackChange - that happens when Play() is called.
func (s *serviceImpl) QueueMoveTo(index int) *Track {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.queue.JumpTo(index)
	if t == nil {
		return nil
	}

	track := TrackFromPlaylist(*t)
	return &track
}

// RepeatMode returns the current repeat mode.
func (s *serviceImpl) RepeatMode() RepeatMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return RepeatMode(s.queue.RepeatMode())
}

// Shuffle returns whether shuffle is enabled.
func (s *serviceImpl) Shuffle() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.Shuffle()
}

// Subscribe creates a new event subscription.
func (s *serviceImpl) Subscribe() *Subscription {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	sub := newSubscription()
	s.subs = append(s.subs, sub)
	return sub
}

// Close shuts down the service.
func (s *serviceImpl) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.done)
	s.mu.Unlock()

	s.subsMu.Lock()
	for _, sub := range s.subs {
		sub.close()
	}
	s.subs = nil
	s.subsMu.Unlock()

	return nil
}

// watchTrackFinished listens for track finished signals and auto-advances.
func (s *serviceImpl) watchTrackFinished() {
	for {
		select {
		case <-s.done:
			return
		case <-s.player.FinishedChan():
			s.handleTrackFinished()
		}
	}
}

// startPlayback plays a path and records the player's start count, so a later
// finish can tell whether the player started something on its own.
//
// Must be called with s.mu held, and returns with it held. It releases the lock
// while the player opens the file: that is blocking I/O, seconds on a cold
// network mount and unbounded on an unreachable one, and the renderer reads the
// same lock on every frame (issue #45). playPathMu keeps player commands
// serialised over the window where s.mu is not held, and is never held while
// waiting for s.mu, so the two cannot deadlock.
//
// The caller's captured state can go stale across that window; the cost is a
// possibly inaccurate event under concurrent commands, against a guaranteed UI
// freeze otherwise.
func (s *serviceImpl) startPlayback(path string) error {
	s.mu.Unlock()
	s.playPathMu.Lock()
	err := s.player.Play(path)
	s.playPathMu.Unlock()
	s.mu.Lock()

	s.startsAtPlay = s.player.TrackStarts()
	return err
}

// transition describes one queue move and everything that must follow it.
type transition struct {
	// move advances the queue position and returns the track now current, or
	// nil when the queue is exhausted.
	move func() *playlist.Track
	// shouldStart reports whether the player must be told to play the track
	// moved to. It runs after the move, before TrackChange is emitted.
	shouldStart func() bool
	// onExhausted runs when move returns nil. Optional.
	onExhausted func()
}

// applyTransition performs one queue transition: it captures the track being
// left, moves the queue, records what was played, starts the new track when the
// transition asks for it, and reports the change. It returns any error from
// starting playback; the path that failed is s.lastPlayedPath.
//
// Must be called with s.mu held. The start step releases and reacquires s.mu
// while the player opens the file (issue #45), so state captured before a
// transition can be stale after it.
func (s *serviceImpl) applyTransition(t transition) error {
	prevTrack := s.currentTrackLocked()
	prevIndex := s.queue.CurrentIndex()

	next := t.move()
	if next == nil {
		if t.onExhausted != nil {
			t.onExhausted()
		}
		return nil
	}

	s.lastPlayedIndex = s.queue.CurrentIndex()
	s.lastPlayedPath = next.Path

	if t.shouldStart() {
		if err := s.startPlayback(next.Path); err != nil {
			return err
		}
	}

	// Only now, with the track it moved to playing: seeking past the end stops
	// the player before signalling finished (issue #38), and a subscriber that
	// reads the live state on TrackChange would otherwise see Stopped and keep
	// that stale view until the next state change.
	s.emitTrackChange(prevTrack, prevIndex)
	return nil
}

// isActiveLocked reports whether the player is playing or paused, which is what
// makes a user-initiated transition start the track it moves to.
// Must be called while holding mu.
func (s *serviceImpl) isActiveLocked() bool {
	state := s.player.State()
	return state == player.Playing || state == player.Paused
}

// stopAndEmitLocked stops the player and reports the state it left.
// Must be called while holding mu.
func (s *serviceImpl) stopAndEmitLocked() {
	prevState := s.playerStateToState(s.player.State())
	s.player.Stop()
	s.emitStateChange(prevState, s.playerStateToState(s.player.State()))
}

// stopFromPlayingLocked stops the player and reports the stop as coming from
// Playing, which is what a track finishing always is.
// Must be called while holding mu.
func (s *serviceImpl) stopFromPlayingLocked() {
	s.player.Stop()
	s.emitStateChange(StatePlaying, StateStopped)
}

// handleTrackFinished advances to the next track when the current track ends.
func (s *serviceImpl) handleTrackFinished() {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.applyTransition(transition{
		move: s.queue.Next,
		// The player performs gapless transitions itself, so it may already
		// have started the track the queue just moved to. Neither the state nor
		// the track path can tell: it is Playing in both cases, and repeat-one
		// or a duplicated queue entry start the same path again. The start
		// counter can — it only moves when the player begins a track.
		shouldStart: func() bool {
			if s.player.TrackStarts() != s.startsAtPlay {
				s.startsAtPlay = s.player.TrackStarts()
				return false
			}
			return true
		},
		onExhausted: s.stopFromPlayingLocked,
	})
	if err != nil {
		s.stopFromPlayingLocked()
		s.emitError("play_next", s.lastPlayedPath, err)
	}
}

// emitStateChange notifies all subscribers of a state change.
// Must be called while holding mu. Acquires subsMu internally.
func (s *serviceImpl) emitStateChange(prev, curr State) {
	if prev == curr {
		return
	}
	e := StateChange{Previous: prev, Current: curr}
	s.subsMu.RLock()
	for _, sub := range s.subs {
		sub.sendState(e)
	}
	s.subsMu.RUnlock()
}

// emitTrackChange notifies all subscribers of a track change.
// Must be called while holding mu. Acquires subsMu internally.
func (s *serviceImpl) emitTrackChange(prevTrack *Track, prevIndex int) {
	curr := s.currentTrackLocked()
	currIndex := s.queue.CurrentIndex()

	// Check both index AND path - queue may have been replaced entirely
	prevPath := ""
	if prevTrack != nil {
		prevPath = prevTrack.Path
	}
	currPath := ""
	if curr != nil {
		currPath = curr.Path
	}
	if prevIndex == currIndex && prevPath == currPath {
		return // Only emit if actually changed
	}

	e := TrackChange{
		Previous:      prevTrack,
		Current:       curr,
		PreviousIndex: prevIndex,
		Index:         currIndex,
	}
	s.subsMu.RLock()
	for _, sub := range s.subs {
		sub.sendTrack(e)
	}
	s.subsMu.RUnlock()
}

// emitPositionChange notifies all subscribers of a position change.
// Must be called while holding mu. Acquires subsMu internally.
func (s *serviceImpl) emitPositionChange() {
	pos := s.player.Position()
	s.subsMu.RLock()
	for _, sub := range s.subs {
		sub.sendPosition(pos)
	}
	s.subsMu.RUnlock()
}

// emitModeChange notifies all subscribers of a mode change.
// Must be called while holding mu. Acquires subsMu internally.
func (s *serviceImpl) emitModeChange() {
	e := ModeChange{
		RepeatMode: RepeatMode(s.queue.RepeatMode()),
		Shuffle:    s.queue.Shuffle(),
	}
	s.subsMu.RLock()
	for _, sub := range s.subs {
		sub.sendMode(e)
	}
	s.subsMu.RUnlock()
}

// emitQueueChange notifies all subscribers of a queue content change.
// Must be called while holding mu. Acquires subsMu internally.
func (s *serviceImpl) emitQueueChange() {
	tracks := make([]Track, 0, len(s.queue.Tracks()))
	for _, t := range s.queue.Tracks() {
		tracks = append(tracks, TrackFromPlaylist(t))
	}
	e := QueueChange{
		Tracks: tracks,
		Index:  s.queue.CurrentIndex(),
	}
	s.subsMu.RLock()
	for _, sub := range s.subs {
		sub.sendQueue(e)
	}
	s.subsMu.RUnlock()
}

// emitError notifies all subscribers of an error.
// Must be called while holding mu. Acquires subsMu internally.
func (s *serviceImpl) emitError(operation, path string, err error) {
	e := ErrorEvent{
		Operation: operation,
		Path:      path,
		Err:       err,
	}
	s.subsMu.RLock()
	for _, sub := range s.subs {
		sub.sendError(e)
	}
	s.subsMu.RUnlock()
}

// Play starts playback of the current track in the queue.
// Emits TrackChange if the track being played is different from the last played track,
// but only if we were already playing something (not on first play).
func (s *serviceImpl) Play() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue.Tracks()) == 0 {
		return ErrEmptyQueue
	}

	track := s.queue.Current()
	if track == nil {
		return ErrNoCurrentTrack
	}

	currentIndex := s.queue.CurrentIndex()
	currentPath := track.Path

	// Check if we're playing a different track than before
	// Only emit TrackChange if we already played something (lastPlayedIndex >= 0)
	wasPlaying := s.lastPlayedIndex >= 0
	trackChanged := wasPlaying && (s.lastPlayedIndex != currentIndex || s.lastPlayedPath != currentPath)

	// Capture previous track info for TrackChange event
	var prevTrack *Track
	prevIndex := s.lastPlayedIndex
	if trackChanged {
		// Build previous track from what we remember
		prevTrack = &Track{Path: s.lastPlayedPath}
	}

	prevState := s.playerStateToState(s.player.State())
	if err := s.startPlayback(track.Path); err != nil {
		return err
	}

	// Update last played tracking
	s.lastPlayedIndex = currentIndex
	s.lastPlayedPath = currentPath

	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)

	// Emit TrackChange after state change if track actually changed
	if trackChanged {
		s.emitTrackChange(prevTrack, prevIndex)
	}

	return nil
}

// PlayPath plays a track directly from a file path.
// This bypasses the queue and plays the specified file.
func (s *serviceImpl) PlayPath(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevState := s.playerStateToState(s.player.State())
	if err := s.startPlayback(path); err != nil {
		return err
	}
	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// Pause pauses playback if currently playing.
func (s *serviceImpl) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.player.State() != player.Playing {
		return nil // no-op
	}

	prevState := s.playerStateToState(s.player.State())
	s.player.Pause()
	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// Stop stops playback.
func (s *serviceImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.player.State() == player.Stopped {
		return nil // no-op
	}

	prevState := s.playerStateToState(s.player.State())
	s.player.Stop()
	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// Toggle toggles between play and pause states.
func (s *serviceImpl) Toggle() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevState := s.playerStateToState(s.player.State())

	switch s.player.State() {
	case player.Playing:
		s.player.Pause()
	case player.Paused:
		s.player.Resume()
	case player.Stopped:
		// Play current track if available
		if len(s.queue.Tracks()) == 0 {
			return ErrEmptyQueue
		}
		track := s.queue.Current()
		if track == nil {
			return ErrNoCurrentTrack
		}
		if err := s.startPlayback(track.Path); err != nil {
			return err
		}
	}

	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// Next advances to the next track in the queue.
// If the player was active (playing or paused), it starts playing the new track.
// At end of queue (with repeat off), stops playback and emits StateChange.
func (s *serviceImpl) Next() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasActive := s.isActiveLocked()
	err := s.applyTransition(transition{
		move:        s.queue.Next,
		shouldStart: func() bool { return wasActive },
		onExhausted: func() {
			if wasActive {
				s.stopAndEmitLocked()
			}
		},
	})
	return err
}

// Previous goes back to the previous track in the queue.
// If already at the start (index 0 or less), does nothing.
// If the player was active, starts playing the new track.
func (s *serviceImpl) Previous() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentIndex := s.queue.CurrentIndex()
	if currentIndex <= 0 {
		return nil // At start, no-op
	}

	wasActive := s.isActiveLocked()
	err := s.applyTransition(transition{
		move:        func() *playlist.Track { return s.queue.JumpTo(currentIndex - 1) },
		shouldStart: func() bool { return wasActive },
	})
	return err
}

// Seek adjusts the playback position by the given delta.
func (s *serviceImpl) Seek(delta time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.player.Seek(delta)
	s.emitPositionChange()
	return nil
}

// SeekTo seeks to an absolute position.
func (s *serviceImpl) SeekTo(position time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.player.Position()
	delta := position - current
	s.player.Seek(delta)
	s.emitPositionChange()
	return nil
}

// JumpTo jumps to the specified index in the queue.
// Returns ErrInvalidIndex if the index is out of bounds.
// If the player was active, starts playing the new track.
func (s *serviceImpl) JumpTo(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.queue.Tracks()) {
		return ErrInvalidIndex
	}

	wasActive := s.isActiveLocked()
	err := s.applyTransition(transition{
		move:        func() *playlist.Track { return s.queue.JumpTo(index) },
		shouldStart: func() bool { return wasActive },
	})
	return err
}

// SetRepeatMode sets the repeat mode.
func (s *serviceImpl) SetRepeatMode(mode RepeatMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue.SetRepeatMode(playlist.RepeatMode(mode))
	s.emitModeChange()
}

// CycleRepeatMode cycles through repeat modes and returns the new mode.
func (s *serviceImpl) CycleRepeatMode() RepeatMode {
	s.mu.Lock()
	defer s.mu.Unlock()
	newMode := s.queue.CycleRepeatMode()
	s.emitModeChange()
	return RepeatMode(newMode)
}

// SetShuffle sets the shuffle state.
func (s *serviceImpl) SetShuffle(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue.SetShuffle(enabled)
	s.emitModeChange()
}

// ToggleShuffle toggles shuffle and returns the new state.
func (s *serviceImpl) ToggleShuffle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	newState := s.queue.ToggleShuffle()
	s.emitModeChange()
	return newState
}
