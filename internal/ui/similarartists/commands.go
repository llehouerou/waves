package similarartists

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/request"
)

// Lastfm is what the popup asks of Last.fm: the artists similar to one.
type Lastfm interface {
	GetSimilarArtists(artist string, limit int) ([]core.SimilarArtist, error)
}

// Library is what the popup reads of the library.
type Library interface {
	Artists() ([]string, error)
}

// FetchResultMsg contains the result of fetching similar artists.
type FetchResultMsg struct {
	Request      request.ID
	InLibrary    []SimilarArtistItem
	NotInLibrary []SimilarArtistItem
	Err          error
}

// fetchParams contains parameters for the fetch command.
type fetchParams struct {
	client     Lastfm
	library    Library
	artistName string
}

// fetch fetches similar artists and partitions them by library presence:
// its answer carries the ID of the request.
func fetch(params fetchParams) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			msg := fetchSimilar(params)
			msg.Request = id
			return msg
		}
	}
}

// fetchSimilar fetches the artists similar to the one of params.
func fetchSimilar(params fetchParams) FetchResultMsg {
	// Fetch similar artists from Last.fm
	similar, err := params.client.GetSimilarArtists(params.artistName, 30)
	if err != nil {
		return FetchResultMsg{Err: err}
	}

	// Get library artists for lookup
	libraryArtists, err := params.library.Artists()
	if err != nil {
		return FetchResultMsg{Err: err}
	}

	// Build case-insensitive lookup set
	librarySet := make(map[string]string) // lowercase -> original case
	for _, a := range libraryArtists {
		librarySet[strings.ToLower(a)] = a
	}

	// Partition results
	var inLib, notInLib []SimilarArtistItem
	for _, s := range similar {
		item := SimilarArtistItem{
			Name:       s.Name,
			MatchScore: s.MatchScore,
		}
		if _, exists := librarySet[strings.ToLower(s.Name)]; exists {
			item.InLibrary = true
			inLib = append(inLib, item)
		} else {
			notInLib = append(notInLib, item)
		}
	}

	// Sort each list by score descending
	sort.Slice(inLib, func(i, j int) bool {
		return inLib[i].MatchScore > inLib[j].MatchScore
	})
	sort.Slice(notInLib, func(i, j int) bool {
		return notInLib[i].MatchScore > notInLib[j].MatchScore
	})

	return FetchResultMsg{
		InLibrary:    inLib,
		NotInLibrary: notInLib,
	}
}
