package core

import (
	"reflect"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/albumpreset"
)

// presetNames are the names of the album presets, in the list's order.
func presetNames(t *testing.T, c *Core) []string {
	t.Helper()
	presets, err := c.ClientState().ListAlbumPresets()
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	names := make([]string, len(presets))
	for i := range presets {
		names[i] = presets[i].Name
	}
	return names
}

func TestClientState_AlbumPresetsAreSavedListedAndDeleted(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	kept := c.ClientState()

	byArtist := albumpreset.Settings{
		GroupFields:  []albumpreset.GroupField{albumpreset.GroupFieldArtist},
		SortCriteria: []albumpreset.SortCriterion{{Field: albumpreset.SortFieldAlbum, Order: albumpreset.SortAsc}},
	}
	id, err := kept.SaveAlbumPreset("By artist", byArtist)
	if err != nil {
		t.Fatalf("save preset: %v", err)
	}

	// A new database comes with two presets.
	if got, want := presetNames(t, c), []string{"By artist", "Newly added", "Newly released"}; !slices.Equal(got, want) {
		t.Fatalf("presets = %v, want %v", got, want)
	}
	presets, err := kept.ListAlbumPresets()
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	if got := presets[0]; got.ID != id || !reflect.DeepEqual(got.Settings, byArtist) {
		t.Errorf("By artist = %+v, want it as it was saved, with ID %d", got, id)
	}

	if err := kept.DeleteAlbumPreset(id); err != nil {
		t.Fatalf("delete preset: %v", err)
	}
	if got, want := presetNames(t, c), []string{"Newly added", "Newly released"}; !slices.Equal(got, want) {
		t.Errorf("presets = %v, want %v", got, want)
	}
}

func TestClientState_AFirstRunHasNoNavigationState(t *testing.T) {
	c, _ := newTestCore(t, Options{})

	if nav, err := c.ClientState().GetNavigation(); err != nil || nav != nil {
		t.Errorf("navigation state = %+v (err %v), want none", nav, err)
	}
}
