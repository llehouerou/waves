package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/lastfmauth"
)

// handleLastfmMsg handles Last.fm related messages.
func (m *Model) handleLastfmMsg(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case lastfm.TokenResultMsg:
		return m.handleLastfmTokenResult(msg)
	case lastfm.SessionResultMsg:
		return m.handleLastfmSessionResult(msg)
	case lastfm.NowPlayingResultMsg:
		return m.handleLastfmNowPlayingResult(msg)
	case lastfm.ScrobbleResultMsg:
		return m.handleLastfmScrobbleResult(msg)
	case lastfm.RetryPendingMsg:
		return m.handleLastfmRetryPending()
	case lastfm.RetryResultMsg:
		return m.handleLastfmRetryResult(msg)
	case lastfmauth.ActionMsg:
		return m.handleLastfmAuthAction(msg)
	}
	return *m, nil
}

// handleLastfmTokenResult handles the result of requesting an auth token.
func (m *Model) handleLastfmTokenResult(msg lastfm.TokenResultMsg) (Model, tea.Cmd) {
	if msg.Err != nil {
		// Update popup to show error
		if pop := m.Popups.Get(popupctl.LastfmAuth); pop != nil {
			if lfm, ok := pop.(*lastfmauth.Model); ok {
				lfm.SetError(msg.Err.Error())
			}
		}
		return *m, nil
	}

	// Store token for use after user confirms authorization
	m.lastfmAuthToken = msg.Token

	// Update popup to show waiting state
	if pop := m.Popups.Get(popupctl.LastfmAuth); pop != nil {
		if lfm, ok := pop.(*lastfmauth.Model); ok {
			lfm.SetWaitingCallback()
		}
	}

	// Open browser with auth URL (desktop auth flow - no callback)
	_ = lastfm.OpenBrowser(msg.AuthURL)

	return *m, nil
}

// handleLastfmSessionResult handles the result of exchanging token for session.
func (m *Model) handleLastfmSessionResult(msg lastfm.SessionResultMsg) (Model, tea.Cmd) {
	if msg.Err != nil {
		if pop := m.Popups.Get(popupctl.LastfmAuth); pop != nil {
			if lfm, ok := pop.(*lastfmauth.Model); ok {
				lfm.SetError(msg.Err.Error())
			}
		}
		return *m, nil
	}

	// Save session to database
	if stateMgr, ok := m.StateMgr.(*state.Manager); ok {
		if err := stateMgr.SaveLastfmSession(msg.Username, msg.SessionKey); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpLastfmAuth, err))
			return *m, nil
		}
	}

	// Update model
	m.LastfmSession = &state.LastfmSession{
		Username:   msg.Username,
		SessionKey: msg.SessionKey,
		LinkedAt:   time.Now(),
	}
	m.Lastfm.SetSessionKey(msg.SessionKey)

	// Update popup
	if pop := m.Popups.Get(popupctl.LastfmAuth); pop != nil {
		if lfm, ok := pop.(*lastfmauth.Model); ok {
			lfm.SetSession(m.LastfmSession)
		}
	}

	// Start retry tick for pending scrobbles
	return *m, lastfm.RetryTickCmd()
}

// handleLastfmNowPlayingResult handles the result of now playing update.
// Non-critical errors are silently ignored as now playing is best-effort.
func (m *Model) handleLastfmNowPlayingResult(_ lastfm.NowPlayingResultMsg) (Model, tea.Cmd) {
	return *m, nil
}

// handleLastfmScrobbleResult handles the result of a scrobble submission.
func (m *Model) handleLastfmScrobbleResult(msg lastfm.ScrobbleResultMsg) (Model, tea.Cmd) {
	if msg.Err != nil {
		m.queueFailedScrobble(msg.TrackPath)
	}
	return *m, nil
}

// queueFailedScrobble adds a failed scrobble to the pending queue for retry.
func (m *Model) queueFailedScrobble(trackPath string) {
	if m.ScrobbleState == nil || m.ScrobbleState.TrackPath != trackPath {
		return
	}

	track := m.buildScrobbleTrack()
	if track == nil {
		return
	}

	stateMgr, ok := m.StateMgr.(*state.Manager)
	if !ok {
		return
	}

	_ = stateMgr.AddPendingScrobble(state.PendingScrobble{
		Artist:        track.Artist,
		Track:         track.Track,
		Album:         track.Album,
		DurationSecs:  int(track.Duration.Seconds()),
		Timestamp:     track.Timestamp,
		MBRecordingID: track.MBRecordingID,
	})
}

// handleLastfmRetryPending triggers retry of pending scrobbles.
func (m *Model) handleLastfmRetryPending() (Model, tea.Cmd) {
	if !m.isLastfmLinked() {
		return *m, nil
	}

	if stateMgr, ok := m.StateMgr.(*state.Manager); ok {
		return *m, lastfm.RetryPendingCmd(lastfm.RetryPendingParams{
			Client:   m.Lastfm,
			StateMgr: stateMgr,
		})
	}
	return *m, nil
}

// handleLastfmRetryResult handles the result of retrying pending scrobbles.
func (m *Model) handleLastfmRetryResult(_ lastfm.RetryResultMsg) (Model, tea.Cmd) {
	// Re-queue retry tick if still linked
	if m.isLastfmLinked() {
		return *m, lastfm.RetryTickCmd()
	}
	return *m, nil
}

// handleLastfmAuthAction handles actions from the Last.fm auth popup.
func (m *Model) handleLastfmAuthAction(msg lastfmauth.ActionMsg) (Model, tea.Cmd) {
	switch msg.Action { //nolint:exhaustive // ActionNone requires no handling
	case lastfmauth.ActionClose:
		m.lastfmAuthToken = "" // Clear pending token
		m.Popups.Hide(popupctl.LastfmAuth)

	case lastfmauth.ActionStartAuth:
		if m.Lastfm != nil {
			return *m, lastfm.GetTokenCmd(m.Lastfm)
		}

	case lastfmauth.ActionUnlink:
		// Delete session from database
		if stateMgr, ok := m.StateMgr.(*state.Manager); ok {
			_ = stateMgr.DeleteLastfmSession()
		}
		m.LastfmSession = nil
		// Clear session key from client but keep client
		if m.Lastfm != nil {
			m.Lastfm.SetSessionKey("")
		}
		// Update popup
		if pop := m.Popups.Get(popupctl.LastfmAuth); pop != nil {
			if lfm, ok := pop.(*lastfmauth.Model); ok {
				lfm.SetSession(nil)
			}
		}

	case lastfmauth.ActionConfirmAuth:
		// User manually confirmed they authorized - use stored token
		if m.lastfmAuthToken != "" && m.Lastfm != nil {
			token := m.lastfmAuthToken
			m.lastfmAuthToken = ""
			return *m, lastfm.GetSessionCmd(m.Lastfm, token)
		}
	}

	return *m, nil
}

// buildScrobbleTrack creates a ScrobbleTrack from the current playing track.
func (m *Model) buildScrobbleTrack() *lastfm.ScrobbleTrack {
	current := m.PlaybackService.CurrentTrack()
	if current == nil {
		return nil
	}

	info := m.PlaybackService.TrackInfo()
	if info == nil {
		return nil
	}

	track := &lastfm.ScrobbleTrack{
		Artist:   info.Artist,
		Track:    info.Title,
		Album:    info.Album,
		Duration: m.PlaybackService.Duration(),
	}

	if m.ScrobbleState != nil {
		track.Timestamp = m.ScrobbleState.StartedAt
	} else {
		track.Timestamp = time.Now()
	}

	// Use MusicBrainz recording ID if available
	if info.MBRecordingID != "" {
		track.MBRecordingID = info.MBRecordingID
	}

	// Set album artist if different from track artist
	if info.AlbumArtist != "" && info.AlbumArtist != info.Artist {
		track.AlbumArtist = info.AlbumArtist
	}

	return track
}
