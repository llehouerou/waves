package lastfm

import (
	"errors"
	"fmt"

	"github.com/shkh/lastfm-go/lastfm"
)

// ErrNotAuthenticated is returned when an operation requires authentication.
var ErrNotAuthenticated = errors.New("not authenticated")

// Client wraps the Last.fm API for scrobbling operations.
type Client struct {
	api        *lastfm.Api
	apiKey     string
	apiSecret  string
	sessionKey string
}

// New creates a new Last.fm client with the given API credentials.
func New(apiKey, apiSecret string) *Client {
	return &Client{
		api:       lastfm.New(apiKey, apiSecret),
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

// SetSessionKey sets the authenticated session key.
func (c *Client) SetSessionKey(key string) {
	c.sessionKey = key
	c.api.SetSession(key)
}

// SessionKey returns the current session key.
func (c *Client) SessionKey() string {
	return c.sessionKey
}

// IsAuthenticated returns true if a session key is set.
func (c *Client) IsAuthenticated() bool {
	return c.sessionKey != ""
}

// GetToken requests an authentication token from Last.fm.
func (c *Client) GetToken() (string, error) {
	result, err := c.api.GetToken()
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

// GetSession exchanges an authorized token for a session key.
func (c *Client) GetSession(token string) (username, sessionKey string, err error) {
	err = c.api.LoginWithToken(token)
	if err != nil {
		return "", "", fmt.Errorf("get session: %w", err)
	}

	// Get the session key from the API
	sessionKey = c.api.GetSessionKey()
	c.sessionKey = sessionKey

	// Get the username by calling user.getInfo
	userInfo, err := c.api.User.GetInfo(nil)
	if err != nil {
		// Session is valid but couldn't get username - still return session
		// This can happen if Last.fm API is temporarily unavailable
		return "unknown", sessionKey, nil //nolint:nilerr // username is optional
	}

	return userInfo.Name, sessionKey, nil
}

// UpdateNowPlaying sends a "now playing" notification to Last.fm.
func (c *Client) UpdateNowPlaying(track ScrobbleTrack) error {
	if !c.IsAuthenticated() {
		return ErrNotAuthenticated
	}

	params := lastfm.P{
		"artist": track.Artist,
		"track":  track.Track,
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

	_, err := c.api.Track.UpdateNowPlaying(params)
	if err != nil {
		return fmt.Errorf("update now playing: %w", err)
	}
	return nil
}

// Scrobble submits a track play to Last.fm.
func (c *Client) Scrobble(track ScrobbleTrack) error {
	if !c.IsAuthenticated() {
		return ErrNotAuthenticated
	}

	params := lastfm.P{
		"artist":    track.Artist,
		"track":     track.Track,
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

	_, err := c.api.Track.Scrobble(params)
	if err != nil {
		return fmt.Errorf("scrobble: %w", err)
	}
	return nil
}

// ScrobbleBatch submits multiple track plays to Last.fm (up to 50).
func (c *Client) ScrobbleBatch(tracks []ScrobbleTrack) error {
	if !c.IsAuthenticated() {
		return ErrNotAuthenticated
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
		"artist":    artists,
		"track":     trackNames,
		"timestamp": timestamps,
		"album":     albums,
	}

	_, err := c.api.Track.Scrobble(params)
	if err != nil {
		return fmt.Errorf("batch scrobble: %w", err)
	}
	return nil
}
