package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/albumart"
)

// Bubble Tea only writes the last view of a frame, and a track change loads
// the art twice (the queue moved, then the track started). The removal of the
// previous cover, written once, was lost when both loads landed in one frame
// and the new track had no cover to place: the old one stayed on screen.
func TestView_KeepsRemovingThePreviousCoverOfATrackWithout(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // albumart.New opens its disk cache there
	m := newIntegrationTestModel(t)
	m.loadingState = loadingDone
	m.AlbumArt = albumart.New(&albumart.KittyProtocol{})
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m, _ = updateModel(t, m, AlbumArtLoadedMsg{Path: "/a.mp3", PNG: []byte("png")})
	m, _ = updateModel(t, m, AlbumArtLoadedMsg{Path: "/b.mp3"})
	removal := m.albumArtPendingTransmit
	if removal == "" {
		t.Fatal("precondition: a track without a cover removes the previous one")
	}

	m, _ = updateModel(t, m, AlbumArtLoadedMsg{Path: "/b.mp3"})

	if !strings.Contains(m.View(), removal) {
		t.Error("the view no longer removes the previous cover: it stays on screen")
	}
}
