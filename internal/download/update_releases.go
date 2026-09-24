package download

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/musicbrainz/workflow"
	"github.com/llehouerou/waves/internal/releases"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// relTab is a tab of the releases list; it indexes relCursors.
type relTab int

const (
	relTabRecent relTab = iota
	relTabUpcoming
)

func (t relTab) String() string { return [...]string{"Recent", "Upcoming"}[t] }

// other returns the tab that is not this one.
func (t relTab) other() relTab { return 1 - t }

// relFilter narrows the releases list to library artists or discoveries.
type relFilter int

const (
	relFilterAll relFilter = iota
	relFilterLibrary
	relFilterDiscoveries
	relFilterCount
)

func (f relFilter) String() string { return [...]string{"all", "library", "discoveries"}[f] }

// SlskdMissingMsg is shown when a download is asked for without slskd configured.
const SlskdMissingMsg = "slskd not configured — see config.toml [slskd] section"

// ReleaseRow is a cached release plus the library facts the list renders.
type ReleaseRow struct {
	releases.Release
	Owned       bool // the album is already in the library
	Downloading bool // a download of it is pending or in flight
}

// ReleasesLoadedMsg carries the matched releases read from the cache, or the
// error of a background refresh (rows stay untouched then). Refreshed marks the
// messages that end a ListenBrainz refresh, not the ones a warm-up triggers.
type ReleasesLoadedMsg struct {
	Rows      []ReleaseRow
	Refreshed bool
	Err       error
}

// RefreshReleases asks the app for a forced ListenBrainz refresh.
type RefreshReleases struct{}

// ActionType implements action.Action.
func (RefreshReleases) ActionType() string { return "download.refresh_releases" }

// LoadReleasesParams are the sources the list is built from.
type LoadReleasesParams struct {
	Cache     *releases.Cache
	Library   *library.Library
	Downloads *downloads.Manager
	Refreshed bool // the message ends a ListenBrainz refresh
}

// LoadReleasesCmd reads the cached releases and marks the ones already owned or
// already downloading.
func LoadReleasesCmd(p LoadReleasesParams) tea.Cmd {
	return func() tea.Msg {
		matched, err := p.Cache.Matched()
		if err != nil {
			return ReleasesLoadedMsg{Refreshed: p.Refreshed, Err: err}
		}
		var active map[string]bool
		if p.Downloads != nil {
			active, _ = p.Downloads.ActiveReleaseGroups() //nolint:errcheck // a missing mark is not worth an error line
		}
		return ReleasesLoadedMsg{Rows: markRows(matched, p.Library, active), Refreshed: p.Refreshed}
	}
}

// markRows flags releases already in the library and those being downloaded.
//
// ponytail: one query per distinct library artist (~150 on a real library), off
// the UI thread. A single all-albums query if it ever shows.
func markRows(matched []releases.Release, lib *library.Library, active map[string]bool) []ReleaseRow {
	rows := make([]ReleaseRow, len(matched))
	albums := make(map[string]map[string]struct{})
	for i := range matched {
		r := &matched[i]
		rows[i] = ReleaseRow{Release: *r, Downloading: active[r.ReleaseGroupMBID]}
		if lib == nil || !r.InLibrary {
			continue
		}
		set, ok := albums[r.ArtistCreditName]
		if !ok {
			set = lib.AlbumsForArtistNormalized(r.ArtistCreditName)
			albums[r.ArtistCreditName] = set
		}
		_, rows[i].Owned = set[library.NormalizeTitle(r.ReleaseName)]
	}
	return rows
}

// ReleasesParams configures phase 0 when it opens.
type ReleasesParams struct {
	Cache       *releases.Cache
	Discoveries bool  // a Last.fm API key is configured
	Refreshing  bool  // a ListenBrainz refresh is in flight
	RefreshErr  error // last background refresh failure, nil when none
}

// StartReleases opens phase 0 on the cached list.
func (m *Model) StartReleases(p ReleasesParams) tea.Cmd {
	m.relDiscoveries = p.Discoveries
	m.relRefreshing = p.Refreshing
	m.relErr = errmsg.Format(errmsg.OpReleasesRefresh, p.RefreshErr)
	m.state = StateReleasesLoading
	m.searchInput.Blur()
	return LoadReleasesCmd(LoadReleasesParams{Cache: p.Cache, Library: m.lib, Downloads: m.downloads})
}

// FromReleases reports whether the current flow started from the releases list.
func (m *Model) FromReleases() bool {
	return m.fromReleases
}

// handleReleasesPhaseKey handles keyboard input in the releases list.
func (m *Model) handleReleasesPhaseKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	key := msg.String()
	if m.relSearching {
		m.editReleasesQuery(msg)
		return m, nil
	}
	switch key {
	case "/":
		m.relSearching = true
	case keyEnter:
		cmd := m.handleReleasesEnter()
		return m, cmd
	case keyBackspace:
		// The two entry points are independent: no fallback to artist search.
		return m, func() tea.Msg { return ActionMsg(Close{}) }
	case "tab", "h", "l", "left", "right":
		m.relTab = m.relTab.other()
		m.relCursors[m.relTab].ClampToBounds(len(m.visibleReleases()))
	case "f":
		m.relFilter = (m.relFilter + 1) % m.filterCount()
		m.resetReleaseCursors()
	case "r":
		m.relRefreshing = true
		m.relErr = ""
		return m, func() tea.Msg { return ActionMsg(RefreshReleases{}) }
	default:
		rows := m.visibleReleases()
		m.relCursors[m.relTab].HandleKey(key, len(rows), m.releasesBodyHeight())
	}
	return m, nil
}

// editReleasesQuery types into the incremental search. Enter keeps the query
// and leaves typing mode, esc drops it (esc is intercepted upstream).
func (m *Model) editReleasesQuery(msg tea.KeyMsg) {
	switch msg.String() {
	case keyEnter:
		m.relSearching = false
		return
	case keyBackspace:
		if q := []rune(m.relQuery); len(q) > 0 {
			m.relQuery = string(q[:len(q)-1])
		}
	case " ":
		// Bubble Tea reports space as KeySpace, not as a rune.
		m.relQuery += " "
	default:
		if msg.Type != tea.KeyRunes {
			return
		}
		m.relQuery += string(msg.Runes)
	}
	m.resetReleaseCursors()
}

// clearReleasesSearch drops the query and leaves typing mode.
func (m *Model) clearReleasesSearch() {
	m.relSearching = false
	m.relQuery = ""
	m.resetReleaseCursors()
}

// resetReleaseCursors puts both tabs back on their first row.
func (m *Model) resetReleaseCursors() {
	for i := range m.relCursors {
		m.relCursors[i].Reset()
	}
}

// handleReleasesEnter jumps into the download flow with the MusicBrainz context
// synthesised from the cached row: no extra MusicBrainz call.
func (m *Model) handleReleasesEnter() tea.Cmd {
	if m.state != StateReleasesResults {
		return nil
	}
	rows := m.visibleReleases()
	if len(rows) == 0 {
		return nil
	}
	// The rows may have shrunk under the cursor since it last moved.
	cur := &m.relCursors[m.relTab]
	cur.ClampToBounds(len(rows))
	pos := cur.Pos()
	if m.slskdClient == nil {
		m.errorMsg = SlskdMissingMsg
		return nil
	}

	row := rows[pos]
	m.selectedArtist = &musicbrainz.Artist{Name: row.ArtistCreditName}
	m.selectedReleaseGroup = &musicbrainz.ReleaseGroup{
		ID:           row.ReleaseGroupMBID,
		Title:        row.ReleaseName,
		PrimaryType:  row.PrimaryType,
		FirstRelease: row.ReleaseDate,
	}
	m.fromReleases = true
	m.state = StateReleaseLoading
	m.errorMsg = ""
	m.statusMsg = "Loading releases..."
	return workflow.FetchReleasesCmd(m.mbClient, row.ReleaseGroupMBID)
}

// handleReleasesLoaded stores a fresh list, or the error of a failed refresh.
func (m *Model) handleReleasesLoaded(msg ReleasesLoadedMsg) (popup.Popup, tea.Cmd) {
	// A warm-up reload must not claim the ListenBrainz refresh is over.
	if msg.Refreshed {
		m.relRefreshing = false
	}
	m.relErr = errmsg.Format(errmsg.OpReleasesRefresh, msg.Err)
	if msg.Err == nil {
		m.relRows = msg.Rows
	}
	if m.state == StateReleasesLoading {
		m.state = StateReleasesResults
	}
	m.relCursors[m.relTab].ClampToBounds(len(m.visibleReleases()))
	return m, nil
}

// filterCount drops the discoveries filter when no Last.fm key is configured.
func (m *Model) filterCount() relFilter {
	if m.relDiscoveries {
		return relFilterCount
	}
	return relFilterCount - 1
}

// matchesQuery reports whether a row matches the incremental search.
func (m *Model) matchesQuery(r *ReleaseRow) bool {
	if m.relQuery == "" {
		return true
	}
	text := strings.ToLower(r.ArtistCreditName + " " + r.ReleaseName)
	return strings.Contains(text, strings.ToLower(m.relQuery))
}

// matchesFilter reports whether a row passes the current filter and search.
func (m *Model) matchesFilter(r *ReleaseRow) bool {
	if !m.matchesQuery(r) {
		return false
	}
	switch m.relFilter { //nolint:exhaustive // relFilterCount is a bound, not a filter
	case relFilterLibrary:
		return r.InLibrary
	case relFilterDiscoveries:
		return !r.InLibrary
	default:
		return true
	}
}

// visibleReleases returns the rows of the current tab and filter, ordered from
// nearest today outwards.
func (m *Model) visibleReleases() []ReleaseRow {
	today := time.Now().Format(time.DateOnly)
	var out []ReleaseRow
	for i := range m.relRows {
		r := m.relRows[i]
		if (r.ReleaseDate > today) != (m.relTab == relTabUpcoming) {
			continue
		}
		if !m.matchesFilter(&r) {
			continue
		}
		out = append(out, r)
	}
	// Both tabs run from nearest today outwards; a stable sort on the date
	// alone keeps the payload order (alphabetical by artist) within a day.
	sort.SliceStable(out, func(i, j int) bool {
		if m.relTab == relTabRecent {
			return out[i].ReleaseDate > out[j].ReleaseDate
		}
		return out[i].ReleaseDate < out[j].ReleaseDate
	})
	return out
}

// releasesBodyHeight is the body budget in rendered lines, date headers included.
func (m *Model) releasesBodyHeight() int {
	return max(m.Height()-9, 5)
}
