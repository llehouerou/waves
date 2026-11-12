package download

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
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

// Releases is what the popup asks of the new releases: the list, where the
// work that keeps it fresh is, and to start that work. That work reports
// through core.ReleasesRefreshed and core.WarmupFinished, which the app hands
// to Update.
type Releases interface {
	List() ([]core.Release, error)
	Status() core.ReleasesStatus
	Refresh(force bool)
}

// ReleasesLoadedMsg carries the new releases as the core lists them, or why
// they could not be read (the rows shown stay untouched then).
type ReleasesLoadedMsg struct {
	Rows []core.Release
	Err  error
}

// Library is what the popup reads of the library: the albums already owned.
type Library interface {
	AlbumsForArtistNormalized(albumArtist string) map[string]struct{}
}

// loadReleasesCmd reads the new releases off the UI goroutine.
func loadReleasesCmd(rel Releases) tea.Cmd {
	return func() tea.Msg {
		rows, err := rel.List()
		return ReleasesLoadedMsg{Rows: rows, Err: err}
	}
}

// StartReleases opens phase 0 on the new releases of rel, and asks for them
// to be brought up to date: the core starts what is needed, and says where it
// is.
func (m *Model) StartReleases(rel Releases) tea.Cmd {
	m.newReleases = rel
	rel.Refresh(false)
	status := rel.Status()
	m.relDiscoveries = status.Discoveries
	m.relRefreshing = status.Refreshing
	m.relErr = errmsg.Format(errmsg.OpReleasesRefresh, status.Err)
	m.state = StateReleasesLoading
	m.searchInput.Blur()
	return loadReleasesCmd(rel)
}

// handleReleasesEvent takes the end of the work that keeps the list fresh. A
// popup that never opened the list has nothing to do with it.
func (m *Model) handleReleasesEvent(e tea.Msg) (popup.Popup, tea.Cmd) {
	if m.newReleases == nil {
		return m, nil
	}
	if refreshed, ok := e.(core.ReleasesRefreshed); ok {
		m.relRefreshing = false
		m.relErr = errmsg.Format(errmsg.OpReleasesRefresh, refreshed.Err)
		if refreshed.Err != nil {
			return m, nil // nothing changed: the list keeps what it shows
		}
	}
	return m, loadReleasesCmd(m.newReleases)
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
		if m.newReleases != nil {
			m.relRefreshing = true
			m.relErr = ""
			m.newReleases.Refresh(true)
		}
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
	if !m.slskdConfigured() {
		m.errorMsg = SlskdMissingMsg
		return nil
	}

	row := rows[pos]
	m.selectedArtist = &core.MBArtist{Name: row.ArtistCreditName}
	m.selectedReleaseGroup = &core.MBReleaseGroup{
		ID:           row.ReleaseGroupMBID,
		Title:        row.ReleaseName,
		PrimaryType:  row.PrimaryType,
		FirstRelease: row.ReleaseDate,
	}
	m.fromReleases = true
	m.state = StateReleaseLoading
	m.errorMsg = ""
	m.statusMsg = "Loading releases..."
	return mbworkflow.FetchReleasesCmd(m.mbClient, row.ReleaseGroupMBID)
}

// handleReleasesLoaded stores the list read, or tells why it could not be.
// A list that was read says nothing of the last refresh: its failure, if
// any, stays told.
func (m *Model) handleReleasesLoaded(msg ReleasesLoadedMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.relErr = errmsg.Format(errmsg.OpReleasesRefresh, msg.Err)
	} else {
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
func (m *Model) matchesQuery(r *core.Release) bool {
	if m.relQuery == "" {
		return true
	}
	text := strings.ToLower(r.ArtistCreditName + " " + r.ReleaseName)
	return strings.Contains(text, strings.ToLower(m.relQuery))
}

// matchesFilter reports whether a row passes the current filter and search.
func (m *Model) matchesFilter(r *core.Release) bool {
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
func (m *Model) visibleReleases() []core.Release {
	today := time.Now().Format(time.DateOnly)
	var out []core.Release
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
