package lastfm

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/shkh/lastfm-go/lastfm"

	"github.com/llehouerou/waves/internal/version"
)

// ErrNotAuthenticated is returned when an operation requires authentication.
var ErrNotAuthenticated = errors.New("not authenticated")

// ErrTimeout is returned when Last.fm did not answer a request within the
// bound of the client: a failure to reach it, not an answer of Last.fm.
var ErrTimeout = errors.New("no answer from Last.fm in time")

// requestTimeout bounds each request to Last.fm, body included.
const requestTimeout = 30 * time.Second

// Last.fm error codes that say nothing of the request itself.
const (
	codeAuthenticationFailed = 4
	codeOperationFailed      = 8 // backend error: try again
	codeInvalidSession       = 9
	codeInvalidAPIKey        = 10
	codeServiceOffline       = 11
	codeInvalidSignature     = 13
	codeTemporaryError       = 16
	codeSuspendedAPIKey      = 26
	codeRateLimitExceeded    = 29
)

// lastfmCode returns the code of Last.fm's error answer err carries; ok is
// false when err is no answer of Last.fm, a failure to reach it.
func lastfmCode(err error) (code int, ok bool) {
	var answer *lastfm.LastfmError
	if !errors.As(err, &answer) {
		return 0, false
	}
	return answer.Code, true
}

// IsRefusal reports whether err is Last.fm refusing the request itself: one
// it will refuse however often it is sent. A failure to reach Last.fm is no
// refusal, and neither is an answer that blames something else than the
// request: Last.fm asking to try again later (down, unavailable, rate
// limiting), the application's credentials, or the session (IsSessionLost).
func IsRefusal(err error) bool {
	code, ok := lastfmCode(err)
	if !ok {
		return false
	}
	switch code {
	case codeOperationFailed, codeServiceOffline, codeTemporaryError, codeRateLimitExceeded,
		codeAuthenticationFailed, codeInvalidAPIKey, codeInvalidSignature, codeSuspendedAPIKey,
		codeInvalidSession:
		return false
	}
	return true
}

// IsSessionLost reports whether err is Last.fm no longer accepting the
// session: the user revoked waves on last.fm, and the account must be linked
// again.
func IsSessionLost(err error) bool {
	code, ok := lastfmCode(err)
	return ok && code == codeInvalidSession
}

// Last.fm API parameter names.
const (
	paramArtist = "artist"
	paramTrack  = "track"
	paramLimit  = "limit"
)

// Client wraps the Last.fm API for scrobbling operations. It is safe for
// concurrent use: an account can be linked while it scrobbles. Each request
// it sends ends within its bound: past it, the method returns ErrTimeout.
type Client struct {
	apiKey    string
	apiSecret string
	timeout   time.Duration // the bound of each request

	// api carries the session. It is replaced, never changed, when the
	// session changes: a request in flight keeps the one it started with.
	api atomic.Pointer[lastfm.Api]
}

// New creates a new Last.fm client with the given API credentials.
func New(apiKey, apiSecret string) *Client {
	c := &Client{apiKey: apiKey, apiSecret: apiSecret, timeout: requestTimeout}
	c.SetSessionKey("")
	return c
}

// newAPI builds the API of a session; "" for none.
func (c *Client) newAPI(sessionKey string) *lastfm.Api {
	api := lastfm.New(c.apiKey, c.apiSecret)
	api.SetUserAgent(version.UserAgent())
	api.SetSession(sessionKey)
	return api
}

// SetSessionKey sets the authenticated session key; "" unlinks the client.
func (c *Client) SetSessionKey(key string) {
	c.api.Store(c.newAPI(key))
}

// IsAuthenticated returns true if a session key is set.
func (c *Client) IsAuthenticated() bool {
	return c.api.Load().GetSessionKey() != ""
}

// authenticated returns the API of the session, or ErrNotAuthenticated when
// there is none.
func (c *Client) authenticated() (*lastfm.Api, error) {
	api := c.api.Load()
	if api.GetSessionKey() == "" {
		return nil, ErrNotAuthenticated
	}
	return api, nil
}

// GetToken requests an authentication token from Last.fm. It returns
// ErrTimeout when Last.fm did not answer in time.
func (c *Client) GetToken() (string, error) {
	api := c.api.Load()
	result, err := await(context.Background(), c.timeout, api.GetToken)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	return result, nil
}

// GetAuthURL returns the URL for user authorization (desktop auth flow).
// User authorizes on Last.fm, then returns to the app and confirms.
func (c *Client) GetAuthURL(token string) string {
	return fmt.Sprintf("https://www.last.fm/api/auth/?api_key=%s&token=%s", c.apiKey, token)
}

// GetSession exchanges an authorized token for a session key. The client is
// not authenticated by it: SetSessionKey does that. It returns ErrTimeout
// when Last.fm did not answer the exchange in time, and the username
// "unknown" when it did not answer user.getInfo in time.
func (c *Client) GetSession(token string) (username, sessionKey string, err error) {
	// The exchange writes the session key into api: only the call reads it,
	// so a call left behind by the bound writes nothing read afterwards.
	api := c.newAPI("")
	sessionKey, err = await(context.Background(), c.timeout, func() (string, error) {
		if err := api.LoginWithToken(token); err != nil {
			return "", err
		}
		return api.GetSessionKey(), nil
	})
	if err != nil {
		return "", "", fmt.Errorf("get session: %w", err)
	}

	// Get the username by calling user.getInfo
	userInfo, err := await(context.Background(), c.timeout, func() (lastfm.UserGetInfo, error) {
		return api.User.GetInfo(nil)
	})
	if err != nil {
		// Session is valid but couldn't get username - still return session
		// This can happen if Last.fm API is temporarily unavailable
		return "unknown", sessionKey, nil //nolint:nilerr // username is optional
	}

	return userInfo.Name, sessionKey, nil
}

// UpdateNowPlaying sends a "now playing" notification to Last.fm. It returns
// ErrTimeout when Last.fm did not answer in time.
func (c *Client) UpdateNowPlaying(track ScrobbleTrack) error {
	api, err := c.authenticated()
	if err != nil {
		return err
	}

	params := lastfm.P{
		paramArtist: track.Artist,
		paramTrack:  track.Track,
	}

	if track.Album != "" {
		params["album"] = track.Album
	}
	if track.AlbumArtist != "" && track.AlbumArtist != track.Artist {
		params["albumArtist"] = track.AlbumArtist
	}
	if track.Duration > 0 {
		params["duration"] = int(track.Duration.Seconds())
	}
	if track.MBRecordingID != "" {
		params["mbid"] = track.MBRecordingID
	}

	_, err = await(context.Background(), c.timeout, func() (lastfm.TrackUpdateNowPlaying, error) {
		return api.Track.UpdateNowPlaying(params)
	})
	if err != nil {
		return fmt.Errorf("update now playing: %w", err)
	}
	return nil
}

// Scrobble submits a track play to Last.fm. It returns the error of ctx when
// ctx ends before Last.fm answered, and ErrTimeout when Last.fm did not answer
// in time: whether Last.fm took the scrobble is then unknown.
func (c *Client) Scrobble(ctx context.Context, track ScrobbleTrack) error {
	api, err := c.authenticated()
	if err != nil {
		return err
	}

	params := lastfm.P{
		paramArtist: track.Artist,
		paramTrack:  track.Track,
		"timestamp": track.Timestamp.Unix(),
	}

	if track.Album != "" {
		params["album"] = track.Album
	}
	if track.AlbumArtist != "" && track.AlbumArtist != track.Artist {
		params["albumArtist"] = track.AlbumArtist
	}
	if track.Duration > 0 {
		params["duration"] = int(track.Duration.Seconds())
	}
	if track.MBRecordingID != "" {
		params["mbid"] = track.MBRecordingID
	}

	_, err = await(ctx, c.timeout, func() (lastfm.TrackScrobble, error) { return api.Track.Scrobble(params) })
	if err != nil {
		return fmt.Errorf("scrobble: %w", err)
	}
	return nil
}

// ScrobbleBatch submits multiple track plays to Last.fm (up to 50). It
// returns ErrTimeout when Last.fm did not answer in time.
func (c *Client) ScrobbleBatch(tracks []ScrobbleTrack) error {
	api, err := c.authenticated()
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return nil
	}
	if len(tracks) > 50 {
		tracks = tracks[:50] // Last.fm limit
	}

	// Build arrays for batch submission
	artists := make([]string, len(tracks))
	trackNames := make([]string, len(tracks))
	timestamps := make([]int64, len(tracks))
	albums := make([]string, len(tracks))

	for i, t := range tracks {
		artists[i] = t.Artist
		trackNames[i] = t.Track
		timestamps[i] = t.Timestamp.Unix()
		albums[i] = t.Album
	}

	params := lastfm.P{
		paramArtist: artists,
		paramTrack:  trackNames,
		"timestamp": timestamps,
		"album":     albums,
	}

	_, err = await(context.Background(), c.timeout, func() (lastfm.TrackScrobble, error) {
		return api.Track.Scrobble(params)
	})
	if err != nil {
		return fmt.Errorf("batch scrobble: %w", err)
	}
	return nil
}

// GetSimilarArtists fetches similar artists from Last.fm. It returns the error
// of ctx when ctx ends before Last.fm answered, and ErrTimeout when Last.fm
// did not answer in time.
// autocorrect=1: the artist names come from file tags, not from Last.fm.
func (c *Client) GetSimilarArtists(ctx context.Context, artist string, limit int) ([]SimilarArtist, error) {
	params := lastfm.P{
		paramArtist:   artist,
		paramLimit:    limit,
		"autocorrect": 1,
	}

	api := c.api.Load()
	result, err := await(ctx, c.timeout, func() (lastfm.ArtistGetSimilar, error) { return api.Artist.GetSimilar(params) })
	if err != nil {
		return nil, fmt.Errorf("get similar artists: %w", err)
	}

	artists := make([]SimilarArtist, 0, len(result.Similars))
	for _, a := range result.Similars {
		score := 0.0
		if a.Match != "" {
			_, _ = fmt.Sscanf(a.Match, "%f", &score) //nolint:errcheck // parse failure means score stays 0
		}
		artists = append(artists, SimilarArtist{
			Name:       a.Name,
			MatchScore: score,
		})
	}

	return artists, nil
}

// GetArtistTopTracks fetches top tracks for an artist from Last.fm. It
// returns ErrTimeout when Last.fm did not answer in time.
func (c *Client) GetArtistTopTracks(artist string, limit int) ([]TopTrack, error) {
	params := lastfm.P{
		paramArtist: artist,
		paramLimit:  limit,
	}

	api := c.api.Load()
	result, err := await(context.Background(), c.timeout, func() (lastfm.ArtistGetTopTracks, error) {
		return api.Artist.GetTopTracks(params)
	})
	if err != nil {
		return nil, fmt.Errorf("get artist top tracks: %w", err)
	}

	tracks := make([]TopTrack, 0, len(result.Tracks))
	for i, t := range result.Tracks {
		playcount := 0
		if t.PlayCount != "" {
			_, _ = fmt.Sscanf(t.PlayCount, "%d", &playcount) //nolint:errcheck // parse failure means count stays 0
		}
		tracks = append(tracks, TopTrack{
			Name:      t.Name,
			Playcount: playcount,
			Rank:      i + 1,
		})
	}

	return tracks, nil
}

// GetUserArtistTracks fetches tracks the user has scrobbled for an artist.
// Requires authentication. It returns ErrTimeout when Last.fm did not answer
// one of its two requests in time.
func (c *Client) GetUserArtistTracks(artist string, limit int) ([]UserTrack, error) {
	api, err := c.authenticated()
	if err != nil {
		return nil, err
	}

	// Get the username from a previous call to user.getInfo
	userInfo, err := await(context.Background(), c.timeout, func() (lastfm.UserGetInfo, error) {
		return api.User.GetInfo(nil)
	})
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}

	params := lastfm.P{
		"user":      userInfo.Name,
		paramArtist: artist,
		paramLimit:  limit,
	}

	result, err := await(context.Background(), c.timeout, func() (lastfm.UserGetArtistTracks, error) {
		return api.User.GetArtistTracks(params)
	})
	if err != nil {
		return nil, fmt.Errorf("get user artist tracks: %w", err)
	}

	// Aggregate plays per track
	trackPlays := make(map[string]int)
	for i := range result.Tracks {
		trackPlays[result.Tracks[i].Name]++
	}

	tracks := make([]UserTrack, 0, len(trackPlays))
	for name, count := range trackPlays {
		tracks = append(tracks, UserTrack{
			Name:      name,
			Playcount: count,
		})
	}

	return tracks, nil
}

// await is the one place a call of shkh/lastfm-go is waited for. It returns
// what call returns, the error of ctx when ctx ends first, or ErrTimeout when
// call has not returned bound after it began. The library takes no context:
// a call left behind is left to end on its own, keeping its connection until
// Last.fm lets go, and what it returns is dropped.
func await[T any](ctx context.Context, bound time.Duration, call func() (T, error)) (T, error) {
	type answer struct {
		value T
		err   error
	}
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	answered := make(chan answer, 1) // the call never waits for a receiver gone
	timer := time.NewTimer(bound)
	defer timer.Stop()
	go func() {
		value, err := call()
		answered <- answer{value, err}
	}()
	select {
	case a := <-answered:
		return a.value, a.err
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-timer.C:
		return zero, ErrTimeout
	}
}
