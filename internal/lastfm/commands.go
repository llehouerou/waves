package lastfm

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/state"
)

// Message types for Last.fm operations.

// TokenResultMsg contains the result of requesting an auth token.
type TokenResultMsg struct {
	Token   string
	AuthURL string
	Err     error
}

// TokenReceivedMsg is sent when the OAuth callback receives the token.
type TokenReceivedMsg struct {
	Token string
}

// SessionResultMsg contains the result of exchanging token for session.
type SessionResultMsg struct {
	Username   string
	SessionKey string
	Err        error
}

// NowPlayingResultMsg contains the result of updating now playing.
type NowPlayingResultMsg struct {
	Err error
}

// ScrobbleResultMsg contains the result of a scrobble submission.
type ScrobbleResultMsg struct {
	TrackPath string // To correlate with the track
	Err       error
}

// RetryPendingMsg triggers retry of pending scrobbles.
type RetryPendingMsg struct{}

// RetryResultMsg contains the result of retrying pending scrobbles.
type RetryResultMsg struct {
	Succeeded int
	Failed    int
	Err       error
}

// GetTokenCmd requests an authentication token from Last.fm.
func GetTokenCmd(client *Client) tea.Cmd {
	return func() tea.Msg {
		token, err := client.GetToken()
		if err != nil {
			return TokenResultMsg{Err: err}
		}
		authURL := client.GetAuthURL(token)
		return TokenResultMsg{Token: token, AuthURL: authURL}
	}
}

// WaitForAuthCallbackCmd waits for the OAuth callback to receive the token.
func WaitForAuthCallbackCmd(tokenChan <-chan string) tea.Cmd {
	return func() tea.Msg {
		select {
		case token := <-tokenChan:
			return TokenReceivedMsg{Token: token}
		case <-time.After(5 * time.Minute):
			return TokenReceivedMsg{} // Timeout, empty token
		}
	}
}

// GetSessionCmd exchanges the authorized token for a session key.
func GetSessionCmd(client *Client, token string) tea.Cmd {
	return func() tea.Msg {
		username, sessionKey, err := client.GetSession(token)
		return SessionResultMsg{
			Username:   username,
			SessionKey: sessionKey,
			Err:        err,
		}
	}
}

// NowPlayingCmd sends a "now playing" notification to Last.fm.
func NowPlayingCmd(client *Client, track ScrobbleTrack) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateNowPlaying(track)
		return NowPlayingResultMsg{Err: err}
	}
}

// ScrobbleCmd submits a track play to Last.fm.
func ScrobbleCmd(client *Client, track ScrobbleTrack, trackPath string) tea.Cmd {
	return func() tea.Msg {
		err := client.Scrobble(track)
		return ScrobbleResultMsg{TrackPath: trackPath, Err: err}
	}
}

// RetryPendingParams contains parameters for retrying pending scrobbles.
type RetryPendingParams struct {
	Client   *Client
	StateMgr *state.Manager
}

// RetryPendingCmd retries pending scrobbles from the queue.
func RetryPendingCmd(params RetryPendingParams) tea.Cmd {
	return func() tea.Msg {
		pending, err := params.StateMgr.GetPendingScrobbles()
		if err != nil {
			return RetryResultMsg{Err: err}
		}

		if len(pending) == 0 {
			return RetryResultMsg{}
		}

		var succeeded, failed int
		const maxAttempts = 10

		for i := range pending {
			p := &pending[i]
			// Skip if too many attempts
			if p.Attempts >= maxAttempts {
				continue
			}

			track := ScrobbleTrack{
				Artist:        p.Artist,
				Track:         p.Track,
				Album:         p.Album,
				Duration:      time.Duration(p.DurationSecs) * time.Second,
				Timestamp:     p.Timestamp,
				MBRecordingID: p.MBRecordingID,
			}

			err := params.Client.Scrobble(track)
			if err != nil {
				failed++
				_ = params.StateMgr.UpdatePendingScrobbleAttempt(p.ID, err.Error())
			} else {
				succeeded++
				_ = params.StateMgr.DeletePendingScrobble(p.ID)
			}
		}

		return RetryResultMsg{Succeeded: succeeded, Failed: failed}
	}
}

// RetryTickCmd returns a command that triggers pending retry after a delay.
func RetryTickCmd() tea.Cmd {
	return tea.Tick(5*time.Minute, func(_ time.Time) tea.Msg {
		return RetryPendingMsg{}
	})
}
