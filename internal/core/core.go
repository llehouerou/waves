// Package core is the library every waves client embeds: it owns the domain
// state and the background logic, and is driven through commands, queries and
// events carrying plain data. See docs/adr/0001-embedded-core-and-gio-gui.md.
//
// Nothing in here depends on a client ticking: the core runs on its own clock.
package core

import (
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
)

// eventBuffer is how many events a client may leave unread before the core
// drops new ones rather than wait for it.
const eventBuffer = 64

// Options are the dependencies the core does not own yet. Each later step of
// the core extraction (issue #88) moves some of them inside.
type Options struct {
	State  state.Interface  // pending scrobbles
	Player player.Interface // nil: the real audio output. Tests pass player.NewMock().
	Lastfm *lastfm.Client   // nil: no scrobbling

	// tick is the period of the core's clock; zero means one second. Only
	// tests shorten it.
	tick time.Duration
}

// Core owns the domain state and the background logic of waves.
type Core struct {
	playback *Playback
	state    state.Interface
	lastfm   *lastfm.Client
	tick     time.Duration

	// play is the play of the current track. Only the loop touches it.
	play *play

	emitMu   sync.Mutex
	events   chan Event
	closed   bool
	loopDone chan struct{}
}

// New creates a core and starts its background loop.
func New(opts Options) *Core {
	p := opts.Player
	if p == nil {
		p = player.New()
	}
	tick := opts.tick
	if tick == 0 {
		tick = time.Second
	}

	svc := playback.New(p, playlist.NewQueue())
	c := &Core{
		playback: &Playback{Service: svc},
		state:    opts.State,
		lastfm:   opts.Lastfm,
		tick:     tick,
		events:   make(chan Event, eventBuffer),
		loopDone: make(chan struct{}),
	}
	go c.run(svc.Subscribe())
	return c
}

// Playback returns the playback domain: transport, queue, modes.
func (c *Core) Playback() *Playback {
	return c.playback
}

// Events returns the stream of everything that happens in the core, in the
// order the core saw it. It is closed by Close.
//
// An event is a hint that something changed: a client that falls eventBuffer
// events behind loses the newest ones, so it must re-read what it shows
// through queries rather than rebuild its state from the events alone.
func (c *Core) Events() <-chan Event {
	return c.events
}

// Close stops playback and the background loop, and ends the event stream.
func (c *Core) Close() error {
	_ = c.playback.Stop() //nolint:errcheck // Stop never fails; the signature is the service's
	err := c.playback.Close()
	<-c.loopDone

	c.emitMu.Lock()
	defer c.emitMu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.events)
	}
	return err
}

// emit sends an event to the client without ever waiting for it.
func (c *Core) emit(e Event) {
	c.emitMu.Lock()
	defer c.emitMu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.events <- e:
	default:
	}
}

// run is the core's background loop: it reacts to what the playback service
// reports, forwards it to the client, and checks on its own clock what only
// time brings (the scrobble threshold). The clock runs while a track plays.
func (c *Core) run(sub *playback.Subscription) {
	defer close(c.loopDone)

	clock := time.NewTicker(c.tick)
	clock.Stop()
	defer clock.Stop()

	for {
		select {
		case <-sub.Done:
			return
		case e := <-sub.StateChanged:
			// The live state, not the event's: events of different kinds are
			// not ordered between them, and a later one may already be read.
			if c.playback.IsPlaying() {
				clock.Reset(c.tick)
				// A first play emits no TrackChange.
				if e.Previous == playback.StateStopped {
					c.trackStarted()
				}
			} else {
				clock.Stop()
			}
			c.emit(StateChanged(e))
		case e := <-sub.TrackChanged:
			c.trackStarted()
			c.emit(TrackChanged(e))
		case e := <-sub.QueueChanged:
			c.emit(QueueChanged(e))
		case e := <-sub.ModeChanged:
			c.emit(ModeChanged(e))
		case e := <-sub.PositionChanged:
			c.emit(PositionChanged(e))
		case e := <-sub.Error:
			c.emit(PlaybackFailed(e))
		case <-clock.C:
			if c.playback.IsPlaying() {
				c.checkScrobble()
			}
		}
	}
}

// play is what the core remembers about the play of the current track.
type play struct {
	startedAt time.Time
	scrobbled bool
}

// trackStarted begins a new play of the current track.
func (c *Core) trackStarted() {
	if c.playback.CurrentTrack() == nil {
		c.play = nil
		return
	}
	c.play = &play{startedAt: time.Now()}
}
