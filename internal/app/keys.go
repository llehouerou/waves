// internal/app/keys.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/search"
)

// handleGPrefixKey handles 'g' key to start a key sequence.
// Works from both navigator and queue panel focus (for deep search).
func (m *Model) handleGPrefixKey(key string) (bool, tea.Cmd) {
	if key == "g" {
		m.Input.StartKeySequence("g")
		return true, nil
	}
	return false, nil
}

// handleGSequence handles key sequences starting with 'g'.
func (m Model) handleGSequence(key string) (tea.Model, tea.Cmd) {
	m.Input.ClearKeySequence()

	switch key {
	case "f":
		// Deep search in file browser, library, or playlists
		switch m.Navigation.ViewMode() {
		case ViewFileBrowser:
			currentPath := m.Navigation.FileNav().CurrentPath()
			m.Input.StartDeepSearch(context.Background(), func(ctx context.Context) <-chan navigator.ScanResult {
				return navigator.ScanDir(ctx, currentPath)
			})
			return m, m.waitForScan()
		case ViewLibrary:
			// Use FTS-backed search function
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
			return m, nil
		case ViewPlaylists:
			m.Input.StartDeepSearchWithItems(m.AllPlaylistSearchItems())
			return m, nil
		case ViewDownload:
			// Download view has built-in search, no deep search needed
			return m, nil
		}
	case "p":
		// Open library sources popup
		if m.Navigation.ViewMode() == ViewLibrary {
			sources, err := m.Library.Sources()
			if err != nil {
				m.Popups.ShowError(err.Error())
				return m, nil
			}
			m.Popups.ShowLibrarySources(sources)
			return m, nil
		}
	case "r":
		// Incremental library refresh
		if m.Navigation.ViewMode() == ViewLibrary {
			cmd := m.startLibraryScan(m.Library.Refresh)
			return m, cmd
		}
	case "R":
		// Full library rescan
		if m.Navigation.ViewMode() == ViewLibrary {
			cmd := m.startLibraryScan(m.Library.FullRefresh)
			return m, cmd
		}
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
