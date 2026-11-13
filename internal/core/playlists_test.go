package core

import (
	"testing"
)

// lowTrackIDs returns the library IDs of Low's tracks, in album order.
func lowTrackIDs(t *testing.T, c *Core) []int64 {
	t.Helper()
	tracks, err := c.Library().Tracks("Low", "Album")
	if err != nil {
		t.Fatalf("library tracks: %v", err)
	}
	ids := make([]int64, len(tracks))
	for i := range tracks {
		ids[i] = tracks[i].ID
	}
	return ids
}

func TestPlaylists_TracksOfAPlaylistGoStraightIntoTheQueue(t *testing.T) {
	mgr := openTestState(t)
	addLibraryTracks(t, mgr, "Low", 2)
	c, _ := newTestCore(t, Options{state: mgr})
	pls := c.Playlists()

	id, err := pls.Create(nil, "Mix")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ids := lowTrackIDs(t, c)
	if err := pls.AddTracks(id, ids); err != nil {
		t.Fatalf("add tracks: %v", err)
	}

	tracks, err := pls.Tracks(id)
	if err != nil {
		t.Fatalf("tracks: %v", err)
	}
	c.Playback().AddTracks(tracks...)

	queue := c.Playback().QueueTracks()
	if len(queue) != 2 || queue[0].ID != ids[0] || queue[1].Title != "Low 2" || queue[1].Path != "/lib/Low/2.mp3" {
		t.Errorf("queue = %+v, want Low's two tracks", queue)
	}
}

func TestPlaylists_ToggledFavoritesAreInTheFavoritesPlaylist(t *testing.T) {
	mgr := openTestState(t)
	addLibraryTracks(t, mgr, "Low", 2)
	c, _ := newTestCore(t, Options{state: mgr})
	pls := c.Playlists()
	ids := lowTrackIDs(t, c)

	now, err := pls.ToggleFavorites(ids[:1])
	if err != nil || !now[ids[0]] {
		t.Fatalf("toggle = %v (err %v), want the track favorited", now, err)
	}

	favorites, err := pls.FavoriteTrackIDs()
	if err != nil || len(favorites) != 1 || !favorites[ids[0]] {
		t.Errorf("favorites = %v (err %v), want the first track only", favorites, err)
	}
	lists, err := pls.List(nil)
	if err != nil || len(lists) == 0 || !IsFavoritesPlaylist(lists[0].ID) {
		t.Fatalf("playlists = %+v (err %v), want Favorites first", lists, err)
	}
	if tracks, err := pls.Tracks(lists[0].ID); err != nil || len(tracks) != 1 || tracks[0].ID != ids[0] {
		t.Errorf("Favorites tracks = %+v (err %v), want the first track", tracks, err)
	}
}
