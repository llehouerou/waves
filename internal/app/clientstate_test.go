package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/albumpreset"
	"github.com/llehouerou/waves/internal/ui/albumview"
)

// The TUI holds no database: what it keeps between two runs, the core keeps
// for it.
func TestAlbumPresets_AreKeptThroughTheCore(t *testing.T) {
	m := newLibraryTestModel(t, nil)
	m.StateMgr = nil

	m.handleAlbumPresetsAction(albumview.PresetSaved{Name: "By artist", Settings: albumpreset.DefaultSettings()})

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Fatalf("saving the preset failed: %s", msg)
	}
	presets, err := m.Core.ClientState().ListAlbumPresets()
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	var id int64
	for _, p := range presets {
		if p.Name == "By artist" {
			id = p.ID
		}
	}
	if id == 0 {
		t.Fatalf("presets = %+v, want By artist among them", presets)
	}

	m.handleAlbumPresetsAction(albumview.PresetDeleted{ID: id})

	presets, err = m.Core.ClientState().ListAlbumPresets()
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	for _, p := range presets {
		if p.Name == "By artist" {
			t.Errorf("presets = %+v, want By artist deleted", presets)
		}
	}
}
