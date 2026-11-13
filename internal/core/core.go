// Package core is the library every waves client embeds: it owns the domain
// state and the background logic, and is driven through commands, queries and
// events carrying plain data. See docs/adr/0001-embedded-core-and-gio-gui.md.
//
// Nothing in here depends on a client ticking: the core runs on its own clock.
package core

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/listenbrainz"
	"github.com/llehouerou/waves/internal/lrclib"
	"github.com/llehouerou/waves/internal/lyrics"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/notify"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/radio"
	"github.com/llehouerou/waves/internal/releases"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/state"
)

// eventBuffer is how many events a client may leave unread before the core
// drops new ones rather than wait for it.
const eventBuffer = 64

// Options are what a client gives the core: where its data is, and what the
// config file asks for.
type Options struct {
	// Database is the path of the database file: the queue, the volume, the
	// library, the playlists, the downloads, the Last.fm session, what a
	// client keeps between runs. The core opens it, creating it when it is
	// missing, and closes it in Close. DefaultDatabase is where the user's is.
	//
	// Empty, nothing is kept between runs and the core has neither library,
	// playlists, downloads nor new releases: the tests of a client that never
	// reads them, which must not open the database of whoever runs them.
	Database string
	Player   player.Interface // nil: the real audio output. Tests pass player.NewMock().

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

	// LyricsCache is the folder the lyrics fetched from lrclib are kept in;
	// empty, they are fetched again each time. The client gives it: a test
	// must not write in the user's cache.
	LyricsCache string

	// Notifications is the [notifications] section of the config file: what
	// the core tells the desktop. Its zero value tells nothing, and opens no
	// connection to the session bus.
	Notifications config.NotificationsConfig

	// MPRIS registers waves on the session bus as a media player, which
	// desktop widgets and media keys show and control. The zero value stays
	// off the bus.
	MPRIS bool

	// tick is the period of the core's clock; zero means one second. Only
	// tests shorten it.
	tick time.Duration

	// musicbrainz stands in for MusicBrainz; nil means the real one. Only
	// tests replace it: they must pass offline.
	musicbrainz musicbrainzAPI

	// listenbrainz stands in for ListenBrainz, the same way.
	listenbrainz freshReleasesAPI

	// volumes stands in for the removable devices mounted on this machine;
	// nil means those it really has. Only tests replace it.
	volumes func() ([]ExportVolume, error)

	// lrclib stands in for lrclib, as musicbrainz does for MusicBrainz.
	lrclib lyrics.Client

	// notifier stands in for the desktop the notifications are sent to; nil
	// means the real one. Only tests replace it.
	notifier notify.Notifier

	// mpris stands in for the MPRIS server on the session bus; nil means the
	// real one. Only tests replace it.
	mpris func(playback.Service) io.Closer

	// state is a database already open, which the core then does not close.
	// Only tests give one: they leave in it what an earlier run would have,
	// and read it after Close.
	state state.Interface
}

// Core owns the domain state and the background logic of waves.
type Core struct {
	playback    *Playback
	library     *Library
	playlists   *Playlists
	downloads   *Downloads
	musicbrainz *MusicBrainz
	lastfm      *Lastfm
	releases    *Releases
	exports     *Exports
	lyrics      *Lyrics
	player      player.Interface
	state       state.Interface
	ownsState   bool         // the core opened the database: Close closes it
	radio       *radio.Radio // nil unless a Last.fm account can be linked
	tick        time.Duration

	notifications *notifications
	mpris         io.Closer // the MPRIS server on the session bus; nil when none is served

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
	work        sync.WaitGroup // background work that Close waits for: it writes to the database
}

// New opens the database, creates a core, restores the volume and the queue
// of the previous run, and starts its background work. It fails when the
// database cannot be opened, and has then started nothing.
func New(opts Options) (*Core, error) {
	st, ownsState, err := openState(opts)
	if err != nil {
		return nil, err
	}

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
		state:       st,
		ownsState:   ownsState,
		tick:        tick,
		fills:       make(chan radio.FillResult, 1), // one fill at a time: its sender never waits
		queueDirty:  make(chan struct{}, 1),
		volumeDirty: make(chan struct{}, 1),
		events:      make(chan Event, eventBuffer),
		stop:        make(chan struct{}),
		loopDone:    make(chan struct{}),
		persistDone: make(chan struct{}),
	}
	c.playback = &Playback{playbackAPI: svc, svc: svc, core: c}

	var slskdClient *slskd.Client
	if opts.Slskd.Configured() {
		slskdClient = slskd.NewClient(opts.Slskd.URL, opts.Slskd.APIKey)
	}

	// A core that was given no database has neither library, playlists,
	// downloads nor new releases: the tests of a client that never reads
	// them.
	var (
		lib *library.Library
		pls *playlists.Playlists
		dls *downloads.Manager
		rel *releases.Cache
	)
	if db := c.state.DB(); db != nil {
		lib = library.New(db)
		pls = playlists.New(db, lib)
		dls = downloads.New(db, slskdClient, opts.Slskd.CompletedPath)
		rel = releases.NewCache(db)
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
		c.radio = radio.New(c.state.DB(), c.lastfm.client, lib, opts.Radio)
	}
	lb := opts.listenbrainz
	if lb == nil {
		lb = listenbrainz.New()
	}
	c.releases = &Releases{cache: rel, db: c.state.DB(), lb: lb, ttlDays: opts.Radio.CacheTTLDays, core: c}
	c.exports = &Exports{volumes: opts.volumes, core: c}
	if c.exports.volumes == nil {
		c.exports.volumes = export.DetectVolumes
	}
	if db := c.state.DB(); db != nil {
		c.exports.targets = export.NewTargetRepository(db)
	}
	lrc := opts.lrclib
	if lrc == nil {
		lrc = lrclib.New()
	}
	c.lyrics = &Lyrics{source: lyrics.NewSource(opts.LyricsCache, lrc), core: c}
	c.notifications = newNotifications(c, opts)
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
	// Last: the queue is restored when a desktop widget asks what plays.
	if opts.MPRIS {
		serve := opts.mpris
		if serve == nil {
			serve = serveMPRIS
		}
		c.mpris = serve(svc)
	}
	return c, nil
}

// DefaultDatabase is where the database of the user is: waves.db, in the
// waves folder of their data directory.
func DefaultDatabase() (string, error) {
	return state.DefaultPath()
}

// openState returns the state the core runs on, and whether the core opened
// it, and so closes it.
func openState(opts Options) (st state.Interface, owns bool, err error) {
	switch {
	case opts.state != nil:
		return opts.state, false, nil
	case opts.Database == "":
		return state.NewMock(), true, nil
	}
	mgr, err := state.OpenAt(opts.Database)
	if err != nil {
		return nil, false, err
	}
	return mgr, true, nil
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

// Releases returns the new releases domain: what the artists of the library,
// and those similar to them, release.
func (c *Core) Releases() *Releases {
	return c.releases
}

// Exports returns the exports domain: the export targets, and the exports
// to them.
func (c *Core) Exports() *Exports {
	return c.exports
}

// Lyrics returns where the lyrics of a track come from.
func (c *Core) Lyrics() *Lyrics {
	return c.lyrics
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
// volume one last time, closes the database and ends the event stream.
func (c *Core) Close() error {
	c.closeOnce.Do(func() {
		// MPRIS leaves the session bus first: a widget told that playback
		// stopped would offer to start it again, on a core that is closing.
		if c.mpris != nil {
			_ = c.mpris.Close() //nolint:errcheck // it fails when the bus was never reached: nothing to leave
		}
		_ = c.playback.Stop() //nolint:errcheck // Stop never fails; the signature is the service's
		err := c.playback.svc.Close()
		<-c.loopDone
		close(c.stop)
		<-c.persistDone
		c.downloads.close()
		// An import or a retag ends after the file it is on, a pass over
		// the pending scrobbles after the one it is sending, a refresh of
		// the new releases at once, a warm-up after its requests in flight:
		// the database must still be there for them. An export ends after
		// the copy it is on, at once when it is converting: no file is left
		// half written.
		c.work.Wait()

		// Whatever was still marked, and whatever a dropped event missed.
		c.closeErr = errors.Join(err, c.saveQueue(), c.saveVolume())
		// The database last: all of the above wrote there. What a client
		// saved of its state a moment ago is written now.
		if c.ownsState {
			c.closeErr = errors.Join(c.closeErr, c.state.Close())
		}

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

// untilStopped returns a context that ends when Close begins, for the
// background work a request in flight would otherwise hold Close for. cancel
// releases it when the work ends first.
func (c *Core) untilStopped() (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		select {
		case <-c.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
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
	track := c.playback.CurrentTrack()
	if track == nil {
		c.play = nil
		return
	}
	c.play = &play{startedAt: time.Now()}
	c.notifications.trackStarted(*track)
	c.fillRadioOnLastTrack()
}
