// Package core is the library every waves client embeds: it owns the domain
// state and the background logic, and is driven through commands, queries and
// events carrying plain data. See docs/adr/0001-embedded-core-and-gio-gui.md.
//
// Nothing in here depends on a client ticking: the core runs on its own clock.
package core

import (
	"errors"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/radio"
	"github.com/llehouerou/waves/internal/state"
)

// eventBuffer is how many events a client may leave unread before the core
// drops new ones rather than wait for it.
const eventBuffer = 64

// Options are the dependencies the core does not own yet. Each later step of
// the core extraction (issue #88) moves some of them inside.
type Options struct {
	State     state.Interface      // the database; queue, volume, pending scrobbles
	Player    player.Interface     // nil: the real audio output. Tests pass player.NewMock().
	Lastfm    *lastfm.Client       // nil: no scrobbling, no radio
	Playlists *playlists.Playlists // nil: radio ignores favorites
	Radio     config.RadioConfig

	// LibrarySources are the sources of the config file: the library takes
	// them once, when it has none.
	LibrarySources []string

	// tick is the period of the core's clock; zero means one second. Only
	// tests shorten it.
	tick time.Duration
}

// Core owns the domain state and the background logic of waves.
type Core struct {
	playback  *Playback
	library   *Library
	player    player.Interface
	state     state.Interface
	lastfm    *lastfm.Client
	radio     *radio.Radio // nil without Last.fm
	playlists *playlists.Playlists
	tick      time.Duration

	// Only the loop touches these.
	play    *play                 // the play of the current track
	filling bool                  // a radio fill is on its way
	fills   chan radio.FillResult // where it lands

	// What the persist goroutine is to save.
	queueDirty  chan struct{}
	volumeDirty chan struct{}

	emitMu sync.Mutex
	events chan Event
	closed bool

	closeOnce   sync.Once
	closeErr    error
	stop        chan struct{} // closed to stop the persist goroutine
	loopDone    chan struct{}
	persistDone chan struct{}
}

// New creates a core, restores the volume and the queue of the previous run,
// and starts its background work.
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
		player:      p,
		state:       opts.State,
		lastfm:      opts.Lastfm,
		playlists:   opts.Playlists,
		tick:        tick,
		fills:       make(chan radio.FillResult, 1), // one fill at a time: its sender never waits
		queueDirty:  make(chan struct{}, 1),
		volumeDirty: make(chan struct{}, 1),
		events:      make(chan Event, eventBuffer),
		stop:        make(chan struct{}),
		loopDone:    make(chan struct{}),
		persistDone: make(chan struct{}),
	}
	c.playback = &Playback{Service: svc, core: c}

	// A state without a database has no library: state.Mock, for the tests
	// of a client that never reads it.
	var lib *library.Library
	if db := opts.State.DB(); db != nil {
		lib = library.New(db)
	}
	c.library = &Library{libraryAPI: lib, lib: lib, core: c}
	if opts.Lastfm != nil {
		c.radio = radio.New(opts.State.DB(), opts.Lastfm, lib, opts.Radio)
	}
	c.restore(lib)
	if lib != nil {
		c.library.start(opts.LibrarySources)
	}
	go c.run(svc.Subscribe())
	go c.persist()
	return c
}

// Playback returns the playback domain: transport, queue, modes, volume.
func (c *Core) Playback() *Playback {
	return c.playback
}

// Library returns the library domain: sources, artists, albums, tracks,
// search, scan.
func (c *Core) Library() *Library {
	return c.library
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

// Close stops playback and the background work, saves the queue and the
// volume one last time, and ends the event stream.
func (c *Core) Close() error {
	c.closeOnce.Do(func() {
		_ = c.playback.Stop() //nolint:errcheck // Stop never fails; the signature is the service's
		err := c.playback.Close()
		<-c.loopDone
		close(c.stop)
		<-c.persistDone

		// Whatever was still marked, and whatever a dropped event missed.
		c.closeErr = errors.Join(err, c.saveQueue(), c.saveVolume())

		c.emitMu.Lock()
		c.closed = true
		close(c.events)
		c.emitMu.Unlock()
	})
	return c.closeErr
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
// reports, marks what must be saved, forwards the event to the client, and
// checks on its own clock what only
// time brings (the scrobble threshold, the end of the queue in radio mode).
// The clock runs while a track plays.
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
			mark(c.queueDirty) // the queue position is saved with the queue
			c.emit(TrackChanged(e))
		case e := <-sub.QueueChanged:
			mark(c.queueDirty)
			c.emit(QueueChanged(e))
		case e := <-sub.ModeChanged:
			mark(c.queueDirty) // so are repeat and shuffle
			c.emit(ModeChanged(e))
		case e := <-sub.PositionChanged:
			c.emit(PositionChanged(e))
		case e := <-sub.Error:
			c.emit(PlaybackFailed(e))
		case res := <-c.fills:
			c.radioFilled(res)
		case <-clock.C:
			if c.playback.IsPlaying() {
				c.checkScrobble()
				c.fillRadioNearEnd()
			}
		}
	}
}

// play is what the core remembers about the play of the current track.
type play struct {
	startedAt   time.Time
	scrobbled   bool
	radioFilled bool // radio already filled the queue near the end of this play
}

// trackStarted begins a new play of the current track.
func (c *Core) trackStarted() {
	if c.playback.CurrentTrack() == nil {
		c.play = nil
		return
	}
	c.play = &play{startedAt: time.Now()}
	c.fillRadioOnLastTrack()
}
