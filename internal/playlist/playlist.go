package playlist

import (
	"slices"
	"time"
)

// Track represents a single track in a playlist.
type Track struct {
	ID          int64  // library track ID (0 if from filesystem)
	Path        string // file path for playback
	Title       string
	Artist      string
	Album       string
	TrackNumber int
	DiscNumber  int
	Genre       string
	Year        int
	Duration    time.Duration
}

// EntryID identifies one entry of a playlist: a track added twice is two
// entries, each with its own ID. A playlist never gives the same ID twice, so
// an entry keeps its ID in every track list the queue history recorded it in.
// The zero EntryID is no entry.
type EntryID uint64

// entry is one place of a playlist: a track and the ID of that place.
type entry struct {
	id    EntryID
	track Track
}

// Playlist holds an ordered collection of tracks, each one an entry.
type Playlist struct {
	entries []entry
	lastID  EntryID
}

// NewPlaylist creates a new empty playlist.
func NewPlaylist() *Playlist {
	return &Playlist{
		entries: make([]entry, 0),
	}
}

// Add appends tracks to the playlist, each as a new entry.
func (p *Playlist) Add(tracks ...Track) {
	for _, t := range tracks {
		p.lastID++
		p.entries = append(p.entries, entry{id: p.lastID, track: t})
	}
}

// Remove removes the track at the given index.
// Returns false if index is out of bounds.
func (p *Playlist) Remove(index int) bool {
	if index < 0 || index >= len(p.entries) {
		return false
	}
	p.entries = slices.Delete(p.entries, index, index+1)
	return true
}

// Clear removes all tracks from the playlist.
func (p *Playlist) Clear() {
	p.entries = p.entries[:0]
}

// Tracks returns a copy of all tracks.
func (p *Playlist) Tracks() []Track {
	result := make([]Track, len(p.entries))
	for i := range p.entries {
		result[i] = p.entries[i].track
	}
	return result
}

// Track returns the track at the given index, or nil if out of bounds.
func (p *Playlist) Track(index int) *Track {
	if index < 0 || index >= len(p.entries) {
		return nil
	}
	return &p.entries[index].track
}

// EntryAt returns the ID of the entry at the given index, or the zero EntryID
// if out of bounds.
func (p *Playlist) EntryAt(index int) EntryID {
	if index < 0 || index >= len(p.entries) {
		return 0
	}
	return p.entries[index].id
}

// IndexOf returns the index of the entry id, or -1 if the playlist does not
// hold it.
func (p *Playlist) IndexOf(id EntryID) int {
	if id == 0 {
		return -1
	}
	for i := range p.entries {
		if p.entries[i].id == id {
			return i
		}
	}
	return -1
}

// Len returns the number of tracks.
func (p *Playlist) Len() int {
	return len(p.entries)
}

// Move moves the track at fromIndex to toIndex.
// Returns false if either index is out of bounds.
func (p *Playlist) Move(fromIndex, toIndex int) bool {
	if fromIndex < 0 || fromIndex >= len(p.entries) {
		return false
	}
	if toIndex < 0 || toIndex >= len(p.entries) {
		return false
	}
	if fromIndex == toIndex {
		return true
	}

	e := p.entries[fromIndex]
	p.entries = slices.Delete(p.entries, fromIndex, fromIndex+1)
	p.entries = slices.Insert(p.entries, toIndex, e)
	return true
}

// rewrite passes every entry through rewrite, in place, and reports whether
// one of them changed.
func (p *Playlist) rewrite(rewrite func(entry) entry) bool {
	changed := false
	for i := range p.entries {
		e := rewrite(p.entries[i])
		if e != p.entries[i] {
			p.entries[i] = e
			changed = true
		}
	}
	return changed
}

// snapshot returns a copy of the entries, for the queue history.
func (p *Playlist) snapshot() []entry {
	return slices.Clone(p.entries)
}

// restore puts back entries a snapshot took, with their IDs.
func (p *Playlist) restore(entries []entry) {
	p.entries = slices.Clone(entries)
}
