package playlist

import "time"

// Track represents a single track in a playlist.
type Track struct {
	ID          int64  // library track ID (0 if from filesystem)
	Path        string // file path for playback
	Title       string
	Artist      string
	Album       string
	TrackNumber int
	Duration    time.Duration
}

// Playlist holds an ordered collection of tracks.
type Playlist struct {
	tracks []Track
}

// NewPlaylist creates a new empty playlist.
func NewPlaylist() *Playlist {
	return &Playlist{
		tracks: make([]Track, 0),
	}
}

// Add appends tracks to the playlist.
func (p *Playlist) Add(tracks ...Track) {
	p.tracks = append(p.tracks, tracks...)
}

// Remove removes the track at the given index.
// Returns false if index is out of bounds.
func (p *Playlist) Remove(index int) bool {
	if index < 0 || index >= len(p.tracks) {
		return false
	}
	p.tracks = append(p.tracks[:index], p.tracks[index+1:]...)
	return true
}

// Clear removes all tracks from the playlist.
func (p *Playlist) Clear() {
	p.tracks = p.tracks[:0]
}

// Tracks returns a copy of all tracks.
func (p *Playlist) Tracks() []Track {
	result := make([]Track, len(p.tracks))
	copy(result, p.tracks)
	return result
}

// Track returns the track at the given index, or nil if out of bounds.
func (p *Playlist) Track(index int) *Track {
	if index < 0 || index >= len(p.tracks) {
		return nil
	}
	return &p.tracks[index]
}

// Len returns the number of tracks.
func (p *Playlist) Len() int {
	return len(p.tracks)
}

// Move moves the track at fromIndex to toIndex.
// Returns false if either index is out of bounds.
func (p *Playlist) Move(fromIndex, toIndex int) bool {
	if fromIndex < 0 || fromIndex >= len(p.tracks) {
		return false
	}
	if toIndex < 0 || toIndex >= len(p.tracks) {
		return false
	}
	if fromIndex == toIndex {
		return true
	}

	track := p.tracks[fromIndex]
	// Remove from old position
	p.tracks = append(p.tracks[:fromIndex], p.tracks[fromIndex+1:]...)
	// Insert at new position
	p.tracks = append(p.tracks[:toIndex], append([]Track{track}, p.tracks[toIndex:]...)...)
	return true
}
