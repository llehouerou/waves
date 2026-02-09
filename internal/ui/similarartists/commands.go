package similarartists

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
)

// FetchResultMsg contains the result of fetching similar artists.
type FetchResultMsg struct {
	InLibrary    []SimilarArtistItem
	NotInLibrary []SimilarArtistItem
	Err          error
}

// FetchParams contains parameters for the fetch command.
type FetchParams struct {
	Client     *lastfm.Client
	Library    *library.Library
	ArtistName string
}

// FetchCmd fetches similar artists and partitions them by library presence.
func FetchCmd(params FetchParams) tea.Cmd {
	return func() tea.Msg {
		// Fetch similar artists from Last.fm
		similar, err := params.Client.GetSimilarArtists(params.ArtistName, 30)
		if err != nil {
			return FetchResultMsg{Err: err}
		}

		// Get library artists for lookup
		libraryArtists, err := params.Library.Artists()
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
}
