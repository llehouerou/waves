package lyrics

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

const testTrackPath = "/test/track.mp3"

func newTestLyricsPopup() *testutil.PopupHarness {
	m := New(nil) // nil source - we'll set state manually
	m.SetSize(80, 40)
	return testutil.NewPopupHarness(m)
}

func newLoadedLyricsPopup(lines []core.LyricsLine) *testutil.PopupHarness {
	h, _, _ := fetchedPopup(&coreLyrics{lyrics: &core.TrackLyrics{Lines: lines}})
	return h
}

// fetchedPopup is a popup on src, whose lyrics of the track at testTrackPath
// came: it returns them.
func fetchedPopup(src *coreLyrics) (*testutil.PopupHarness, *Model, tea.Msg) {
	m := New(src)
	m.SetSize(80, 40)
	h := testutil.NewPopupHarness(m)
	msg, _ := h.ExecuteAndSend(m.SetTrack(testTrackPath, "Low", "Canada", "Trust", 0))
	return h, m, msg
}

func getAction(t *testing.T, h *testutil.PopupHarness) action.Action {
	t.Helper()
	cmd := h.LastCommand()
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	msg := testutil.ExecuteCmd(cmd)
	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}
	return actionMsg.Action
}

// Close tests

func TestLyrics_CloseWithEscape(t *testing.T) {
	h := newTestLyricsPopup()

	h.SendEscape()

	act := getAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Fatalf("expected Close, got %T", act)
	}
}

func TestLyrics_CloseWithQ(t *testing.T) {
	h := newTestLyricsPopup()

	h.SendKey("q")

	act := getAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Fatalf("expected Close, got %T", act)
	}
}

// Passthrough tests

func TestLyrics_UnhandledKeyPassthrough(t *testing.T) {
	h := newTestLyricsPopup()

	h.SendKey("p") // Unhandled key - should passthrough

	act := getAction(t, h)
	passthrough, ok := act.(Passthrough)
	if !ok {
		t.Fatalf("expected Passthrough, got %T", act)
	}
	if passthrough.Key.String() != "p" {
		t.Errorf("Key = %q, want 'p'", passthrough.Key.String())
	}
}

// Scroll tests

func TestLyrics_ScrollDownWithJ(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	initialOffset := m.scrollOffset

	h.SendKey("j")
	h.SendKey("j")

	if m.scrollOffset <= initialOffset {
		t.Error("scroll offset should increase after pressing j")
	}
}

func TestLyrics_ScrollDownWithArrow(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	initialOffset := m.scrollOffset

	h.SendDown()
	h.SendDown()

	if m.scrollOffset <= initialOffset {
		t.Error("scroll offset should increase after pressing down")
	}
}

func TestLyrics_ScrollUpWithK(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// First scroll down
	h.SendKey("j")
	h.SendKey("j")
	h.SendKey("j")
	afterDown := m.scrollOffset

	// Then scroll up
	h.SendKey("k")

	if m.scrollOffset >= afterDown {
		t.Error("scroll offset should decrease after pressing k")
	}
}

func TestLyrics_ScrollUpWithArrow(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// First scroll down
	h.SendDown()
	h.SendDown()
	h.SendDown()
	afterDown := m.scrollOffset

	// Then scroll up
	h.SendUp()

	if m.scrollOffset >= afterDown {
		t.Error("scroll offset should decrease after pressing up")
	}
}

func TestLyrics_ScrollToTopWithG(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// Scroll down first
	h.SendKey("j")
	h.SendKey("j")
	h.SendKey("j")

	// Go to top
	h.SendKey("g")

	if m.scrollOffset != 0 {
		t.Errorf("scrollOffset = %d, want 0", m.scrollOffset)
	}
}

func TestLyrics_ScrollToBottomWithShiftG(t *testing.T) {
	lines := makeSampleLines(30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	h.SendKey("G")

	maxScroll := m.maxScroll()
	if m.scrollOffset != maxScroll {
		t.Errorf("scrollOffset = %d, want %d (maxScroll)", m.scrollOffset, maxScroll)
	}
}

// Auto-scroll tests

func TestLyrics_ScrollDisablesAutoScroll(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	if !m.autoScroll {
		t.Skip("autoScroll already disabled")
	}

	h.SendKey("j")

	if m.autoScroll {
		t.Error("autoScroll should be disabled after manual scroll")
	}
}

func TestLyrics_CReenablesAutoScroll(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30)
	h := newLoadedLyricsPopup(lines)

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// Disable auto-scroll by scrolling
	h.SendKey("j")
	if m.autoScroll {
		t.Skip("autoScroll not disabled")
	}

	// Re-enable with 'c'
	h.SendKey("c")

	if !m.autoScroll {
		t.Error("autoScroll should be enabled after pressing c")
	}
}

// FetchedMsg handling tests

func TestLyrics_FetchedMsgLoadsLyrics(t *testing.T) {
	h, _, _ := fetchedPopup(&coreLyrics{lyrics: &core.TrackLyrics{Lines: []core.LyricsLine{{Text: "Hello world"}}}})

	if err := h.AssertViewContains("Hello world"); err != "" {
		t.Error(err)
	}
}

// The lyrics of the track shown before, come after a second SetTrack, are
// dropped: the popup takes those of the track it shows.
func TestLyrics_FetchedMsgIgnoresStaleResults(t *testing.T) {
	src := &coreLyrics{lyrics: &core.TrackLyrics{Lines: []core.LyricsLine{{Text: "Old lyrics"}}}}
	m := New(src)
	m.SetSize(80, 40)
	h := testutil.NewPopupHarness(m)
	earlier := testutil.ExecuteCmd(m.SetTrack("/test/old.mp3", "Low", "Lazy", "Trust", 0))
	src.lyrics = &core.TrackLyrics{Lines: []core.LyricsLine{{Text: "Current lyrics"}}}
	later := m.SetTrack("/test/current.mp3", "Low", "Canada", "Trust", 0)

	h.AssertDrops(t, earlier)
	if m.state != StateLoading {
		t.Fatalf("state = %d, want the lyrics of the current track loading", m.state)
	}

	h.ExecuteAndSend(later)
	h.AssertDrops(t, earlier)
	if err := h.AssertViewContains("Current lyrics"); err != "" {
		t.Error(err)
	}
}

// closedPopupsLyrics are the lyrics of the track at testTrackPath, and their
// failed fetch, each fetched for a popup closed before they came.
func closedPopupsLyrics() map[string]tea.Msg {
	fetch := func(src *coreLyrics) tea.Msg {
		m := New(src)
		return testutil.ExecuteCmd(m.SetTrack(testTrackPath, "Low", "Canada", "Trust", 0))
	}
	return map[string]tea.Msg{
		"lyrics":       fetch(&coreLyrics{lyrics: &core.TrackLyrics{Lines: []core.LyricsLine{{Text: "Closed lyrics"}}}}),
		"failed fetch": fetch(&coreLyrics{err: testError("connection timeout")}),
	}
}

// A popup drops the lyrics fetched for a popup closed since, of the same
// track, while it waits for its own and once it shows them.
func TestLyrics_DropsTheLyricsOfAClosedPopup(t *testing.T) {
	for name, msg := range closedPopupsLyrics() {
		t.Run("loading/"+name, func(t *testing.T) {
			src := &coreLyrics{lyrics: &core.TrackLyrics{Lines: makeSampleLines(50)}}
			m := New(src)
			m.SetSize(80, 40)
			h := testutil.NewPopupHarness(m)
			own := m.SetTrack(testTrackPath, "Low", "Canada", "Trust", 0)

			h.AssertDrops(t, msg)
			if m.state != StateLoading {
				t.Fatalf("state = %d, want loading", m.state)
			}
			h.ExecuteAndSend(own)
			if m.state != StateLoaded || len(m.lyrics.Lines) != 50 {
				t.Errorf("state = %d, want its own lyrics", m.state)
			}
		})
		t.Run("loaded/"+name, func(t *testing.T) {
			h, m, _ := fetchedPopup(&coreLyrics{lyrics: &core.TrackLyrics{Lines: makeSampleLines(50)}})
			h.SendKey("j")
			h.SendKey("j")

			h.AssertDrops(t, msg)
			if m.scrollOffset != 2 || m.state != StateLoaded {
				t.Errorf("scroll %d, state %d; want its lyrics, scrolled by 2", m.scrollOffset, m.state)
			}
		})
	}
}

// The lyrics shown, delivered again, leave the scroll where it is.
func TestLyrics_TakesItsLyricsOnce(t *testing.T) {
	h, m, own := fetchedPopup(&coreLyrics{lyrics: &core.TrackLyrics{Lines: makeSampleLines(50)}})
	h.SendKey("j")
	h.SendKey("j")

	h.AssertDrops(t, own)
	if m.scrollOffset != 2 {
		t.Errorf("scroll = %d, want it left at 2", m.scrollOffset)
	}
}

func TestLyrics_FetchedMsgHandlesNotFound(t *testing.T) {
	h, _, _ := fetchedPopup(&coreLyrics{err: core.ErrNoLyrics})

	if err := h.AssertViewContains("No lyrics found"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_FetchedMsgHandlesError(t *testing.T) {
	h, _, _ := fetchedPopup(&coreLyrics{err: testError("connection timeout")})

	if err := h.AssertViewContains("Error loading lyrics"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("connection timeout"); err != "" {
		t.Error(err)
	}
}

// View tests

func TestLyrics_ViewShowsTitle(t *testing.T) {
	h := newTestLyricsPopup()

	if err := h.AssertViewContains("Lyrics"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_ViewShowsLoadingState(t *testing.T) {
	h := newTestLyricsPopup()

	if err := h.AssertViewContains("Loading lyrics"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_ViewShowsCloseHint(t *testing.T) {
	h := newTestLyricsPopup()

	if err := h.AssertViewContains("esc close"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_ViewShowsScrollHint(t *testing.T) {
	lines := makeSampleLines(50) // Need more than visibleHeight (30) to enable scrolling
	h := newLoadedLyricsPopup(lines)

	if err := h.AssertViewContains("j/k scroll"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_ViewEmptyWhenNoSize(t *testing.T) {
	m := New(nil)
	// Don't set size
	h := testutil.NewPopupHarness(m)

	if h.View() != "" {
		t.Errorf("view = %q, want empty when no size", h.View())
	}
}

// SetPosition test

func TestLyrics_SetPositionUpdatesCurrentLine(t *testing.T) {
	// Create synced lyrics with timestamps
	syncedLines := []core.LyricsLine{
		{Time: 0, Text: "Line one"},
		{Time: 5 * time.Second, Text: "Line two"},
		{Time: 10 * time.Second, Text: "Line three"},
	}
	_, m, _ := fetchedPopup(&coreLyrics{lyrics: &core.TrackLyrics{Lines: syncedLines}})

	// Set position to 7 seconds - should be on line 2 (index 1)
	m.SetPosition(7 * time.Second)

	if m.currentLine != 1 {
		t.Errorf("currentLine = %d, want 1", m.currentLine)
	}
}

// Fetching through the core

// coreLyrics stands in for the core: it has the lyrics, or the error, it is
// given, and remembers the track it was asked about.
type coreLyrics struct {
	lyrics *core.TrackLyrics
	err    error
	asked  core.LyricsTrack
}

func (c *coreLyrics) Fetch(track core.LyricsTrack) (*core.TrackLyrics, error) {
	c.asked = track
	return c.lyrics, c.err
}

func TestLyrics_SetTrackAsksTheCoreForTheLyricsAndShowsThem(t *testing.T) {
	src := &coreLyrics{lyrics: &core.TrackLyrics{Lines: []core.LyricsLine{{Text: "From Canada"}}}}
	m := New(src)
	m.SetSize(80, 40)
	h := testutil.NewPopupHarness(m)

	cmd := m.SetTrack("/music/canada.mp3", "Low", "Canada", "Trust", 224*time.Second)
	h.SendMsg(testutil.ExecuteCmd(cmd))

	want := core.LyricsTrack{FilePath: "/music/canada.mp3", Artist: "Low", Title: "Canada", Album: "Trust", Duration: 224 * time.Second}
	if src.asked != want {
		t.Errorf("the core was asked about %+v, want %+v", src.asked, want)
	}
	if err := h.AssertViewContains("From Canada"); err != "" {
		t.Error(err)
	}
}

// Nobody having the lyrics of a track is not a failure to fetch them.
func TestLyrics_NoLyricsIsNotShownAsAnError(t *testing.T) {
	m := New(&coreLyrics{err: core.ErrNoLyrics})
	m.SetSize(80, 40)
	h := testutil.NewPopupHarness(m)

	h.SendMsg(testutil.ExecuteCmd(m.SetTrack(testTrackPath, "Low", "Canada", "Trust", 0)))

	if err := h.AssertViewContains("No lyrics found"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewNotContains("Error loading lyrics"); err != "" {
		t.Error(err)
	}
}

// Helper functions

func makeSampleLines(count int) []core.LyricsLine {
	lines := make([]core.LyricsLine, count)
	for i := range count {
		lines[i] = core.LyricsLine{
			Time: time.Duration(i) * time.Second,
			Text: "Sample lyric line",
		}
	}
	return lines
}

type testError string

func (e testError) Error() string { return string(e) }
