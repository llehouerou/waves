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
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/radio"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/state"
)

// eventBuffer is how many events a client may leave unread before the core
// drops new ones rather than wait for it.
const eventBuffer = 64

// Options are the dependencies the core does not own yet. Each later step of
// the core extraction (issue #88) moves some of them inside.
type Options struct {
	State  state.Interface  // the database; queue, volume, Last.fm session, pending scrobbles
	Player player.Interface // nil: the real audio output. Tests pass player.NewMock().

	// Lastfm is the [lastfm] section of the config file. No API key: no
	// Last.fm at all. No secret: no account can be linked, so no scrobbling
	// and no radio.
	Lastfm config.LastfmConfig
	Radio  config.RadioConfig

	// LibrarySources are the sources of the config file: the library takes
	// them once, when it has none.
	LibrarySources []string

	// Slskd is where slskd is and where it writes finished files; the zero
	// value means no slskd: downloads are kept, never queued nor synced.
	Slskd config.SlskdConfig

	// Rename is how an import names the files it moves into the library.
	Rename config.RenameConfig

	// tick is the period of the core's clock; zero means one second. Only
	// tests shorten it.
	tick time.Duration

	// musicbrainz stands in for MusicBrainz; nil means the real one. Only
	// tests replace it: they must pass offline.
	musicbrainz musicbrainzAPI
}

// Core owns the domain state and the background logic of waves.
type Core struct {
	playback    *Playback
	library     *Library
	playlists   *Playlists
	downloads   *Downloads
	musicbrainz *MusicBrainz
	lastfm      *Lastfm
	player      player.Interface
	state       state.Interface
	radio       *radio.Radio // nil unless a Last.fm account can be linked
	tick        time.Duration

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
	work        sync.WaitGroup // what Close waits for: an import, a retag, the pending scrobbles
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

	var slskdClient *slskd.Client
	if opts.Slskd.Configured() {
		slskdClient = slskd.NewClient(opts.Slskd.URL, opts.Slskd.APIKey)
	}

	// A state without a database has neither library, playlists nor
	// downloads: state.Mock, for the tests of a client that never reads them.
	var (
		lib *library.Library
		pls *playlists.Playlists
		dls *downloads.Manager
	)
	if db := opts.State.DB(); db != nil {
		lib = library.New(db)
		pls = playlists.New(db, lib)
		dls = downloads.New(db, slskdClient, opts.Slskd.CompletedPath)
	}
	c.library = &Library{libraryAPI: lib, lib: lib, core: c}
	c.playlists = &Playlists{playlistsAPI: pls, pls: pls}
	c.downloads = &Downloads{
		downloadsAPI:  dls,
		mgr:           dls,
		client:        slskdClient,
		core:          c,
		completedPath: opts.Slskd.CompletedPath,
		rename:        opts.Rename.ToRenameConfig(),
		asked:         make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
	mb := opts.musicbrainz
	if mb == nil {
		mb = musicbrainz.NewClient()
	}
	c.musicbrainz = &MusicBrainz{musicbrainzAPI: mb}
	c.lastfm = newLastfm(c, opts.Lastfm)
	if c.lastfm.configured {
		c.radio = radio.New(opts.State.DB(), c.lastfm.client, lib, opts.Radio)
	}
	c.restore(lib)
	if lib != nil {
		c.library.start(opts.LibrarySources)
	}
	go c.run(svc.Subscribe())
	go c.persist()
	c.downloads.start()
	if c.lastfm.configured {
		c.work.Go(c.lastfm.run)
	}
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

// Playlists returns the playlists domain: playlists, folders, favorites.
func (c *Core) Playlists() *Playlists {
	return c.playlists
}

// Downloads returns the downloads domain: the downloads, their Sync with
// slskd, the slskd search.
func (c *Core) Downloads() *Downloads {
	return c.downloads
}

// MusicBrainz returns the MusicBrainz catalogue.
func (c *Core) MusicBrainz() *MusicBrainz {
	return c.musicbrainz
}

// Lastfm returns the Last.fm account of the user.
func (c *Core) Lastfm() *Lastfm {
	return c.lastfm
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
		c.downloads.close()
		// An import or a retag ends after the file it is on, a pass over
		// the pending scrobbles after the one it is sending: the database
		// must still be there for them.
		c.work.Wait()

		// Whatever was still marked, and whatever a dropped event missed.
		c.closeErr = errors.Join(err, c.saveQueue(), c.saveVolume())

		c.emitMu.Lock()
		c.closed = true
		close(c.events)
		c.emitMu.Unlock()
	})
	return c.closeErr
}

// stopping reports whether Close has begun: background work it cuts short
// must not report that as a failure.
func (c *Core) stopping() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
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
