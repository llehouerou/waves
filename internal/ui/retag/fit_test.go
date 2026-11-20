package retag

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/testutil"
)

// === The tag preview in a short terminal (issue #316) ===

// The popup manager sizes the retag popup of an 80x24 terminal 64x16, and
// lets it grow to 24 rows; that of a 120x40 terminal 96x28, up to 40 rows.
func sizeFor(m *Model, terminalWidth, terminalHeight int) {
	m.SetSize(terminalWidth*80/100, terminalHeight*70/100)
	m.SetMaxHeight(terminalHeight)
}

var tagRange = regexp.MustCompile(`\((\d+)-(\d+) of 19\)`)

// tagsShown is the range of tags the tag preview says it shows, "" when it
// says none.
func tagsShown(h *testutil.PopupHarness) string {
	return tagRange.FindString(testutil.StripANSI(h.View()))
}

// showsTag reports whether the tag preview lists the tag named field.
func showsTag(h *testutil.PopupHarness, field string) bool {
	for line := range strings.SplitSeq(testutil.StripANSI(h.View()), "\n") {
		if strings.HasPrefix(line, field+"  ") {
			return true
		}
	}
	return false
}

func TestRetag_TagPreviewScrollsItsTagsInAShortTerminal(t *testing.T) {
	h, m, _ := previewingPopup(t)
	sizeFor(m, 80, 24)

	assertView(t, h, "[Enter] Retag")
	if rows := testutil.BoxRows(h.View(), m.Width(), m.Height()); rows > 24 {
		t.Errorf("box of %d rows, want at most 24", rows)
	}
	r := tagsShown(h)
	line := testutil.FindLine(testutil.StripANSI(h.View()), r)
	if !strings.HasPrefix(r, "(1-") || !strings.Contains(line, "[j/k ctrl+d/u g/G] Scroll") {
		t.Fatalf("range line %q, want the first tags and the scroll keys", line)
	}
	if !showsTag(h, "Artist") || showsTag(h, "MB Group ID") {
		t.Error("the first page does not show Artist alone")
	}

	h.SendKey("j")
	if got := tagsShown(h); !strings.HasPrefix(got, "(2-") {
		t.Errorf("j: %q, want from tag 2", got)
	}
	h.SendSpecialKey(tea.KeyUp)
	if got := tagsShown(h); got != r {
		t.Errorf("up: %q, want %q", got, r)
	}
	h.SendSpecialKey(tea.KeyCtrlD)
	if got := tagsShown(h); got == r {
		t.Error("ctrl+d did not scroll")
	}
	h.SendSpecialKey(tea.KeyCtrlU)
	h.SendKey("G")
	if !showsTag(h, "MB Group ID") || !strings.HasSuffix(tagsShown(h), "-19 of 19)") {
		t.Errorf("G: %q, want the last tag", tagsShown(h))
	}
	assertView(t, h, "[Enter] Retag")
	h.SendSpecialKey(tea.KeyHome)
	if !showsTag(h, "Artist") {
		t.Error("home: the first tag is not shown")
	}
	h.SendSpecialKey(tea.KeyEnd)
	h.SendKey("g")
	if !showsTag(h, "Artist") {
		t.Error("g: the first tag is not shown")
	}
	if len(h.Commands()) != 0 || m.state != StateTagPreview {
		t.Errorf("scrolling: state %d, %d commands", m.state, len(h.Commands()))
	}
}

func TestRetag_TagPreviewWaitingForItsCoverKeepsItsHelpLine(t *testing.T) {
	h, m, _, _ := previewingReleaseOn(t, fakeMB{covers: trustCovers()})
	sizeFor(m, 80, 24)
	assertView(t, h, "Waiting for cover art...")
	if tagsShown(h) == "" {
		t.Error("no range")
	}
}

func TestRetag_TagPreviewInATallTerminalShowsEveryTag(t *testing.T) {
	h, m, _ := previewingPopup(t)
	sizeFor(m, 120, 40)
	if r := tagsShown(h); r != "" {
		t.Errorf("range %q, want every tag", r)
	}
	for _, field := range []string{"Artist", "Genre", "MB Group ID"} {
		if !showsTag(h, field) {
			t.Errorf("%s is not shown", field)
		}
	}
	assertView(t, h, "[Enter] Retag")
}

// A new release previewed shows its tags from the first.
func TestRetag_ReleasePickedBackShowsItsTagsFromTheFirst(t *testing.T) {
	h, m, _, cover := previewingReleaseOn(t, fakeMB{covers: trustCovers()})
	h.ExecuteAndSend(cover)
	sizeFor(m, 80, 24)
	h.SendKey("G")
	listReleases(t, h, m)
	h.ExecuteAndSend(pickOther(t, h, m))
	if !showsTag(h, "Artist") {
		t.Errorf("the preview of other starts at %q", tagsShown(h))
	}
}

// The scroll keys belong to the tag preview: in the lists, j/k move the
// cursor as before, and the others do nothing.
func TestRetag_ScrollKeysDoNothingInTheLists(t *testing.T) {
	h, m := listingPopup(t, mixedGroups()...)
	sizeFor(m, 80, 24)
	before := m.releaseGroupCursor.Pos()
	h.SendKey("j")
	if m.releaseGroupCursor.Pos() != before+1 {
		t.Errorf("j: cursor at %d, want %d", m.releaseGroupCursor.Pos(), before+1)
	}
	h.SendKey("k")
	view := h.View()
	for _, key := range []tea.KeyType{tea.KeyCtrlD, tea.KeyCtrlU, tea.KeyHome, tea.KeyEnd} {
		h.SendSpecialKey(key)
	}
	h.SendKey("g")
	h.SendKey("G")
	if h.View() != view || m.releaseGroupCursor.Pos() != before || m.state != StateReleaseGroupResults {
		t.Errorf("the scroll keys changed the list: cursor %d, state %d", m.releaseGroupCursor.Pos(), m.state)
	}
	if len(h.Commands()) != 0 {
		t.Errorf("the scroll keys started %d commands", len(h.Commands()))
	}
}
