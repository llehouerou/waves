package download

// State represents the current state of the download view.
type State int

const (
	// Phase 1: Artist Search
	StateSearch          State = iota // Waiting for search input
	StateArtistSearching              // Searching artists
	StateArtistResults                // Showing artist results

	// Phase 2: MusicBrainz Release Selection
	StateReleaseGroupLoading   // Loading release groups
	StateReleaseGroupResults   // Showing release groups
	StateReleaseLoading        // Loading releases for track count
	StateReleaseResults        // Showing releases to select track count
	StateReleaseDetailsLoading // Loading release details (with tracks)

	// Phase 3: Slskd Source Selection
	StateSlskdSearching // Searching slskd
	StateSlskdResults   // Showing slskd results
	StateDownloading    // Download queued
)

// IsSearchPhase returns true if in the artist search phase.
func (s State) IsSearchPhase() bool {
	return s >= StateSearch && s <= StateArtistResults
}

// IsReleaseGroupPhase returns true if in the release group selection phase.
func (s State) IsReleaseGroupPhase() bool {
	return s >= StateReleaseGroupLoading && s <= StateReleaseGroupResults
}

// IsReleasePhase returns true if in the release selection phase.
func (s State) IsReleasePhase() bool {
	return s >= StateReleaseLoading && s <= StateReleaseDetailsLoading
}

// IsSlskdPhase returns true if in the slskd source selection phase.
func (s State) IsSlskdPhase() bool {
	return s >= StateSlskdSearching && s <= StateDownloading
}

// IsLoading returns true if this is a loading/async state.
func (s State) IsLoading() bool {
	switch s {
	case StateArtistSearching, StateReleaseGroupLoading,
		StateReleaseLoading, StateReleaseDetailsLoading,
		StateSlskdSearching, StateDownloading:
		return true
	case StateSearch, StateArtistResults, StateReleaseGroupResults,
		StateReleaseResults, StateSlskdResults:
		return false
	}
	return false
}

// CanNavigate returns true if this state allows cursor navigation.
func (s State) CanNavigate() bool {
	switch s {
	case StateArtistResults, StateReleaseGroupResults,
		StateReleaseResults, StateSlskdResults:
		return true
	case StateSearch, StateArtistSearching, StateReleaseGroupLoading,
		StateReleaseLoading, StateReleaseDetailsLoading,
		StateSlskdSearching, StateDownloading:
		return false
	}
	return false
}
