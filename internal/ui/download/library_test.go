package download

import (
	"errors"
	"testing"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeLibrary holds the albums of each artist it was given, in the form the
// library compares them in, or fails every read with err when it is set.
type fakeLibrary struct {
	albums map[string][]string
	err    error
}

func (f fakeLibrary) AlbumsForArtistNormalized(albumArtist string) (map[string]struct{}, error) {
	if f.err != nil {
		return nil, f.err
	}
	set := map[string]struct{}{}
	for _, album := range f.albums[albumArtist] {
		set[core.NormalizeTitle(album)] = struct{}{}
	}
	return set, nil
}

// libraryPopup is a fresh download popup on fakeMB and lib.
func libraryPopup(lib Library) (*testutil.PopupHarness, *Model) {
	m := New(&fakeDownloads{}, fakeMB{}, FilterConfig{}, lib)
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m), m
}

func TestReleaseGroups_AreNotListedWhenTheAlbumsTheLibraryHoldsCannotBeRead(t *testing.T) {
	failed := errors.New("no such table: library_tracks")
	h, m := libraryPopup(fakeLibrary{err: failed})
	searchNirvana(t, h, m)

	stepTo(t, h, m, h.SendEnter(), StateArtistResults)

	if len(m.releaseGroups) != 0 {
		t.Errorf("release groups = %+v, want none listed", m.releaseGroups)
	}
	if err := h.AssertViewContains(errmsg.Format(errmsg.OpLibraryLoad, failed)); err != "" {
		t.Error(err)
	}
	// Enter on the artist asks again.
	if cmd := h.SendEnter(); cmd == nil || m.state != StateReleaseGroupLoading {
		t.Errorf("Enter on the artist: state %d, command %v; want the release groups asked again", m.state, cmd)
	}
}

func TestReleaseGroups_TheLibraryHoldsAreMarkedInLibrary(t *testing.T) {
	h, m := libraryPopup(fakeLibrary{albums: map[string][]string{"Nirvana": {"Unplugged"}}})
	listGroups(t, h, m)

	if err := h.AssertViewContains(icons.InLibrary() + " Unplugged"); err != "" {
		t.Error(err)
	}
}
