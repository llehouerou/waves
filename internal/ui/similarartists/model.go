package similarartists

import (
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
)

// SimilarArtistItem represents a similar artist with library status.
type SimilarArtistItem struct {
	Name       string
	MatchScore float64 // 0.0-1.0 from Last.fm
	InLibrary  bool
}

// Model is the similar artists popup state.
type Model struct {
	client  *lastfm.Client
	library *library.Library

	artistName   string              // Source artist name
	inLibrary    []SimilarArtistItem //nolint:unused // used by Update/View in subsequent tasks
	notInLibrary []SimilarArtistItem //nolint:unused // used by Update/View in subsequent tasks

	cursor   int //nolint:unused // used by Update/View in subsequent tasks
	loading  bool
	errorMsg string //nolint:unused // used by View in subsequent tasks

	width, height int
}

// New creates a new similar artists popup.
func New(client *lastfm.Client, lib *library.Library, artistName string) *Model {
	return &Model{
		client:     client,
		library:    lib,
		artistName: artistName,
		loading:    true,
	}
}

// SetSize sets the available dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// totalItems returns the total number of items across both sections.
//
//nolint:unused // used by Update/View in subsequent tasks
func (m *Model) totalItems() int {
	return len(m.inLibrary) + len(m.notInLibrary)
}

// selectedItem returns the currently selected item, or nil if none.
//
//nolint:unused // used by Update/View in subsequent tasks
func (m *Model) selectedItem() *SimilarArtistItem {
	if m.totalItems() == 0 {
		return nil
	}
	if m.cursor < len(m.inLibrary) {
		return &m.inLibrary[m.cursor]
	}
	idx := m.cursor - len(m.inLibrary)
	if idx < len(m.notInLibrary) {
		return &m.notInLibrary[idx]
	}
	return nil
}
