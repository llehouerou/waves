package core

import (
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/cover"
	"github.com/llehouerou/waves/internal/notify"
)

// notifications tells the desktop what the core does. One goroutine sends
// them all, away from the loop: the desktop may take its time to answer.
// Nothing waits for it, Close included: quitting must not hang on a
// notification daemon that does not answer.
type notifications struct {
	out     notify.Notifier // nil: the config file asks for none
	cfg     config.NotificationsConfig
	core    *Core
	started chan Track               // the last track that started, waiting to be told
	imports chan notify.Notification // the import that made it, waiting to be told
}

// on reports whether a setting of the config file is set, and to true.
func on(setting *bool) bool {
	return setting != nil && *setting
}

// newNotifications opens the desktop notifications the config file asks for.
// When it asks for none, nothing is opened: no connection to the session bus.
func newNotifications(c *Core, opts Options) *notifications {
	n := &notifications{
		core:    c,
		cfg:     opts.Notifications,
		started: make(chan Track, 1),
		imports: make(chan notify.Notification, 1),
	}
	if !on(n.cfg.Enabled) {
		return n
	}
	n.out = opts.notifier
	if n.out == nil {
		// Without a session bus it is a notifier that sends nothing.
		n.out, _ = notify.New()
	}
	go n.run()
	return n
}

// run sends the notifications for as long as the core lives.
func (n *notifications) run() {
	// The notification saying what plays, which the next one replaces: the
	// desktop shows one, not one per track played.
	var nowPlaying uint32
	for {
		select {
		case <-n.core.stop:
			return
		case track := <-n.started:
			note := notify.Notification{
				Title:      track.Title,
				Body:       track.Artist + " · " + track.Album,
				Timeout:    n.cfg.Timeout,
				ReplacesID: nowPlaying,
				Urgency:    notify.UrgencyLow,
			}
			// Here rather than on the loop: finding the cover reads the
			// disk, which a library on the network makes slow (issue #49).
			if on(n.cfg.ShowAlbumArt) {
				if art := cover.Find(track.Path); art != "" {
					note.Icon = "file://" + art
				}
			}
			// One that failed has no ID: the next one replaces nothing.
			nowPlaying, _ = n.out.Notify(note) //nolint:errcheck // a notification that fails is not worth telling
		case note := <-n.imports:
			_, _ = n.out.Notify(note) //nolint:errcheck // see above
		}
	}
}

// close leaves the desktop. Called by Close once the core stops: a
// notification still being sent fails rather than holding the connection.
func (n *notifications) close() {
	if n.out != nil {
		_ = n.out.Disconnect() //nolint:errcheck // nothing is left to send on it
	}
}

// trackStarted tells the desktop which track now plays. Called by the loop
// only, and it never waits: a track still waiting to be told is no longer
// what plays, and gives way.
func (n *notifications) trackStarted(track Track) {
	if n.out == nil || !on(n.cfg.NowPlaying) {
		return
	}
	select {
	case <-n.started:
	default:
	}
	n.started <- track // the loop alone sends: there is room
}

// imported tells the desktop that an import made it. It never waits: Close
// waits for the import that calls it.
func (n *notifications) imported(artist, album string) {
	if n.out == nil || !on(n.cfg.Downloads) {
		return
	}
	select {
	case n.imports <- notify.Notification{
		Title:   "Download Complete",
		Body:    artist + " - " + album,
		Timeout: n.cfg.Timeout,
		Urgency: notify.UrgencyNormal,
	}:
	default:
		// The desktop has not taken the one before: it is not answering.
	}
}
