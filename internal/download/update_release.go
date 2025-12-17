package download

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// handleReleasePhaseKey handles keyboard input in the release phase.
func (m *Model) handleReleasePhaseKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		cmd := m.handleReleaseEnter()
		return m, cmd
	case keyUp, "k":
		if m.state == StateReleaseResults {
			m.releaseCursor.Move(-1, len(m.releases), m.height-12)
		}
	case keyDown, "j":
		if m.state == StateReleaseResults {
			m.releaseCursor.Move(1, len(m.releases), m.height-12)
		}
	case keyHome, "g":
		if m.state == StateReleaseResults {
			m.releaseCursor.JumpStart()
		}
	case keyEnd, "G":
		if m.state == StateReleaseResults {
			m.releaseCursor.JumpEnd(len(m.releases), m.height-12)
		}
	case keyBackspace:
		m.handleReleaseBack()
	case "d":
		// Toggle deduplicate filter
		if m.state == StateReleaseResults {
			m.deduplicateRelease = !m.deduplicateRelease
			m.reapplyReleaseFilters()
		}
	}
	return m, nil
}

// handleReleaseEnter processes Enter key in the release phase.
func (m *Model) handleReleaseEnter() tea.Cmd {
	//nolint:exhaustive // Only handling release phase states
	switch m.state {
	case StateReleaseResults:
		pos := m.releaseCursor.Pos()
		if len(m.releases) == 0 || pos >= len(m.releases) {
			return nil
		}
		// User selected a release - fetch full details with tracks
		selected := m.releases[pos]
		m.selectedRelease = &selected
		m.expectedTracks = selected.TrackCount
		m.state = StateReleaseDetailsLoading
		m.statusMsg = "Loading release details..."
		return fetchReleaseDetailsCmd(m.mbClient, selected.ID)

	case StateReleaseLoading, StateReleaseDetailsLoading:
		// No action during loading
		return nil

	default:
		// Other states handled by different phase handlers
		return nil
	}
}

// handleReleaseBack processes Backspace key in the release phase.
func (m *Model) handleReleaseBack() {
	//nolint:exhaustive // Only handling release phase states
	switch m.state {
	case StateReleaseResults:
		// Go back to release groups
		m.state = StateReleaseGroupResults
		m.selectedReleaseGroup = nil
		m.releases = nil
		m.releaseCursor.Reset()
		m.expectedTracks = 0
		m.statusMsg = ""
	case StateReleaseLoading, StateReleaseDetailsLoading:
		// No escape action in loading states
	default:
		// Other states handled by different phase handlers
	}
}

// handleReleaseResult processes release results for track count determination.
func (m *Model) handleReleaseResult(msg ReleaseResultMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateReleaseGroupResults
		m.errorMsg = fmt.Sprintf("Error loading releases: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	// Handle case where no releases found
	if len(msg.Releases) == 0 {
		m.state = StateReleaseGroupResults
		m.errorMsg = "No releases found for this release group"
		m.statusMsg = ""
		return m, nil
	}

	// Sort releases by date ascending (oldest first)
	releases := msg.Releases
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Date < releases[j].Date
	})

	// Store raw releases for re-filtering
	m.releasesRaw = releases
	m.reapplyReleaseFilters()

	// Always show release selection - user must choose for import process
	m.releaseCursor.Reset()
	m.state = StateReleaseResults
	m.statusMsg = "Select a release"
	return m, nil
}

// handleReleaseDetailsResult processes release details and starts slskd search.
func (m *Model) handleReleaseDetailsResult(msg ReleaseDetailsResultMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateReleaseResults
		m.errorMsg = fmt.Sprintf("Error loading release details: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	// Store the full release details for later use (importing)
	m.selectedReleaseDetails = msg.Details

	// Now start slskd search
	cmd := m.startSlskdSearchWithTrackCount()
	return m, cmd
}

// reapplyReleaseFilters filters releases based on current deduplicateRelease setting.
func (m *Model) reapplyReleaseFilters() {
	if m.deduplicateRelease {
		m.releases = deduplicateReleases(m.releasesRaw)
	} else {
		m.releases = m.releasesRaw
	}
	// Clamp cursor if out of bounds
	m.releaseCursor.ClampToBounds(len(m.releases))
}

// releaseKey is used for deduplicating releases.
type releaseKey struct {
	trackCount  int
	year        string
	formats     string
	releaseType string
}

// deduplicateReleases removes duplicate releases based on (track count, year, formats, release type).
// For duplicates, keeps the one with preferred country: XW > US > others.
// Preserves the original order (assumes already sorted by date).
func deduplicateReleases(releases []musicbrainz.Release) []musicbrainz.Release {
	seen := make(map[releaseKey]int) // key -> index in result
	result := make([]musicbrainz.Release, 0, len(releases))

	for i := range releases {
		r := &releases[i]
		key := releaseKey{
			trackCount:  r.TrackCount,
			year:        extractYear(r.Date),
			formats:     r.Formats,
			releaseType: r.ReleaseType,
		}

		if existingIdx, exists := seen[key]; exists {
			// Compare countries - replace if new one is better
			existing := &result[existingIdx]
			if countryPriority(r.Country) < countryPriority(existing.Country) {
				result[existingIdx] = *r
			}
		} else {
			// New key - add to result
			seen[key] = len(result)
			result = append(result, *r)
		}
	}

	return result
}

// countryPriority returns priority for country (lower is better).
// XW (worldwide) > US > others.
func countryPriority(country string) int {
	switch country {
	case "XW":
		return 0
	case "US":
		return 1
	default:
		return 2
	}
}

// extractYear extracts the year (first 4 chars) from a date string.
func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
}
