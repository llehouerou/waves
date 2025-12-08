// internal/app/playback_manager.go
package app

import (
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui/playerbar"
)

// PlaybackManager manages audio playback, the queue, and display mode.
type PlaybackManager struct {
	player      player.Interface
	queue       *playlist.PlayingQueue
	displayMode playerbar.DisplayMode
}

// NewPlaybackManager creates a new PlaybackManager.
func NewPlaybackManager(p player.Interface, q *playlist.PlayingQueue) PlaybackManager {
	return PlaybackManager{
		player:      p,
		queue:       q,
		displayMode: playerbar.ModeExpanded,
	}
}

// --- Player Access ---

// Player returns the player interface for direct access.
func (p *PlaybackManager) Player() player.Interface {
	return p.player
}

// SetPlayer replaces the player implementation.
func (p *PlaybackManager) SetPlayer(pl player.Interface) {
	p.player = pl
}

// --- Queue Access ---

// Queue returns the playing queue for direct access.
func (p *PlaybackManager) Queue() *playlist.PlayingQueue {
	return p.queue
}

// SetQueue replaces the queue.
func (p *PlaybackManager) SetQueue(q *playlist.PlayingQueue) {
	p.queue = q
}

// --- Player State ---

// State returns the current player state.
func (p *PlaybackManager) State() player.State {
	return p.player.State()
}

// IsPlaying returns true if currently playing.
func (p *PlaybackManager) IsPlaying() bool {
	return p.player.State() == player.Playing
}

// IsPaused returns true if currently paused.
func (p *PlaybackManager) IsPaused() bool {
	return p.player.State() == player.Paused
}

// IsStopped returns true if currently stopped.
func (p *PlaybackManager) IsStopped() bool {
	return p.player.State() == player.Stopped
}

// --- Player Controls ---

// Play starts playback of a track by path.
func (p *PlaybackManager) Play(path string) error {
	return p.player.Play(path)
}

// Pause pauses playback.
func (p *PlaybackManager) Pause() {
	p.player.Pause()
}

// Resume resumes playback.
func (p *PlaybackManager) Resume() {
	p.player.Resume()
}

// Toggle toggles between play and pause.
func (p *PlaybackManager) Toggle() {
	p.player.Toggle()
}

// Stop stops playback.
func (p *PlaybackManager) Stop() {
	p.player.Stop()
}

// Seek seeks by the given duration.
func (p *PlaybackManager) Seek(delta time.Duration) {
	p.player.Seek(delta)
}

// --- Position and Duration ---

// Position returns the current playback position.
func (p *PlaybackManager) Position() time.Duration {
	return p.player.Position()
}

// Duration returns the total duration of the current track.
func (p *PlaybackManager) Duration() time.Duration {
	return p.player.Duration()
}

// --- Current Track ---

// CurrentTrack returns the currently playing track, or nil if none.
func (p *PlaybackManager) CurrentTrack() *playlist.Track {
	return p.queue.Current()
}

// --- Display Mode ---

// DisplayMode returns the current player bar display mode.
func (p *PlaybackManager) DisplayMode() playerbar.DisplayMode {
	return p.displayMode
}

// SetDisplayMode sets the player bar display mode.
func (p *PlaybackManager) SetDisplayMode(mode playerbar.DisplayMode) {
	p.displayMode = mode
}

// ToggleDisplayMode cycles between compact and expanded display.
func (p *PlaybackManager) ToggleDisplayMode() {
	if p.displayMode == playerbar.ModeExpanded {
		p.displayMode = playerbar.ModeCompact
	} else {
		p.displayMode = playerbar.ModeExpanded
	}
}

// --- Finished Channel ---

// FinishedChan returns the channel that signals track completion.
func (p *PlaybackManager) FinishedChan() <-chan struct{} {
	return p.player.FinishedChan()
}
