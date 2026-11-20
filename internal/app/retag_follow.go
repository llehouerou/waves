package app

import (
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/librarynav"
)

// millerPlace is where the Miller columns are on an album.
type millerPlace int

const (
	millerOff    millerPlace = iota // elsewhere
	millerOnRow                     // inside the album's artist, on the album's row
	millerInside                    // inside the album
)

// albumPlaces is where the views of the Library view were on an album, read
// before the library is read again: a retag that renames the album leaves
// none of them on it, and each follows it from what it was on.
type albumPlaces struct {
	albumView bool // the album view is on screen with its cursor on the album

	browser       bool // the library browser selects the album
	browserColumn librarybrowser.Column
	browserTrack  int64 // the track the browser selects, 0 for none

	miller      millerPlace
	millerTrack string // the node the cursor is on inside the album
}

// placesOnAlbum reads which views are on the album albumArtist - album.
func (m *Model) placesOnAlbum(albumArtist, album string) albumPlaces {
	browser := m.Navigation.LibraryBrowser()
	was := albumPlaces{
		albumView:     m.albumViewOn(albumArtist, album),
		browser:       m.browserOn(albumArtist, album),
		browserColumn: browser.ActiveColumn(),
		miller:        m.millerOn(albumArtist, album),
	}
	if track := browser.SelectedTrack(); track != nil {
		was.browserTrack = track.ID
	}
	if was.miller == millerInside {
		was.millerTrack = m.Navigation.LibraryNav().SelectedID()
	}
	return was
}

// albumViewOn tells whether the album view is on screen with its cursor on
// the album albumArtist - album.
func (m *Model) albumViewOn(albumArtist, album string) bool {
	if !m.Navigation.IsAlbumViewActive() {
		return false
	}
	selected := m.Navigation.AlbumView().SelectedAlbum()
	return selected != nil && selected.AlbumArtist == albumArtist && selected.Album == album
}

// browserOn tells whether the library browser selects the album
// albumArtist - album.
func (m *Model) browserOn(albumArtist, album string) bool {
	browser := m.Navigation.LibraryBrowser()
	selected := browser.SelectedAlbum()
	return selected != nil && browser.SelectedArtist() == albumArtist && selected.Name == album
}

// millerOn tells where the Miller columns are on the album
// albumArtist - album.
func (m *Model) millerOn(albumArtist, album string) millerPlace {
	nav := m.Navigation.LibraryNav()
	switch nav.Current().ID() {
	case librarynav.AlbumNodeID(albumArtist, album):
		return millerInside
	case librarynav.ArtistNodeID(albumArtist):
		if nav.SelectedID() == librarynav.AlbumNodeID(albumArtist, album) {
			return millerOnRow
		}
	}
	return millerOff
}

// followRetaggedAlbum puts each view that was on the album retagged before
// the library was read again (was) on that album under the names the library
// now has it under, when the retag renamed it: the old names are gone. A view
// still on the album (it kept its names, or files that failed kept them), on
// anything else, or with no album named by the library to follow stays where
// the refresh left it. The library named the album once it read the files
// again: it holds it.
func (m *Model) followRetaggedAlbum(was albumPlaces, finished core.RetagFinished) {
	albumArtist, album := finished.LibraryAlbumArtist, finished.LibraryAlbum
	if albumArtist == "" && album == "" {
		return
	}
	if was.albumView && !m.albumViewOn(finished.AlbumArtist, finished.Album) {
		m.Navigation.AlbumView().SelectAlbum(albumArtist, album)
	}
	if was.browser && !m.browserOn(finished.AlbumArtist, finished.Album) {
		m.selectBrowserAlbum(albumArtist, album, was.browserColumn, was.browserTrack)
	}
	if was.miller != millerOff && m.millerOn(finished.AlbumArtist, finished.Album) != was.miller {
		nav := m.Navigation.LibraryNav()
		if was.miller == millerOnRow {
			nav.FocusByID(librarynav.AlbumNodeID(albumArtist, album))
		} else {
			nav.NavigateTo(librarynav.AlbumNodeID(albumArtist, album))
			nav.SelectByID(was.millerTrack)
		}
	}
}
