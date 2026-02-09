package similarartists

import (
	tea "github.com/charmbracelet/bubbletea"

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
	inLibrary    []SimilarArtistItem // Artists in library
	notInLibrary []SimilarArtistItem // Artists not in library

	cursor   int // Current selection index
	loading  bool
	errorMsg string // Error message if fetch failed

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

// Init starts the initial fetch command.
func (m *Model) Init() tea.Cmd {
	return FetchCmd(FetchParams{
		Client:     m.client,
		Library:    m.library,
		ArtistName: m.artistName,
	})
}

// totalItems returns the total number of items across both sections.
func (m *Model) totalItems() int {
	return len(m.inLibrary) + len(m.notInLibrary)
}

// selectedItem returns the currently selected item, or nil if none.
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
