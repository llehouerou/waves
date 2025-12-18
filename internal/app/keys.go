// internal/app/keys.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/search"
)

// handleFPrefixKey handles 'f' key to start a key sequence.
// Works from both navigator and queue panel focus (for deep search).
func (m *Model) handleFPrefixKey(key string) handler.Result {
	if m.Keys.Resolve(key) == keymap.ActionFPrefix {
		m.Input.StartKeySequence("f")
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// handleFSequence handles key sequences starting with 'f'.
func (m Model) handleFSequence(key string) (tea.Model, tea.Cmd) {
	m.Input.ClearKeySequence()

	// Resolve as "f <key>" sequence
	seqKey := "f " + key
	switch m.Keys.Resolve(seqKey) { //nolint:exhaustive // only handling f-sequence actions
	case keymap.ActionDeepSearch:
		// Deep search in file browser, library, or playlists
		switch m.Navigation.ViewMode() {
		case ViewFileBrowser:
			currentPath := m.Navigation.FileNav().CurrentPath()
			m.Input.StartDeepSearch(context.Background(), func(ctx context.Context) <-chan navigator.ScanResult {
				return navigator.ScanDir(ctx, currentPath)
			})
			return m, m.waitForScan()
		case ViewLibrary:
			// In album view, search albums only
			if m.Navigation.IsAlbumViewActive() {
				searchFn := func(query string) ([]search.Item, error) {
					results, err := m.Library.SearchAlbumsFTS(query)
					if err != nil {
						return nil, err
					}
					items := make([]search.Item, len(results))
					for i, r := range results {
						items[i] = library.SearchItem{Result: r}
					}
					return items, nil
				}
				m.Input.StartDeepSearchWithFunc(searchFn)
			} else {
				// Use FTS-backed search function for all types
				searchFn := func(query string) ([]search.Item, error) {
					results, err := m.Library.SearchFTS(query)
					if err != nil {
						return nil, err
					}
					items := make([]search.Item, len(results))
					for i, r := range results {
						items[i] = library.SearchItem{Result: r}
					}
					return items, nil
				}
				m.Input.StartDeepSearchWithFunc(searchFn)
			}
			return m, nil
		case ViewPlaylists:
			m.Input.StartDeepSearchWithItems(m.AllPlaylistSearchItems())
			return m, nil
		case ViewDownloads:
			// No deep search for downloads view
		}
	case keymap.ActionLibrarySources:
		// Open library sources popup
		if m.Navigation.ViewMode() == ViewLibrary {
			sources, err := m.Library.Sources()
			if err != nil {
				m.Popups.ShowError(errmsg.Format(errmsg.OpSourceLoad, err))
				return m, nil
			}
			m.Popups.ShowLibrarySources(sources)
			return m, nil
		}
	case keymap.ActionRefreshLibrary:
		// Incremental library refresh
		if m.Navigation.ViewMode() == ViewLibrary {
			cmd := m.startLibraryScan(m.Library.Refresh)
			return m, cmd
		}
	case keymap.ActionFullRescan:
		// Full library rescan
		if m.Navigation.ViewMode() == ViewLibrary {
			cmd := m.startLibraryScan(m.Library.FullRefresh)
			return m, cmd
		}
	case keymap.ActionDownloadSoulseek:
		// Open download popup (requires slskd config)
		if m.HasSlskdConfig {
			filters := download.FilterConfig{
				Format:     m.Slskd.Filters.Format,
				NoSlot:     m.Slskd.Filters.NoSlot,
				TrackCount: m.Slskd.Filters.TrackCount,
				AlbumsOnly: m.MusicBrainz.AlbumsOnly,
			}
			cmd := m.Popups.ShowDownload(m.Slskd.URL, m.Slskd.APIKey, filters)
			return m, cmd
		}
	case keymap.ActionLastfmSettings:
		// Open Last.fm settings popup (requires lastfm config)
		if m.HasLastfmConfig {
			cmd := m.Popups.ShowLastfmAuth(m.LastfmSession)
			return m, cmd
		}
	}

	return m, nil
}

// handleOPrefixKey handles 'o' key to start a key sequence in album view.
func (m *Model) handleOPrefixKey(key string) handler.Result {
	if m.Keys.Resolve(key) == keymap.ActionOPrefix && m.Navigation.IsAlbumViewActive() && m.Navigation.IsNavigatorFocused() {
		m.Input.StartKeySequence("o")
		return handler.HandledNoCmd
	}
	return handler.NotHandled
}

// handleOSequence handles key sequences starting with 'o' (album view options).
func (m Model) handleOSequence(key string) (tea.Model, tea.Cmd) {
	m.Input.ClearKeySequence()

	// Only handle in album view
	if !m.Navigation.IsAlbumViewActive() {
		return m, nil
	}

	av := m.Navigation.AlbumView()
	settings := av.Settings()

	// Resolve as "o <key>" sequence
	seqKey := "o " + key
	switch m.Keys.Resolve(seqKey) { //nolint:exhaustive // only handling o-sequence actions
	case keymap.ActionAlbumGrouping:
		// Show grouping popup
		cmd := m.Popups.ShowAlbumGrouping(settings.GroupFields, settings.GroupSortOrder, settings.GroupDateField)
		return m, cmd
	case keymap.ActionAlbumSorting:
		// Show sorting popup
		cmd := m.Popups.ShowAlbumSorting(settings.SortCriteria)
		return m, cmd
	case keymap.ActionAlbumPresets:
		// Show presets popup
		presets, err := m.StateMgr.ListAlbumPresets()
		if err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPresetLoad, err))
			return m, nil
		}
		cmd := m.Popups.ShowAlbumPresets(presets, settings)
		return m, cmd
	}

	return m, nil
}

// handleSeek handles seek operations with debouncing.
func (m *Model) handleSeek(seconds int) {
	if time.Since(m.LastSeekTime) < 150*time.Millisecond {
		return
	}
	m.LastSeekTime = time.Now()
	m.Playback.Seek(time.Duration(seconds) * time.Second)
}
