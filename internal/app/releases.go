// internal/app/releases.go
package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/listenbrainz"
	"github.com/llehouerou/waves/internal/releases"
)

// openNewReleases opens the download popup on the new releases list and starts
// the background work keeping it fresh. That work stays out of the job bar: it
// is the list's own business, said with one discreet line inside the popup.
// No slskd gate either: it is Enter that refuses when slskd is unconfigured.
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
			Refreshing:  m.ReleasesRefreshing,
			RefreshErr:  m.ReleasesErr,
		}))
	}
	return m, tea.Batch(cmds...)
}

// startReleasesRefresh refreshes the ListenBrainz cache in the background.
// force ignores the TTL, never the one-in-flight guard.
func (m *Model) startReleasesRefresh(force bool) tea.Cmd {
	if m.ReleasesRefreshing {
		return nil
	}
	if !force {
		if stale, err := m.Releases.IsStale(); err != nil || !stale {
			return nil
		}
	}

	m.ReleasesRefreshing = true
	cache := m.Releases
	return func() tea.Msg {
		return ReleasesRefreshedMsg{Err: cache.Refresh(context.Background(), listenbrainz.New())}
	}
}

// handleReleasesRefreshed ends the refresh and re-reads the cache. A failed
// refresh never steals focus: the stale list stays with a discreet error line.
func (m Model) handleReleasesRefreshed(msg ReleasesRefreshedMsg) (tea.Model, tea.Cmd) {
	m.ReleasesRefreshing = false

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
	return waitForChannel(m.SimilarCh, func(_ releases.WarmupProgress, ok bool) tea.Msg {
		return SimilarWarmupMsg{Done: !ok}
	})
}

// handleSimilarWarmup re-reads the list when the warm-up ends. Progress is not
// reported anywhere: it fills a cache nobody asked to watch.
func (m Model) handleSimilarWarmup(msg SimilarWarmupMsg) (tea.Model, tea.Cmd) {
	if !msg.Done {
		return m, m.waitForSimilarWarmup()
	}

	m.SimilarCh = nil
	if m.Popups.Download() == nil {
		return m, nil
	}
	return m, m.loadReleasesCmd(false)
}
