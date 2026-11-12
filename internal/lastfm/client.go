package lastfm

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/shkh/lastfm-go/lastfm"

	"github.com/llehouerou/waves/internal/version"
)

// ErrNotAuthenticated is returned when an operation requires authentication.
var ErrNotAuthenticated = errors.New("not authenticated")

// IsRefusal reports whether err is Last.fm's own answer to a request, rather
// than a failure to reach it.
func IsRefusal(err error) bool {
	var refusal *lastfm.LastfmError
	return errors.As(err, &refusal)
}

// Last.fm API parameter names.
const (
	paramArtist = "artist"
	paramTrack  = "track"
	paramLimit  = "limit"
)

// Client wraps the Last.fm API for scrobbling operations. It is safe for
// concurrent use: an account can be linked while it scrobbles.
type Client struct {
	apiKey    string
	apiSecret string

	// api carries the session. It is replaced, never changed, when the
	// session changes: a request in flight keeps the one it started with.
	api atomic.Pointer[lastfm.Api]
}

// New creates a new Last.fm client with the given API credentials.
func New(apiKey, apiSecret string) *Client {
	c := &Client{apiKey: apiKey, apiSecret: apiSecret}
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

// GetToken requests an authentication token from Last.fm.
func (c *Client) GetToken() (string, error) {
	result, err := c.api.Load().GetToken()
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
// not authenticated by it: SetSessionKey does that.
func (c *Client) GetSession(token string) (username, sessionKey string, err error) {
	api := c.newAPI("")
	err = api.LoginWithToken(token)
	if err != nil {
		return "", "", fmt.Errorf("get session: %w", err)
	}
	sessionKey = api.GetSessionKey()

	// Get the username by calling user.getInfo
	userInfo, err := api.User.GetInfo(nil)
	if err != nil {
		// Session is valid but couldn't get username - still return session
		// This can happen if Last.fm API is temporarily unavailable
		return "unknown", sessionKey, nil //nolint:nilerr // username is optional
	}

	return userInfo.Name, sessionKey, nil
}

// UpdateNowPlaying sends a "now playing" notification to Last.fm.
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

	_, err = api.Track.UpdateNowPlaying(params)
	if err != nil {
		return fmt.Errorf("update now playing: %w", err)
	}
	return nil
}

// Scrobble submits a track play to Last.fm.
func (c *Client) Scrobble(track ScrobbleTrack) error {
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

	_, err = api.Track.Scrobble(params)
	if err != nil {
		return fmt.Errorf("scrobble: %w", err)
	}
	return nil
}

// ScrobbleBatch submits multiple track plays to Last.fm (up to 50).
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

	_, err = api.Track.Scrobble(params)
	if err != nil {
		return fmt.Errorf("batch scrobble: %w", err)
	}
	return nil
}

// GetSimilarArtists fetches similar artists from Last.fm.
// autocorrect=1: the artist names come from file tags, not from Last.fm.
func (c *Client) GetSimilarArtists(artist string, limit int) ([]SimilarArtist, error) {
	params := lastfm.P{
		paramArtist:   artist,
		paramLimit:    limit,
		"autocorrect": 1,
	}

	result, err := c.api.Load().Artist.GetSimilar(params)
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

// GetArtistTopTracks fetches top tracks for an artist from Last.fm.
func (c *Client) GetArtistTopTracks(artist string, limit int) ([]TopTrack, error) {
	params := lastfm.P{
		paramArtist: artist,
		paramLimit:  limit,
	}

	result, err := c.api.Load().Artist.GetTopTracks(params)
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
// Requires authentication.
func (c *Client) GetUserArtistTracks(artist string, limit int) ([]UserTrack, error) {
	api, err := c.authenticated()
	if err != nil {
		return nil, err
	}

	// Get the username from a previous call to user.getInfo
	userInfo, err := api.User.GetInfo(nil)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}

	params := lastfm.P{
		"user":      userInfo.Name,
		paramArtist: artist,
		paramLimit:  limit,
	}

	result, err := api.User.GetArtistTracks(params)
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
