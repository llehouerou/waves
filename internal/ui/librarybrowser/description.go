package librarybrowser

import (
	"fmt"
	"strings"

	"github.com/llehouerou/waves/internal/ui/styles"
)

// descriptionHeight is the fixed inner height of the description panel (lines of content).
const descriptionHeight = 4

// renderDescription renders the contextual description panel below the columns.
func (m Model) renderDescription() string {
	t := styles.T()

	var lines []string
	switch m.activeColumn {
	case ColumnArtists:
		lines = m.describeArtist()
	case ColumnAlbums:
		lines = m.describeAlbum()
	case ColumnTracks:
		lines = m.describeTrack()
	}

	// Pad to fixed height
	for len(lines) < descriptionHeight {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	descWidth := m.width - 2 // account for border

	return styles.PanelStyle(m.focused).
		Width(descWidth).
		Foreground(t.FgBase).
		Render(content)
}

// describeArtist returns description lines when an artist is selected.
func (m Model) describeArtist() []string {
	artist := m.SelectedArtist()
	if artist == "" {
		return nil
	}

	t := styles.T()

	albumCount := len(m.albums)
	info := t.S().Base.Render(artist) +
		t.S().Muted.Render(fmt.Sprintf(" \u00b7 %d albums", albumCount))

	shortcuts := shortcut(t, "i", "similar artists") +
		shortcut(t, "a", "add to queue") +
		shortcut(t, "Enter", "browse")

	return []string{info, shortcuts}
}

// describeAlbum returns description lines when an album is selected.
func (m Model) describeAlbum() []string {
	album := m.SelectedAlbum()
	if album == nil {
		return nil
	}

	t := styles.T()

	name := album.Name
	if album.Year > 0 {
		name = fmt.Sprintf("%s (%d)", name, album.Year)
	}

	trackCount := len(m.tracks)
	info := t.S().Base.Render(name) +
		t.S().Muted.Render(fmt.Sprintf(" \u00b7 %d tracks", trackCount))

	// Show genre from the first track if available
	var genreLine string
	if len(m.tracks) > 0 && m.tracks[0].Genre != "" {
		genreLine = t.S().Muted.Render("Genre: ") + t.S().Base.Render(m.tracks[0].Genre)
	}

	shortcuts := shortcut(t, "a", "add to queue") +
		shortcut(t, "Enter", "play") +
		shortcut(t, "t", "retag")

	lines := []string{info}
	if genreLine != "" {
		lines = append(lines, genreLine)
	}
	lines = append(lines, shortcuts)

	return lines
}

// describeTrack returns description lines when a track is selected.
func (m Model) describeTrack() []string {
	track := m.SelectedTrack()
	if track == nil {
		return nil
	}

	t := styles.T()

	titleLine := t.S().Base.Render(track.Title)

	albumName := track.Album
	if track.Year > 0 {
		albumName = fmt.Sprintf("%s (%d)", albumName, track.Year)
	}

	details := t.S().Muted.Render(albumName)
	details += t.S().Muted.Render(fmt.Sprintf(" \u00b7 Track %d", track.TrackNumber))
	if track.Genre != "" {
		details += t.S().Muted.Render(" \u00b7 Genre: ") + t.S().Base.Render(track.Genre)
	}

	shortcuts := shortcut(t, "F", "favorite") +
		shortcut(t, "a", "add to queue") +
		shortcut(t, "Enter", "play") +
		shortcut(t, "d", "delete")

	return []string{titleLine, details, shortcuts}
}

// shortcut renders a key-description pair for the description panel.
func shortcut(t *styles.Theme, key, desc string) string {
	return t.S().Title.Render(key) + " " + t.S().Muted.Render(desc) + "  "
}
