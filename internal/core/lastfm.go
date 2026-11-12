package core

import (
	"errors"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm"
)

// ErrLastfmNotConfigured is returned by what needs more of the [lastfm]
// section than the config file has: a secret to link an account, an API key
// for anything else.
var ErrLastfmNotConfigured = errors.New("the config file lacks Last.fm credentials")

// SimilarArtist is an artist Last.fm finds similar to another, and how much
// from 0 to 1.
type SimilarArtist = lastfm.SimilarArtist

// LastfmLink is an account being linked: the token Last.fm issued, and where
// the user authorizes waves with it.
type LastfmLink struct {
	Token string
	URL   string
}

// Lastfm is the Last.fm account of the user. The core scrobbles to it what is
// played, and sends again, on its own clock, the scrobbles Last.fm did not
// take.
//
// Linking an account is two commands, because the user authorizes waves in a
// browser between them: BeginLink, then CompleteLink once they have. The
// client opens the browser and keeps the token meanwhile.
type Lastfm struct {
	client     *lastfm.Client // nil without an API key
	configured bool           // an API key and a secret: an account can be linked
	core       *Core
	asked      chan struct{} // a pass over the pending scrobbles is asked for now

	mu       sync.Mutex
	username string // the account linked
}

// newLastfm builds the Last.fm client the configuration allows, linked to the
// account an earlier run saved the session of.
func newLastfm(c *Core, cfg config.LastfmConfig) *Lastfm {
	l := &Lastfm{core: c, asked: make(chan struct{}, 1)}
	if cfg.APIKey == "" {
		return l
	}
	l.client = lastfm.New(cfg.APIKey, cfg.APISecret)
	l.configured = cfg.APISecret != ""
	if !l.configured {
		return l
	}

	session, err := c.state.GetLastfmSession()
	switch {
	case err != nil:
		c.emit(Failed{Op: errmsg.OpLastfmAuth, Err: err})
	case session != nil:
		l.client.SetSessionKey(session.SessionKey)
		l.username = session.Username
	}
	return l
}

// Configured reports whether an account can be linked: the config file has a
// Last.fm API key and its secret.
func (l *Lastfm) Configured() bool {
	return l.configured
}

// Account returns the Last.fm user the core scrobbles as; linked is false
// when no account is linked.
func (l *Lastfm) Account() (username string, linked bool) {
	if !l.core.lastfmLinked() {
		return "", false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.username, true
}

// BeginLink asks Last.fm for a token and returns it with the URL the user
// authorizes waves at. Nothing is linked yet.
//
// It waits for Last.fm: a client calls it off its UI goroutine.
func (l *Lastfm) BeginLink() (LastfmLink, error) {
	if !l.configured {
		return LastfmLink{}, ErrLastfmNotConfigured
	}
	token, err := l.client.GetToken()
	if err != nil {
		return LastfmLink{}, err
	}
	return LastfmLink{Token: token, URL: l.client.GetAuthURL(token)}, nil
}

// CompleteLink exchanges a token the user has authorized for a session, and
// saves it: the core scrobbles as that account from then on, in this run and
// the next ones. It returns the account's user name.
//
// It waits for Last.fm: a client calls it off its UI goroutine.
func (l *Lastfm) CompleteLink(token string) (username string, err error) {
	if !l.configured {
		return "", ErrLastfmNotConfigured
	}
	username, sessionKey, err := l.client.GetSession(token)
	if err != nil {
		return "", err
	}
	// Saved first: linked for this run only would be unlinked at the next.
	if err := l.core.state.SaveLastfmSession(username, sessionKey); err != nil {
		return "", err
	}
	l.mu.Lock()
	l.username = username
	l.mu.Unlock()
	l.client.SetSessionKey(sessionKey)
	mark(l.asked) // what waited for an account goes out now
	return username, nil
}

// GetSimilarArtists asks Last.fm for the artists similar to one, most similar
// first, limit at most. It needs an API key, not an account.
//
// It waits for Last.fm: a client calls it off its UI goroutine.
func (l *Lastfm) GetSimilarArtists(artist string, limit int) ([]SimilarArtist, error) {
	if l.client == nil {
		return nil, ErrLastfmNotConfigured
	}
	return l.client.GetSimilarArtists(artist, limit)
}

// Unlink forgets the session of the account linked: the core no longer
// scrobbles. The scrobbles still pending wait for an account to be linked.
func (l *Lastfm) Unlink() error {
	if err := l.core.state.DeleteLastfmSession(); err != nil {
		return err
	}
	if l.client != nil {
		l.client.SetSessionKey("")
	}
	return nil
}

// scrobbleRetryTicks is how many ticks of the core's clock separate two passes
// over the pending scrobbles: five minutes.
const scrobbleRetryTicks = 300

// scrobbleMaxRefusals is how many times Last.fm may refuse a pending scrobble
// before it is given up: one that is refused that often will never be taken.
const scrobbleMaxRefusals = 10

// scrobbleMaxAge is how long a scrobble stays pending: Last.fm takes none
// older than two weeks.
const scrobbleMaxAge = 14 * 24 * time.Hour

// run sends the pending scrobbles again for as long as the core lives,
// starting with what an earlier run left. One goroutine runs every pass: two
// never send the same scrobble.
func (l *Lastfm) run() {
	clock := time.NewTicker(scrobbleRetryTicks * l.core.tick)
	defer clock.Stop()
	for {
		l.sendPending()
		select {
		case <-l.core.stop:
			return
		case <-clock.C:
		case <-l.asked:
		}
	}
}

// sendPending is one pass over the pending scrobbles, oldest first. It stops
// at the first one that fails: Last.fm is down or out of reach, and the rest
// waits for the next pass.
func (l *Lastfm) sendPending() {
	c := l.core
	if !c.lastfmLinked() {
		return
	}
	l.storeFailed(c.state.DeleteOldPendingScrobbles(scrobbleMaxAge))
	pending, err := c.state.GetPendingScrobbles()
	if err != nil {
		l.storeFailed(err)
		return
	}
	for i := range pending {
		if c.stopping() {
			return // Close waits for this pass: no more than the scrobble in flight
		}
		p := &pending[i]
		err := l.client.Scrobble(lastfm.ScrobbleTrack{
			Artist:        p.Artist,
			Track:         p.Track,
			Album:         p.Album,
			Duration:      time.Duration(p.DurationSecs) * time.Second,
			Timestamp:     p.Timestamp,
			MBRecordingID: p.MBRecordingID,
		})
		if err == nil {
			l.storeFailed(c.state.DeletePendingScrobble(p.ID))
			continue
		}
		// Failing to reach Last.fm costs the scrobble nothing; a refusal of
		// Last.fm is counted, and the last one allowed drops the scrobble.
		switch {
		case !lastfm.IsRefusal(err):
		case p.Attempts+1 >= scrobbleMaxRefusals:
			l.storeFailed(c.state.DeletePendingScrobble(p.ID))
		default:
			l.storeFailed(c.state.UpdatePendingScrobbleAttempt(p.ID, err.Error()))
		}
		return
	}
}

// storeFailed reports a failure of the database the pending scrobbles are in.
func (l *Lastfm) storeFailed(err error) {
	if err != nil {
		l.core.emit(Failed{Op: errmsg.OpLastfmScrobble, Err: err})
	}
}
