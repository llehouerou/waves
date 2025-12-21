package library

import (
	"database/sql"
	"errors"
	"testing"
)

func TestArtists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Empty library
	artists, err := lib.Artists()
	if err != nil {
		t.Fatalf("Artists failed: %v", err)
	}
	if len(artists) != 0 {
		t.Errorf("expected 0 artists, got %d", len(artists))
	}

	// Insert tracks with different album artists
	_, err = db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/beatles/abbey/track1.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Come Together', 1, 1, 1969, 1000, 1000),
			('/music/beatles/abbey/track2.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Something', 2, 1, 1969, 1000, 1000),
			('/music/pink/wall/track1.mp3', 1000, 'Pink Floyd', 'Pink Floyd', 'The Wall', 'Another Brick', 1, 1, 1979, 1000, 1000),
			('/music/zeppelin/iv/track1.mp3', 1000, 'Led Zeppelin', 'Led Zeppelin', 'IV', 'Stairway', 1, 1, 1971, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	artists, err = lib.Artists()
	if err != nil {
		t.Fatalf("Artists failed: %v", err)
	}
	if len(artists) != 3 {
		t.Errorf("expected 3 artists, got %d", len(artists))
	}

	// Should be sorted case-insensitively
	expected := []string{"Led Zeppelin", "Pink Floyd", "The Beatles"}
	for i, artist := range artists {
		if artist != expected[i] {
			t.Errorf("artist[%d] = %s, expected %s", i, artist, expected[i])
		}
	}
}

func TestAlbums(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert tracks for an artist with multiple albums
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/beatles/abbey/track1.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Come Together', 1, 1, 1969, 1000, 1000),
			('/music/beatles/revolver/track1.mp3', 1000, 'The Beatles', 'The Beatles', 'Revolver', 'Taxman', 1, 1, 1966, 1000, 1000),
			('/music/beatles/white/track1.mp3', 1000, 'The Beatles', 'The Beatles', 'White Album', 'Back in USSR', 1, 1, 1968, 1000, 1000),
			('/music/beatles/noyear/track1.mp3', 1000, 'The Beatles', 'The Beatles', 'No Year Album', 'Unknown', 1, 1, 0, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	albums, err := lib.Albums("The Beatles")
	if err != nil {
		t.Fatalf("Albums failed: %v", err)
	}
	if len(albums) != 4 {
		t.Errorf("expected 4 albums, got %d", len(albums))
	}

	// Albums should be sorted by year (nulls/zeros last), then by name
	// 1966 Revolver, 1968 White Album, 1969 Abbey Road, 0 No Year Album
	expectedOrder := []struct {
		name string
		year int
	}{
		{"Revolver", 1966},
		{"White Album", 1968},
		{"Abbey Road", 1969},
		{"No Year Album", 0},
	}

	for i, album := range albums {
		if album.Name != expectedOrder[i].name {
			t.Errorf("album[%d].Name = %s, expected %s", i, album.Name, expectedOrder[i].name)
		}
		if album.Year != expectedOrder[i].year {
			t.Errorf("album[%d].Year = %d, expected %d", i, album.Year, expectedOrder[i].year)
		}
	}

	// Non-existent artist should return empty
	albums, err = lib.Albums("Non Existent")
	if err != nil {
		t.Fatalf("Albums failed: %v", err)
	}
	if len(albums) != 0 {
		t.Errorf("expected 0 albums for non-existent artist, got %d", len(albums))
	}
}

func TestTracks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert tracks for an album
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, genre, added_at, updated_at)
		VALUES
			('/music/beatles/abbey/track3.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Maxwell', 3, 1, 1969, 'Rock', 1000, 1000),
			('/music/beatles/abbey/track1.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Come Together', 1, 1, 1969, 'Rock', 1000, 1000),
			('/music/beatles/abbey/track2.mp3', 1000, 'The Beatles', 'The Beatles', 'Abbey Road', 'Something', 2, 1, 1969, 'Rock', 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	tracks, err := lib.Tracks("The Beatles", "Abbey Road")
	if err != nil {
		t.Fatalf("Tracks failed: %v", err)
	}
	if len(tracks) != 3 {
		t.Errorf("expected 3 tracks, got %d", len(tracks))
	}

	// Should be sorted by disc_number, track_number
	if tracks[0].Title != "Come Together" || tracks[0].TrackNumber != 1 {
		t.Errorf("expected first track to be 'Come Together' (1), got %s (%d)", tracks[0].Title, tracks[0].TrackNumber)
	}
	if tracks[1].Title != "Something" || tracks[1].TrackNumber != 2 {
		t.Errorf("expected second track to be 'Something' (2), got %s (%d)", tracks[1].Title, tracks[1].TrackNumber)
	}
	if tracks[2].Title != "Maxwell" || tracks[2].TrackNumber != 3 {
		t.Errorf("expected third track to be 'Maxwell' (3), got %s (%d)", tracks[2].Title, tracks[2].TrackNumber)
	}

	// Verify all fields are populated
	if tracks[0].Genre != "Rock" {
		t.Errorf("expected genre 'Rock', got %s", tracks[0].Genre)
	}
	if tracks[0].Year != 1969 {
		t.Errorf("expected year 1969, got %d", tracks[0].Year)
	}
}

func TestTracks_MultiDisc(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert multi-disc album
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/wall/d2t1.mp3', 1000, 'Pink Floyd', 'Pink Floyd', 'The Wall', 'Hey You', 1, 2, 1979, 1000, 1000),
			('/music/wall/d1t1.mp3', 1000, 'Pink Floyd', 'Pink Floyd', 'The Wall', 'In the Flesh?', 1, 1, 1979, 1000, 1000),
			('/music/wall/d1t2.mp3', 1000, 'Pink Floyd', 'Pink Floyd', 'The Wall', 'The Thin Ice', 2, 1, 1979, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	tracks, err := lib.Tracks("Pink Floyd", "The Wall")
	if err != nil {
		t.Fatalf("Tracks failed: %v", err)
	}

	// Should be sorted by disc, then track
	expected := []struct {
		disc  int
		track int
		title string
	}{
		{1, 1, "In the Flesh?"},
		{1, 2, "The Thin Ice"},
		{2, 1, "Hey You"},
	}

	for i, track := range tracks {
		if track.DiscNumber != expected[i].disc || track.TrackNumber != expected[i].track {
			t.Errorf("track[%d] = disc %d track %d, expected disc %d track %d",
				i, track.DiscNumber, track.TrackNumber, expected[i].disc, expected[i].track)
		}
	}
}

func TestTrackCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Empty library
	count, err := lib.TrackCount()
	if err != nil {
		t.Fatalf("TrackCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tracks, got %d", count)
	}

	// Add some tracks
	_, err = db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/track1.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/track2.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 2', 2, 1, 2023, 1000, 1000),
			('/music/track3.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 3', 3, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	count, err = lib.TrackCount()
	if err != nil {
		t.Fatalf("TrackCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 tracks, got %d", count)
	}
}

func TestTrackByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a track
	result, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, genre, original_date, release_date, label, added_at, updated_at)
		VALUES ('/music/track1.mp3', 1000, 'Artist', 'Album Artist', 'Album', 'Track Title', 5, 2, 2023, 'Electronic', '2023-05-15', '2023-06-01', 'Record Label', 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	id, _ := result.LastInsertId()

	// Fetch by ID
	track, err := lib.TrackByID(id)
	if err != nil {
		t.Fatalf("TrackByID failed: %v", err)
	}

	// Verify all fields
	if track.ID != id {
		t.Errorf("expected ID %d, got %d", id, track.ID)
	}
	if track.Path != "/music/track1.mp3" {
		t.Errorf("expected path '/music/track1.mp3', got %s", track.Path)
	}
	if track.Artist != "Artist" {
		t.Errorf("expected artist 'Artist', got %s", track.Artist)
	}
	if track.AlbumArtist != "Album Artist" {
		t.Errorf("expected album_artist 'Album Artist', got %s", track.AlbumArtist)
	}
	if track.Album != "Album" {
		t.Errorf("expected album 'Album', got %s", track.Album)
	}
	if track.Title != "Track Title" {
		t.Errorf("expected title 'Track Title', got %s", track.Title)
	}
	if track.TrackNumber != 5 {
		t.Errorf("expected track_number 5, got %d", track.TrackNumber)
	}
	if track.DiscNumber != 2 {
		t.Errorf("expected disc_number 2, got %d", track.DiscNumber)
	}
	if track.Year != 2023 {
		t.Errorf("expected year 2023, got %d", track.Year)
	}
	if track.Genre != "Electronic" {
		t.Errorf("expected genre 'Electronic', got %s", track.Genre)
	}
	if track.OriginalDate != "2023-05-15" {
		t.Errorf("expected original_date '2023-05-15', got %s", track.OriginalDate)
	}
	if track.ReleaseDate != "2023-06-01" {
		t.Errorf("expected release_date '2023-06-01', got %s", track.ReleaseDate)
	}
	if track.Label != "Record Label" {
		t.Errorf("expected label 'Record Label', got %s", track.Label)
	}

	// Non-existent ID should return error
	_, err = lib.TrackByID(99999)
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestTrackByPath(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a track
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES ('/music/specific/path/track.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	// Fetch by path
	track, err := lib.TrackByPath("/music/specific/path/track.mp3")
	if err != nil {
		t.Fatalf("TrackByPath failed: %v", err)
	}
	if track.Title != "Track" {
		t.Errorf("expected title 'Track', got %s", track.Title)
	}

	// Non-existent path should return error
	_, err = lib.TrackByPath("/nonexistent/path.mp3")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestArtistCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Empty library
	count, err := lib.ArtistCount()
	if err != nil {
		t.Fatalf("ArtistCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 artists, got %d", count)
	}

	// Add tracks with 2 distinct album artists
	_, err = db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/track1.mp3', 1000, 'Artist 1', 'Album Artist 1', 'Album 1', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/track2.mp3', 1000, 'Artist 2', 'Album Artist 1', 'Album 1', 'Track 2', 2, 1, 2023, 1000, 1000),
			('/music/track3.mp3', 1000, 'Artist 3', 'Album Artist 2', 'Album 2', 'Track 3', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	count, err = lib.ArtistCount()
	if err != nil {
		t.Fatalf("ArtistCount failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 album artists, got %d", count)
	}
}

func TestAlbumCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Empty library
	count, err := lib.AlbumCount()
	if err != nil {
		t.Fatalf("AlbumCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 albums, got %d", count)
	}

	// Add tracks with 3 distinct albums
	_, err = db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/track1.mp3', 1000, 'Artist', 'Artist', 'Album 1', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/track2.mp3', 1000, 'Artist', 'Artist', 'Album 1', 'Track 2', 2, 1, 2023, 1000, 1000),
			('/music/track3.mp3', 1000, 'Artist', 'Artist', 'Album 2', 'Track 3', 1, 1, 2023, 1000, 1000),
			('/music/track4.mp3', 1000, 'Other Artist', 'Other Artist', 'Album 3', 'Track 4', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	count, err = lib.AlbumCount()
	if err != nil {
		t.Fatalf("AlbumCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 albums, got %d", count)
	}
}

func TestAlbumHasMultipleDiscs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Single disc album
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/single/track1.mp3', 1000, 'Artist', 'Artist', 'Single Disc', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/single/track2.mp3', 1000, 'Artist', 'Artist', 'Single Disc', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	hasMultiple, err := lib.AlbumHasMultipleDiscs("Artist", "Single Disc")
	if err != nil {
		t.Fatalf("AlbumHasMultipleDiscs failed: %v", err)
	}
	if hasMultiple {
		t.Error("expected single disc album to return false")
	}

	// Multi disc album
	_, err = db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/multi/d1t1.mp3', 1000, 'Artist', 'Artist', 'Multi Disc', 'Track D1', 1, 1, 2023, 1000, 1000),
			('/music/multi/d2t1.mp3', 1000, 'Artist', 'Artist', 'Multi Disc', 'Track D2', 1, 2, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	hasMultiple, err = lib.AlbumHasMultipleDiscs("Artist", "Multi Disc")
	if err != nil {
		t.Fatalf("AlbumHasMultipleDiscs failed: %v", err)
	}
	if !hasMultiple {
		t.Error("expected multi disc album to return true")
	}
}

func TestArtistTracks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert tracks across multiple albums
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/2020/track1.mp3', 1000, 'Artist', 'Artist', 'Album 2020', 'Track A', 1, 1, 2020, 1000, 1000),
			('/music/2020/track2.mp3', 1000, 'Artist', 'Artist', 'Album 2020', 'Track B', 2, 1, 2020, 1000, 1000),
			('/music/2018/track1.mp3', 1000, 'Artist', 'Artist', 'Album 2018', 'Track C', 1, 1, 2018, 1000, 1000),
			('/music/noyear/track1.mp3', 1000, 'Artist', 'Artist', 'Album NoYear', 'Track D', 1, 1, 0, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	tracks, err := lib.ArtistTracks("Artist")
	if err != nil {
		t.Fatalf("ArtistTracks failed: %v", err)
	}
	if len(tracks) != 4 {
		t.Errorf("expected 4 tracks, got %d", len(tracks))
	}

	// Should be sorted by year (nulls last), album, disc, track
	// 2018 Album first, then 2020 Album (2 tracks), then NoYear Album
	expectedOrder := []string{"Track C", "Track A", "Track B", "Track D"}
	for i, track := range tracks {
		if track.Title != expectedOrder[i] {
			t.Errorf("track[%d] = %s, expected %s", i, track.Title, expectedOrder[i])
		}
	}
}

func TestDeleteTrack(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a track
	result, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES ('/music/to_delete.mp3', 1000, 'Artist', 'Artist', 'Album', 'To Delete', 1, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	id, _ := result.LastInsertId()

	// Add to FTS for complete test
	track, _ := lib.TrackByID(id)
	_ = lib.AddTrackToFTS(track)

	// Delete the track
	if err := lib.DeleteTrack(id); err != nil {
		t.Fatalf("DeleteTrack failed: %v", err)
	}

	// Verify track is gone
	_, err = lib.TrackByID(id)
	if err == nil {
		t.Error("expected error when fetching deleted track")
	}

	// Verify FTS is also cleaned
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM library_search_fts`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 FTS entries after delete, got %d", count)
	}
}

func TestAllAlbums(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert tracks with various metadata
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, genre, label, original_date, release_date, added_at, updated_at)
		VALUES
			('/music/a1/t1.mp3', 1000, 'A1', 'Artist 1', 'Album A', 'T1', 1, 1, 2020, 'Rock', 'Label X', '2020-01-15', '2020-02-01', 2000, 2000),
			('/music/a1/t2.mp3', 1000, 'A1', 'Artist 1', 'Album A', 'T2', 2, 1, 2020, 'Rock', 'Label X', '2020-01-15', '2020-02-01', 1000, 1000),
			('/music/a2/t1.mp3', 1000, 'A2', 'Artist 2', 'Album B', 'T1', 1, 1, 2019, 'Jazz', 'Label Y', '2019-06-10', '', 3000, 3000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	albums, err := lib.AllAlbums()
	if err != nil {
		t.Fatalf("AllAlbums failed: %v", err)
	}

	if len(albums) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(albums))
	}

	// Albums should be sorted by original_date DESC, release_date DESC, added_at DESC
	// Album A (2020-01-15) should come before Album B (2019-06-10)
	if albums[0].Album != "Album A" {
		t.Errorf("expected first album to be 'Album A', got %s", albums[0].Album)
	}
	if albums[1].Album != "Album B" {
		t.Errorf("expected second album to be 'Album B', got %s", albums[1].Album)
	}

	// Verify aggregated data
	if albums[0].TrackCount != 2 {
		t.Errorf("expected Album A to have 2 tracks, got %d", albums[0].TrackCount)
	}
	if albums[0].Genre != "Rock" {
		t.Errorf("expected Album A genre 'Rock', got %s", albums[0].Genre)
	}
	if albums[0].Label != "Label X" {
		t.Errorf("expected Album A label 'Label X', got %s", albums[0].Label)
	}
}

func TestAlbumTrackIDs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert tracks for an album
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/album/t3.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 3', 3, 1, 2023, 1000, 1000),
			('/music/album/t1.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 1', 1, 1, 2023, 1000, 1000),
			('/music/album/t2.mp3', 1000, 'Artist', 'Artist', 'Album', 'Track 2', 2, 1, 2023, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	ids, err := lib.AlbumTrackIDs("Artist", "Album")
	if err != nil {
		t.Fatalf("AlbumTrackIDs failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("expected 3 track IDs, got %d", len(ids))
	}

	// Verify they're in order by fetching tracks
	for i, id := range ids {
		track, err := lib.TrackByID(id)
		if err != nil {
			t.Fatalf("failed to fetch track %d: %v", id, err)
		}
		expectedNum := i + 1
		if track.TrackNumber != expectedNum {
			t.Errorf("track[%d] has track_number %d, expected %d", i, track.TrackNumber, expectedNum)
		}
	}
}

func TestCollectTrackIDs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert test data
	_, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, added_at, updated_at)
		VALUES
			('/music/a1/album1/t1.mp3', 1000, 'Artist', 'Artist 1', 'Album 1', 'Track 1', 1, 1, 2020, 1000, 1000),
			('/music/a1/album1/t2.mp3', 1000, 'Artist', 'Artist 1', 'Album 1', 'Track 2', 2, 1, 2020, 1000, 1000),
			('/music/a1/album2/t1.mp3', 1000, 'Artist', 'Artist 1', 'Album 2', 'Track 1', 1, 1, 2021, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert tracks: %v", err)
	}

	// Test root node - should return nil
	rootNode := Node{level: LevelRoot}
	ids, err := lib.CollectTrackIDs(rootNode)
	if err != nil {
		t.Fatalf("CollectTrackIDs(root) failed: %v", err)
	}
	if ids != nil {
		t.Errorf("expected nil for root node, got %v", ids)
	}

	// Test artist node - should return all artist's tracks
	artistNode := Node{level: LevelArtist, artist: "Artist 1"}
	ids, err = lib.CollectTrackIDs(artistNode)
	if err != nil {
		t.Fatalf("CollectTrackIDs(artist) failed: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 tracks for artist, got %d", len(ids))
	}

	// Test album node - should return album's tracks
	albumNode := Node{level: LevelAlbum, artist: "Artist 1", album: "Album 1"}
	ids, err = lib.CollectTrackIDs(albumNode)
	if err != nil {
		t.Fatalf("CollectTrackIDs(album) failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 tracks for album, got %d", len(ids))
	}

	// Test track node - should return just that track
	track, _ := lib.TrackByPath("/music/a1/album1/t1.mp3")
	trackNode := Node{level: LevelTrack, track: track}
	ids, err = lib.CollectTrackIDs(trackNode)
	if err != nil {
		t.Fatalf("CollectTrackIDs(track) failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != track.ID {
		t.Errorf("expected [%d] for track, got %v", track.ID, ids)
	}

	// Test track node with nil track
	nilTrackNode := Node{level: LevelTrack, track: nil}
	ids, err = lib.CollectTrackIDs(nilTrackNode)
	if err != nil {
		t.Fatalf("CollectTrackIDs(nil track) failed: %v", err)
	}
	if ids != nil {
		t.Errorf("expected nil for nil track, got %v", ids)
	}
}

func TestTrackByID_NullableFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// Insert a track with NULL values for optional fields
	//nolint:dupword // SQL NULL values
	result, err := db.Exec(`
		INSERT INTO library_tracks (path, mtime, artist, album_artist, album, title, track_number, disc_number, year, genre, original_date, release_date, label, added_at, updated_at)
		VALUES ('/music/minimal.mp3', 1000, 'Artist', 'Artist', 'Album', 'Title', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1000, 1000)
	`)
	if err != nil {
		t.Fatalf("failed to insert track: %v", err)
	}

	id, _ := result.LastInsertId()

	track, err := lib.TrackByID(id)
	if err != nil {
		t.Fatalf("TrackByID failed: %v", err)
	}

	// Verify NULL fields are handled correctly
	if track.TrackNumber != 0 {
		t.Errorf("expected track_number 0, got %d", track.TrackNumber)
	}
	if track.DiscNumber != 0 {
		t.Errorf("expected disc_number 0, got %d", track.DiscNumber)
	}
	if track.Year != 0 {
		t.Errorf("expected year 0, got %d", track.Year)
	}
	if track.Genre != "" {
		t.Errorf("expected empty genre, got %s", track.Genre)
	}
	if track.OriginalDate != "" {
		t.Errorf("expected empty original_date, got %s", track.OriginalDate)
	}
	if track.ReleaseDate != "" {
		t.Errorf("expected empty release_date, got %s", track.ReleaseDate)
	}
	if track.Label != "" {
		t.Errorf("expected empty label, got %s", track.Label)
	}
}

// Helper to check for SQL syntax errors in all query methods
func TestQueryMethods_NoSQLErrors(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	lib := New(db)

	// These should all execute without SQL syntax errors on empty DB
	tests := []struct {
		name string
		fn   func() error
	}{
		{"Artists", func() error { _, err := lib.Artists(); return err }},
		{"Albums", func() error { _, err := lib.Albums("test"); return err }},
		{"Tracks", func() error { _, err := lib.Tracks("test", "test"); return err }},
		{"TrackCount", func() error { _, err := lib.TrackCount(); return err }},
		{"ArtistCount", func() error { _, err := lib.ArtistCount(); return err }},
		{"AlbumCount", func() error { _, err := lib.AlbumCount(); return err }},
		{"AlbumHasMultipleDiscs", func() error { _, err := lib.AlbumHasMultipleDiscs("test", "test"); return err }},
		{"ArtistTracks", func() error { _, err := lib.ArtistTracks("test"); return err }},
		{"AllAlbums", func() error { _, err := lib.AllAlbums(); return err }},
		{"AlbumTrackIDs", func() error { _, err := lib.AlbumTrackIDs("test", "test"); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Errorf("%s failed: %v", tc.name, err)
				}
			}
		})
	}
}
