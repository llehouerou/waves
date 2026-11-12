package core

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// musicFolder creates a folder holding a.mp3, b.flac, notes.txt and sub/c.mp3.
// The files are empty: a track without readable tags is titled by its name.
func musicFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"b.flac", "a.mp3", "notes.txt", "sub/c.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func titles(tracks []Track) []string {
	out := make([]string, len(tracks))
	for i, t := range tracks {
		out[i] = t.Title
	}
	return out
}

func TestTracksUnder(t *testing.T) {
	dir := musicFolder(t)

	tests := []struct {
		name string
		path string
		want []string
	}{
		{"a directory gives every music file under it, by path", dir, []string{"a.mp3", "b.flac", "c.mp3"}},
		{"a music file gives itself", filepath.Join(dir, "b.flac"), []string{"b.flac"}},
		{"another file gives nothing", filepath.Join(dir, "notes.txt"), []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracks, err := TracksUnder(tt.path)
			if err != nil {
				t.Fatalf("TracksUnder: %v", err)
			}
			if got := titles(tracks); !slices.Equal(got, tt.want) {
				t.Errorf("titles = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFolderTracks(t *testing.T) {
	dir := musicFolder(t)

	tracks, index, err := FolderTracks(filepath.Join(dir, "b.flac"))
	if err != nil {
		t.Fatalf("FolderTracks: %v", err)
	}
	// The file's own folder only, and where the file is in it.
	if got := titles(tracks); !slices.Equal(got, []string{"a.mp3", "b.flac"}) || index != 1 {
		t.Errorf("FolderTracks(file) = %v at %d, want [a.mp3 b.flac] at 1", got, index)
	}

	tracks, index, err = FolderTracks(dir)
	if err != nil {
		t.Fatalf("FolderTracks: %v", err)
	}
	if got := titles(tracks); !slices.Equal(got, []string{"a.mp3", "b.flac", "c.mp3"}) || index != 0 {
		t.Errorf("FolderTracks(directory) = %v at %d, want everything under it at 0", got, index)
	}
}

func TestTracksFromLibrary(t *testing.T) {
	tracks := TracksFromLibrary([]LibraryTrack{{
		ID: 7, Path: "/lib/low/canada.flac", Title: "Canada", Artist: "Low", AlbumArtist: "Low",
		Album: "Trust", TrackNumber: 2, DiscNumber: 1, Year: 2002, Genre: "Slowcore",
	}})

	want := Track{
		ID: 7, Path: "/lib/low/canada.flac", Title: "Canada", Artist: "Low",
		Album: "Trust", TrackNumber: 2, DiscNumber: 1, Year: 2002, Genre: "Slowcore",
	}
	if len(tracks) != 1 || tracks[0] != want {
		t.Errorf("TracksFromLibrary = %+v, want %+v", tracks, want)
	}
}
