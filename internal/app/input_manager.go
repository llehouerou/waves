// internal/app/input_manager.go
package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/search"
)

// SearchMode represents the current search mode.
type SearchMode int

const (
	// SearchModeOff indicates no search is active.
	SearchModeOff SearchMode = iota
	// SearchModeLocal indicates local search (current column items).
	SearchModeLocal
	// SearchModeDeep indicates deep search (full directory/library scan).
	SearchModeDeep
	// SearchModeAddToPlaylist indicates searching for a playlist to add tracks to.
	SearchModeAddToPlaylist
)

// InputManager manages search mode, key sequences, and input state.
type InputManager struct {
	search     search.Model
	searchMode SearchMode

	// AddToPlaylist state
	addToPlaylistTracks []int64

	// Deep search state
	scanChan   <-chan navigator.ScanResult
	cancelScan context.CancelFunc

	// Key sequence state (e.g., "g" prefix)
	pendingKeys string
}

// NewInputManager creates a new InputManager.
func NewInputManager() InputManager {
	return InputManager{
		search:     search.New(),
		searchMode: SearchModeOff,
	}
}

// --- Search Mode ---

// SearchMode returns the current search mode.
func (i *InputManager) SearchMode() SearchMode {
	return i.searchMode
}

// IsSearchActive returns true if any search mode is active.
func (i *InputManager) IsSearchActive() bool {
	return i.searchMode != SearchModeOff
}

// IsLocalSearch returns true if local search is active.
func (i *InputManager) IsLocalSearch() bool {
	return i.searchMode == SearchModeLocal
}

// IsDeepSearch returns true if deep search is active.
func (i *InputManager) IsDeepSearch() bool {
	return i.searchMode == SearchModeDeep
}

// IsAddToPlaylistSearch returns true if add-to-playlist search is active.
func (i *InputManager) IsAddToPlaylistSearch() bool {
	return i.searchMode == SearchModeAddToPlaylist
}

// StartLocalSearch enters local search mode with the given items.
func (i *InputManager) StartLocalSearch(items []search.Item) {
	i.searchMode = SearchModeLocal
	i.search.SetItems(items)
	i.search.SetLoading(false)
}

// StartDeepSearch enters deep search mode with loading state.
func (i *InputManager) StartDeepSearch(ctx context.Context, scanFn func(context.Context) <-chan navigator.ScanResult) <-chan navigator.ScanResult {
	i.searchMode = SearchModeDeep
	i.search.SetLoading(true)
	ctx, cancel := context.WithCancel(ctx)
	i.cancelScan = cancel
	i.scanChan = scanFn(ctx)
	return i.scanChan
}

// StartDeepSearchWithItems enters deep search mode with pre-loaded items.
func (i *InputManager) StartDeepSearchWithItems(items []search.Item) {
	i.searchMode = SearchModeDeep
	i.search.SetItems(items)
	i.search.SetLoading(false)
}

// StartDeepSearchWithMatcher enters deep search mode with pre-built matcher.
func (i *InputManager) StartDeepSearchWithMatcher(items []search.Item, matcher *search.TrigramMatcher) {
	i.searchMode = SearchModeDeep
	i.search.SetItemsWithMatcher(items, matcher)
	i.search.SetLoading(false)
}

// StartAddToPlaylistSearch enters add-to-playlist search mode.
func (i *InputManager) StartAddToPlaylistSearch(trackIDs []int64, playlistItems []search.Item) {
	i.searchMode = SearchModeAddToPlaylist
	i.addToPlaylistTracks = trackIDs
	i.search.SetItems(playlistItems)
	i.search.SetLoading(false)
}

// EndSearch exits search mode and resets state.
func (i *InputManager) EndSearch() {
	i.searchMode = SearchModeOff
	i.addToPlaylistTracks = nil
	if i.cancelScan != nil {
		i.cancelScan()
		i.cancelScan = nil
	}
	i.scanChan = nil
	i.search.Reset()
}

// UpdateScanResults updates search with new scan results.
func (i *InputManager) UpdateScanResults(items []search.Item, loading bool) {
	i.search.SetItems(items)
	i.search.SetLoading(loading)
}

// ScanChan returns the current scan channel (may be nil).
func (i *InputManager) ScanChan() <-chan navigator.ScanResult {
	return i.scanChan
}

// CancelDeepScan cancels any active deep scan.
func (i *InputManager) CancelDeepScan() {
	if i.cancelScan != nil {
		i.cancelScan()
		i.cancelScan = nil
	}
	i.scanChan = nil
}

// AddToPlaylistTracks returns the track IDs pending for add-to-playlist.
func (i *InputManager) AddToPlaylistTracks() []int64 {
	return i.addToPlaylistTracks
}

// ClearAddToPlaylistTracks clears the pending track IDs.
func (i *InputManager) ClearAddToPlaylistTracks() {
	i.addToPlaylistTracks = nil
}

// --- Search Component Access ---

// Search returns a pointer to the search model for direct access.
func (i *InputManager) Search() *search.Model {
	return &i.search
}

// UpdateSearch updates the search model with a message.
func (i *InputManager) UpdateSearch(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i.search, cmd = i.search.Update(msg)
	return cmd
}

// SearchView returns the search component's view.
func (i *InputManager) SearchView() string {
	return i.search.View()
}

// --- Key Sequences ---

// PendingKeys returns the current pending key sequence.
func (i *InputManager) PendingKeys() string {
	return i.pendingKeys
}

// HasPendingKeys returns true if there are pending keys.
func (i *InputManager) HasPendingKeys() bool {
	return i.pendingKeys != ""
}

// StartKeySequence starts a new key sequence with the given key.
func (i *InputManager) StartKeySequence(key string) {
	i.pendingKeys = key
}

// ClearKeySequence clears any pending key sequence.
func (i *InputManager) ClearKeySequence() {
	i.pendingKeys = ""
}

// IsKeySequence returns true if the pending keys match the given sequence.
func (i *InputManager) IsKeySequence(seq string) bool {
	return i.pendingKeys == seq
}

// --- Normal Mode ---

// IsNormalMode returns true if not in search mode and no key sequence is pending.
func (i *InputManager) IsNormalMode() bool {
	return i.searchMode == SearchModeOff && i.pendingKeys == ""
}

// SetSize updates the search component dimensions.
func (i *InputManager) SetSize(msg tea.WindowSizeMsg) {
	i.search, _ = i.search.Update(msg)
}
