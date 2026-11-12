// internal/app/handlers_export.go
package app

import (
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/librarynav"
)

// handleExportKey handles the 'e' key to open the export popup.
func (m *Model) handleExportKey(key string) handler.Result {
	action := m.Keys.Resolve(key)
	if action != keymap.ActionExport {
		return handler.NotHandled
	}

	// Check if we're in a context that supports export
	if !m.canExport() {
		return handler.NotHandled
	}

	// Collect tracks to export
	tracks, albumName := m.collectExportTracks()
	if len(tracks) == 0 {
		return handler.NotHandled
	}

	// Show export popup
	cmd := m.Popups.ShowExport(m.Core.Exports())
	if exp := m.Popups.Export(); exp != nil {
		exp.SetTracks(tracks, albumName)
	}
	return handler.Handled(cmd)
}

// canExport returns true if export is available in the current context.
func (m *Model) canExport() bool {
	// Can export from library navigator
	if m.Navigation.ViewMode() == navctl.ViewLibrary && m.Navigation.IsNavigatorFocused() {
		return true
	}
	// Can export from queue
	if m.Navigation.IsQueueFocused() {
		return true
	}
	return false
}

// collectExportTracks collects tracks for export based on current selection.
//
//nolint:nestif // Complex but clear structure for collecting tracks from different sources
func (m *Model) collectExportTracks() (tracks []core.ExportTrack, albumName string) {
	var trackIDs []int64

	if m.Navigation.IsQueueFocused() {
		// Export from queue
		queueTracks := m.Core.Playback().QueueTracks()
		for _, t := range queueTracks {
			trackIDs = append(trackIDs, t.ID)
		}
		albumName = "Queue"
	} else if m.Navigation.ViewMode() == navctl.ViewLibrary {
		// Export from library navigator
		switch m.Navigation.LibrarySubMode() {
		case navctl.LibraryModeAlbum:
			album := m.Navigation.AlbumView().SelectedAlbum()
			if album != nil {
				ids, err := m.Core.Library().AlbumTrackIDs(album.AlbumArtist, album.Album)
				if err == nil {
					trackIDs = ids
				}
				albumName = album.Album
			}
		case navctl.LibraryModeBrowser:
			trackIDs, albumName = m.collectBrowserExportTracks()
		case navctl.LibraryModeMiller:
			selected := m.Navigation.LibraryNav().Selected()
			if selected != nil {
				trackIDs = m.collectTrackIDsFromNode(selected)
				albumName = selected.DisplayName()
			}
		}
	}

	// The core turns them into what an export takes: files, names, discs.
	return m.Core.Exports().Tracks(trackIDs), albumName
}

// collectBrowserExportTracks collects track IDs from the library browser based on active column.
func (m *Model) collectBrowserExportTracks() (trackIDs []int64, albumName string) {
	browser := m.Navigation.LibraryBrowser()
	switch browser.ActiveColumn() {
	case librarybrowser.ColumnArtists:
		artist := browser.SelectedArtist()
		if artist == "" {
			return nil, ""
		}
		tracks, err := m.Core.Library().ArtistTracks(artist)
		if err != nil {
			return nil, ""
		}
		ids := make([]int64, len(tracks))
		for i := range tracks {
			ids[i] = tracks[i].ID
		}
		return ids, artist
	case librarybrowser.ColumnAlbums:
		artist := browser.SelectedArtist()
		album := browser.SelectedAlbum()
		if artist == "" || album == nil {
			return nil, ""
		}
		ids, err := m.Core.Library().AlbumTrackIDs(artist, album.Name)
		if err != nil {
			return nil, ""
		}
		return ids, album.Name
	case librarybrowser.ColumnTracks:
		track := browser.SelectedTrack()
		if track == nil {
			return nil, ""
		}
		return []int64{track.ID}, track.Title
	}
	return nil, ""
}

// collectTrackIDsFromNode collects track IDs from a library node.
func (m *Model) collectTrackIDsFromNode(node *librarynav.Node) []int64 {
	switch node.Level() {
	case librarynav.LevelRoot:
		return nil
	case librarynav.LevelArtist:
		tracks, err := m.Core.Library().ArtistTracks(node.Artist())
		if err != nil {
			return nil
		}
		ids := make([]int64, len(tracks))
		for i := range tracks {
			ids[i] = tracks[i].ID
		}
		return ids
	case librarynav.LevelAlbum:
		ids, _ := m.Core.Library().AlbumTrackIDs(node.Artist(), node.Album())
		return ids
	case librarynav.LevelTrack:
		if track := node.Track(); track != nil {
			return []int64{track.ID}
		}
	}
	return nil
}

// handleExportEvent takes an event of an export the core runs: the job bar
// shows the export for as long as it runs, and its end is told.
func (m *Model) handleExportEvent(e core.Event) tea.Cmd {
	switch e := e.(type) {
	case core.ExportProgressed:
		i := slices.IndexFunc(m.exports, func(known core.ExportProgressed) bool { return known.ID == e.ID })
		if i >= 0 {
			m.exports[i] = e
			return nil
		}
		m.exports = append(m.exports, e)
		m.ResizeComponents() // the job bar takes one more line
	case core.ExportFinished:
		m.exports = slices.DeleteFunc(m.exports, func(known core.ExportProgressed) bool { return known.ID == e.ID })
		m.ResizeComponents()
		return m.tellExportEnd(&e)
	}
	return nil
}

// tellExportEnd says how an export ended: an error popup when tracks failed
// or the export stopped on the way, a notification that goes away otherwise.
func (m *Model) tellExportEnd(e *core.ExportFinished) tea.Cmd {
	failed := len(e.Failed)
	switch {
	case e.Err != nil:
		m.Popups.ShowError(fmt.Sprintf("Export to %s stopped after %d of %d tracks: %v",
			e.Target, e.Done-failed, e.Total, e.Err))
		return nil
	case failed > 0 && failed < e.Total:
		m.Popups.ShowError(fmt.Sprintf("Export: %d/%d failed - %v", failed, e.Total, e.Failed[0].Err))
		return nil
	case failed > 0:
		m.Popups.ShowError(fmt.Sprintf("Export failed: %v", e.Failed[0].Err))
		return nil
	}

	var told string
	switch {
	case e.Artist != "" && e.Album != "":
		told = fmt.Sprintf("%s - %s → %s (%d tracks)", e.Artist, e.Album, e.Target, e.Total)
	case e.Artist != "":
		told = fmt.Sprintf("%s → %s (%d tracks)", e.Artist, e.Target, e.Total)
	default:
		told = fmt.Sprintf("Exported %d tracks → %s", e.Total, e.Target)
	}
	m.nextNotificationID++
	id := m.nextNotificationID
	m.Notifications = append(m.Notifications, Notification{ID: id, Message: told})
	m.ResizeComponents()
	return NotificationClearCmd(id)
}

// exportJobs are the exports the core runs, as the job bar shows them.
func (m *Model) exportJobs() []jobbar.Job {
	jobs := make([]jobbar.Job, len(m.exports))
	for i := range m.exports {
		e := &m.exports[i]
		label := "Exporting → " + e.Target
		switch {
		case e.Artist != "" && e.Album != "":
			label = fmt.Sprintf("%s - %s → %s", e.Artist, e.Album, e.Target)
		case e.Artist != "":
			label = fmt.Sprintf("%s → %s", e.Artist, e.Target)
		}
		jobs[i] = jobbar.Job{ID: fmt.Sprintf("export-%d", e.ID), Label: label, Current: e.Done, Total: e.Total}
	}
	return jobs
}
