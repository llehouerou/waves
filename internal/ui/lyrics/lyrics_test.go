package lyrics

import (
	"testing"
	"time"

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
	m := New(nil)
	m.SetSize(80, 40)
	m.trackPath = testTrackPath // Must match FetchedMsg.TrackPath

	// Simulate loaded lyrics via FetchedMsg
	h := testutil.NewPopupHarness(m)
	h.SendMsg(FetchedMsg{TrackPath: testTrackPath, Lyrics: &core.TrackLyrics{Lines: lines}})

	return h
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
	m := New(nil)
	m.SetSize(80, 40)
	m.trackPath = testTrackPath
	h := testutil.NewPopupHarness(m)

	h.SendMsg(FetchedMsg{
		TrackPath: testTrackPath,
		Lyrics:    &core.TrackLyrics{Lines: []core.LyricsLine{{Text: "Hello world"}}},
	})

	if err := h.AssertViewContains("Hello world"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_FetchedMsgIgnoresStaleResults(t *testing.T) {
	m := New(nil)
	m.SetSize(80, 40)
	m.trackPath = "/test/current.mp3" // Current track
	h := testutil.NewPopupHarness(m)

	// Send result for a different track
	h.SendMsg(FetchedMsg{
		TrackPath: "/test/old.mp3", // Different track
		Lyrics:    &core.TrackLyrics{Lines: []core.LyricsLine{{Text: "Old lyrics"}}},
	})

	// Should still be in loading state, not showing old lyrics
	if err := h.AssertViewNotContains("Old lyrics"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_FetchedMsgHandlesNotFound(t *testing.T) {
	m := New(nil)
	m.SetSize(80, 40)
	m.trackPath = testTrackPath
	h := testutil.NewPopupHarness(m)

	h.SendMsg(FetchedMsg{TrackPath: testTrackPath, Err: core.ErrNoLyrics})

	if err := h.AssertViewContains("No lyrics found"); err != "" {
		t.Error(err)
	}
}

func TestLyrics_FetchedMsgHandlesError(t *testing.T) {
	m := New(nil)
	m.SetSize(80, 40)
	m.trackPath = testTrackPath
	h := testutil.NewPopupHarness(m)

	h.SendMsg(FetchedMsg{
		TrackPath: testTrackPath,
		Err:       testError("connection timeout"),
	})

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
	m := New(nil)
	m.SetSize(80, 40)
	m.trackPath = testTrackPath

	// Create synced lyrics with timestamps
	syncedLines := []core.LyricsLine{
		{Time: 0, Text: "Line one"},
		{Time: 5 * time.Second, Text: "Line two"},
		{Time: 10 * time.Second, Text: "Line three"},
	}

	h := testutil.NewPopupHarness(m)
	h.SendMsg(FetchedMsg{TrackPath: testTrackPath, Lyrics: &core.TrackLyrics{Lines: syncedLines}})

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
