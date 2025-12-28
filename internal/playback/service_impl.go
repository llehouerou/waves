// internal/playback/service_impl.go
package playback

import (
	"errors"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
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

	subs   []*Subscription
	subsMu sync.RWMutex

	done   chan struct{}
	closed bool
}

// New creates a new playback service.
func New(p player.Interface, q *playlist.PlayingQueue) Service {
	s := &serviceImpl{
		player: p,
		queue:  q,
		done:   make(chan struct{}),
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
	return &Track{
		ID:          t.ID,
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		TrackNumber: t.TrackNumber,
		Duration:    t.Duration,
	}
}

// TrackInfo returns metadata about the currently playing track.
func (s *serviceImpl) TrackInfo() *player.TrackInfo {
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
	tracks := s.queue.Tracks()
	result := make([]Track, len(tracks))
	for i, t := range tracks {
		result[i] = Track{
			ID:          t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
			Duration:    t.Duration,
		}
	}
	return result
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
	return s.queue.Undo()
}

// Redo reapplies the last undone queue modification.
func (s *serviceImpl) Redo() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queue.Redo()
}

// QueueAdvance advances the queue position (respecting repeat/shuffle modes)
// without starting playback. Returns the track at the new position, or nil.
func (s *serviceImpl) QueueAdvance() *Track {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.queue.Next()
	if t == nil {
		return nil
	}
	return &Track{
		ID:          t.ID,
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		TrackNumber: t.TrackNumber,
		Duration:    t.Duration,
	}
}

// QueueMoveTo moves the queue position to the specified index
// without starting playback. Returns the track at that position, or nil.
func (s *serviceImpl) QueueMoveTo(index int) *Track {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.queue.JumpTo(index)
	if t == nil {
		return nil
	}
	return &Track{
		ID:          t.ID,
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		TrackNumber: t.TrackNumber,
		Duration:    t.Duration,
	}
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

// handleTrackFinished advances to the next track when the current track ends.
func (s *serviceImpl) handleTrackFinished() {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevTrack := s.currentTrackLocked()
	prevIndex := s.queue.CurrentIndex()

	nextTrack := s.queue.Next()
	if nextTrack == nil {
		// End of queue
		s.player.Stop()
		s.emitStateChange(StatePlaying, StateStopped)
		return
	}

	s.emitTrackChange(prevTrack, prevIndex)

	if err := s.player.Play(nextTrack.Path); err != nil {
		s.player.Stop()
		s.emitStateChange(StatePlaying, StateStopped)
		s.emitError("play_next", nextTrack.Path, err)
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

	if prevIndex == currIndex {
		return // Only emit if actually changed
	}

	e := TrackChange{
		Previous: prevTrack,
		Current:  curr,
		Index:    currIndex,
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

	prevState := s.playerStateToState(s.player.State())
	if err := s.player.Play(track.Path); err != nil {
		return err
	}
	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// PlayPath plays a track directly from a file path.
// This bypasses the queue and plays the specified file.
func (s *serviceImpl) PlayPath(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevState := s.playerStateToState(s.player.State())
	if err := s.player.Play(path); err != nil {
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
		if err := s.player.Play(track.Path); err != nil {
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

	prevTrack := s.currentTrackLocked()
	prevIndex := s.queue.CurrentIndex()
	wasActive := s.player.State() == player.Playing || s.player.State() == player.Paused

	nextTrack := s.queue.Next()

	if nextTrack == nil {
		// At end of queue
		if wasActive {
			prevState := s.playerStateToState(s.player.State())
			s.player.Stop()
			currState := s.playerStateToState(s.player.State())
			s.emitStateChange(prevState, currState)
		}
		return nil
	}

	s.emitTrackChange(prevTrack, prevIndex)

	if wasActive {
		if err := s.player.Play(nextTrack.Path); err != nil {
			return err
		}
	}
	return nil
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

	prevTrack := s.currentTrackLocked()
	prevIndex := currentIndex
	wasActive := s.player.State() == player.Playing || s.player.State() == player.Paused

	newTrack := s.queue.JumpTo(currentIndex - 1)

	s.emitTrackChange(prevTrack, prevIndex)

	if wasActive && newTrack != nil {
		if err := s.player.Play(newTrack.Path); err != nil {
			return err
		}
	}
	return nil
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

	// Validate bounds
	tracks := s.queue.Tracks()
	if index < 0 || index >= len(tracks) {
		return ErrInvalidIndex
	}

	prevTrack := s.currentTrackLocked()
	prevIndex := s.queue.CurrentIndex()
	wasActive := s.player.State() == player.Playing || s.player.State() == player.Paused

	newTrack := s.queue.JumpTo(index)

	s.emitTrackChange(prevTrack, prevIndex)

	if wasActive && newTrack != nil {
		if err := s.player.Play(newTrack.Path); err != nil {
			return err
		}
	}
	return nil
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
