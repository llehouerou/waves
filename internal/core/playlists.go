package core

import (
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlists"
)

// The playlist data a client handles, so that it imports the core only.
type (
	Playlist               = playlists.Playlist
	PlaylistFolder         = playlists.Folder
	PlaylistSearchItem     = playlists.SearchItem
	PlaylistDeepSearchItem = playlists.DeepSearchItem
)

// IsFavoritesPlaylist reports whether id is the Favorites playlist, which can
// be neither renamed nor deleted.
func IsFavoritesPlaylist(id int64) bool {
	return playlists.IsFavorites(id)
}

// playlistsAPI is the part of playlists.Playlists a client calls as it is.
type playlistsAPI interface {
	Folders(parentID *int64) ([]PlaylistFolder, error)
	FolderByID(id int64) (*PlaylistFolder, error)
	List(folderID *int64) ([]Playlist, error)
	Get(id int64) (*Playlist, error)
	IsPlaylistEmpty(playlistID int64) (bool, error)
	IsFolderEmpty(folderID int64) (bool, error)
	AllForAddToPlaylist() ([]PlaylistSearchItem, error)
	AllDeepSearchItems() ([]PlaylistDeepSearchItem, error)

	Create(folderID *int64, name string) (int64, error)
	CreateFolder(parentID *int64, name string) (int64, error)
	Rename(id int64, name string) error
	RenameFolder(id int64, name string) error
	Delete(id int64) error
	DeleteFolder(id int64) error
	AddTracks(playlistID int64, trackIDs []int64) error
	RemoveTrack(playlistID int64, position int) error
	MoveIndices(playlistID int64, positions []int, delta int) ([]int, error)
	UpdateLastUsed(id int64) error

	ToggleFavorites(trackIDs []int64) (map[int64]bool, error)
	FavoriteTrackIDs() (map[int64]bool, error)
}

// Playlists is the playlists domain of the core: playlists, their folders
// and their tracks. Favorites is one of them.
type Playlists struct {
	playlistsAPI
	pls *playlists.Playlists
}

// Tracks returns the tracks of a playlist, ready for the queue.
func (p *Playlists) Tracks(id int64) ([]Track, error) {
	tracks, err := p.pls.Tracks(id)
	if err != nil {
		return nil, err
	}
	return playback.TracksFromPlaylist(tracks), nil
}
