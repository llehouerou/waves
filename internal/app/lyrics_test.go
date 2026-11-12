package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// The lyrics of the track playing, as the core fetches them. They are in a
// .lrc file next to the track here: the core then asks lrclib nothing.
func TestLyrics_ThePopupShowsWhatTheCoreFetches(t *testing.T) {
	m := newLibraryTestModel(t, nil)
	track := filepath.Join(t.TempDir(), "02 - Canada.mp3")
	lrc := strings.TrimSuffix(track, ".mp3") + ".lrc"
	if err := os.WriteFile(lrc, []byte("[00:12.50]You can't get away\n[00:20.00]From Canada\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pb := m.Core.Playback()
	pb.AddTracks(core.Track{Path: track, Artist: "Low", Title: "Canada"})
	pb.QueueMoveTo(0)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}

	m, _ = updateModel(t, m, keyMsg("f"))
	m = press(t, m, "y")

	pop := m.Popups.Get(popupctl.Lyrics)
	if pop == nil {
		t.Fatalf("the lyrics popup is not open: %s", m.Popups.ErrorMsg())
	}
	if view := testutil.StripANSI(pop.View()); !strings.Contains(view, "You can't get away") || !strings.Contains(view, "From Canada") {
		t.Errorf("the popup does not show the lyrics of the track:\n%s", view)
	}
}
