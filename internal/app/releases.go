// internal/app/releases.go
package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/listenbrainz"
	"github.com/llehouerou/waves/internal/releases"
	"github.com/llehouerou/waves/internal/ui/jobbar"
)

const (
	releasesRefreshJobID = "releases-refresh"
	similarWarmupJobID   = "similar-warmup"
)

// openNewReleases opens the download popup on the new releases list and starts
// the background jobs keeping it fresh. No slskd gate: the list is a watch
// screen, it is Enter that refuses when slskd is unconfigured.
func (m Model) openNewReleases() (tea.Model, tea.Cmd) {
	filters := download.FilterConfig{
		Format:     m.Slskd.Filters.Format,
		NoSlot:     m.Slskd.Filters.NoSlot,
		TrackCount: m.Slskd.Filters.TrackCount,
		AlbumsOnly: m.MusicBrainz.AlbumsOnly,
	}
	cmds := []tea.Cmd{m.Popups.ShowDownload(m.Slskd.URL, m.Slskd.APIKey, filters, m.Library)}
	cmds = append(cmds, m.startReleasesRefresh(false), m.startSimilarWarmup())

	if dl := m.Popups.Download(); dl != nil {
		cmds = append(cmds, dl.StartReleases(download.ReleasesParams{
			Cache:       m.Releases,
			Downloads:   m.Downloads,
			Discoveries: m.SimilarArtists != nil,
			Refreshing:  m.ReleasesJob != nil,
			RefreshErr:  m.ReleasesErr,
		}))
	}
	return m, tea.Batch(cmds...)
}

// startReleasesRefresh refreshes the ListenBrainz cache in the background.
// force ignores the TTL, never the one-in-flight guard.
func (m *Model) startReleasesRefresh(force bool) tea.Cmd {
	if m.ReleasesJob != nil {
		return nil
	}
	if !force {
		if stale, err := m.Releases.IsStale(); err != nil || !stale {
			return nil
		}
	}

	m.ReleasesJob = &jobbar.Job{ID: releasesRefreshJobID, Label: "Fetching new releases"}
	m.ResizeComponents()
	cache := m.Releases
	return func() tea.Msg {
		return ReleasesRefreshedMsg{Err: cache.Refresh(context.Background(), listenbrainz.New())}
	}
}

// handleReleasesRefreshed ends the refresh job and re-reads the cache. A failed
// refresh never steals focus: the stale list stays with a discreet error line.
func (m Model) handleReleasesRefreshed(msg ReleasesRefreshedMsg) (tea.Model, tea.Cmd) {
	m.ReleasesJob = nil
	m.ResizeComponents()

	// Kept so the failure still shows on the next open, popup closed or not.
	m.ReleasesErr = ""
	if msg.Err != nil {
		m.ReleasesErr = "Could not refresh releases: " + msg.Err.Error()
	}

	if m.Popups.Download() == nil {
		return m, nil
	}
	if msg.Err != nil {
		err := msg.Err
		return m, func() tea.Msg { return download.ReleasesLoadedMsg{Refreshed: true, Err: err} }
	}
	return m, m.loadReleasesCmd(true)
}

// loadReleasesCmd re-reads the list from the cache, library and downloads.
func (m Model) loadReleasesCmd(refreshed bool) tea.Cmd {
	return download.LoadReleasesCmd(download.LoadReleasesParams{
		Cache:     m.Releases,
		Library:   m.Library,
		Downloads: m.Downloads,
		Refreshed: refreshed,
	})
}

// startSimilarWarmup fills the shared similar-artists cache. The job survives
// closing the popup and is only cancelled by quitting the app.
func (m *Model) startSimilarWarmup() tea.Cmd {
	if m.SimilarCh != nil || m.SimilarArtists == nil {
		return nil
	}

	ch := make(chan releases.WarmupProgress)
	m.SimilarCh = ch
	params := releases.WarmupParams{
		DB:       m.StateMgr.DB(),
		Client:   m.SimilarArtists,
		TTLDays:  m.RadioConfig.CacheTTLDays,
		Progress: ch,
	}
	go func() {
		// Warmup only fails on the seed query; a failed pass writes nothing
		// and is retried the next time the list opens.
		_ = releases.Warmup(params) //nolint:errcheck // retried on next open
	}()
	return m.waitForSimilarWarmup()
}

func (m Model) waitForSimilarWarmup() tea.Cmd {
	return waitForChannel(m.SimilarCh, func(progress releases.WarmupProgress, ok bool) tea.Msg {
		if !ok {
			return SimilarWarmupMsg{Done: true}
		}
		return SimilarWarmupMsg{Progress: progress}
	})
}

// handleSimilarWarmup updates the job bar and re-reads the list when done.
func (m Model) handleSimilarWarmup(msg SimilarWarmupMsg) (tea.Model, tea.Cmd) {
	if msg.Done {
		m.SimilarCh = nil
		m.SimilarJob = nil
		m.ResizeComponents()
		if m.Popups.Download() == nil {
			return m, nil
		}
		return m, m.loadReleasesCmd(false)
	}

	appearing := m.SimilarJob == nil
	m.SimilarJob = &jobbar.Job{
		ID:      similarWarmupJobID,
		Label:   "Finding similar artists",
		Current: msg.Progress.Current,
		Total:   msg.Progress.Total,
	}
	if appearing {
		m.ResizeComponents()
	}
	return m, m.waitForSimilarWarmup()
}
