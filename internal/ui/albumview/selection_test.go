package albumview

import (
	"testing"

	"github.com/llehouerou/waves/internal/core"
)

type fakeLibrary []core.AlbumEntry

func (l fakeLibrary) AllAlbums() ([]core.AlbumEntry, error) {
	return append([]core.AlbumEntry(nil), l...), nil
}

// tricky are albums whose names hold colons, the escape character of the
// saved form and its escapes, a colon only, nothing, or plain words.
var tricky = fakeLibrary{
	{AlbumArtist: "AC", Album: "DC:Back"},
	{AlbumArtist: "AC:DC", Album: "Back"},
	{AlbumArtist: "AC:DC:Back", Album: ""},
	{AlbumArtist: "", Album: "AC:DC:Back"},
	{AlbumArtist: "AC%3ADC", Album: "Back"},
	{AlbumArtist: "AC%DC", Album: "Back%25"},
	{AlbumArtist: ":", Album: ":"},
	{AlbumArtist: "::", Album: ""},
	{AlbumArtist: "", Album: "::"},
	{AlbumArtist: "", Album: ""},
	{AlbumArtist: "Björk", Album: "Homogenic"},
	{AlbumArtist: "Low", Album: "Trust"},
}

func newTrickyModel(t *testing.T) Model {
	t.Helper()
	m := New(tricky)
	m.SetSize(80, 40)
	if err := m.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	return m
}

// selectAlbum selects a in m, and checks that it is selected.
func selectAlbum(t *testing.T, m *Model, a core.AlbumEntry) {
	t.Helper()
	m.SelectAlbum(a.AlbumArtist, a.Album)
	if got := m.SelectedAlbum(); got == nil || got.AlbumArtist != a.AlbumArtist || got.Album != a.Album {
		t.Fatalf("SelectAlbum(%q, %q) selects %+v", a.AlbumArtist, a.Album, got)
	}
}

func TestSavedSelection_ReadsBackToTheSameAlbum(t *testing.T) {
	for _, a := range tricky {
		t.Run(a.AlbumArtist+" / "+a.Album, func(t *testing.T) {
			m := newTrickyModel(t)
			selectAlbum(t, &m, a)
			saved := m.SavedSelection()

			restored := newTrickyModel(t)
			restored.RestoreSelection(saved)

			got := restored.SelectedAlbum()
			if got == nil || got.AlbumArtist != a.AlbumArtist || got.Album != a.Album {
				t.Errorf("restored %q to %+v, want %q / %q", saved, got, a.AlbumArtist, a.Album)
			}
		})
	}
}

func TestSavedSelection_TwoAlbumsNeverShareIt(t *testing.T) {
	m := newTrickyModel(t)
	seen := map[string]core.AlbumEntry{"": {}}
	for _, a := range tricky {
		selectAlbum(t, &m, a)
		saved := m.SavedSelection()
		if other, ok := seen[saved]; ok {
			t.Errorf("%q / %q is saved as %q, as %q / %q is", a.AlbumArtist, a.Album, saved, other.AlbumArtist, other.Album)
		}
		seen[saved] = a
	}
}

func TestSavedSelection_NoAlbumSelectedIsEmpty(t *testing.T) {
	m := New(fakeLibrary{})
	if err := m.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := m.SavedSelection(); got != "" {
		t.Errorf("SavedSelection() = %q with no album, want empty", got)
	}
}
