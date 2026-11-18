package similarartists

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeLastfm answers the artists similar to each artist of similar, and err
// for any other.
type fakeLastfm struct {
	similar map[string][]core.SimilarArtist
	err     error
}

func (f fakeLastfm) GetSimilarArtists(artist string, _ int) ([]core.SimilarArtist, error) {
	if s, ok := f.similar[artist]; ok {
		return s, nil
	}
	return nil, f.err
}

type fakeLibrary []string

func (f fakeLibrary) Artists() ([]string, error) { return f, nil }

func lastfm() fakeLastfm {
	return fakeLastfm{
		similar: map[string][]core.SimilarArtist{
			"Autechre": {{Name: "Boards of Canada", MatchScore: 0.9}, {Name: "Aphex Twin", MatchScore: 0.8}},
			"Low":      {{Name: "Galaxie 500", MatchScore: 0.7}},
		},
		err: errors.New("last.fm unavailable"),
	}
}

// openPopup opens the popup of artist: it returns it with the answer of
// its request, not delivered.
func openPopup(artist string) (*testutil.PopupHarness, *Model, tea.Msg) {
	m := New(lastfm(), fakeLibrary{"Aphex Twin"}, artist)
	h := testutil.NewPopupHarness(m)
	h.SetSize(80, 30)
	return h, m, testutil.ExecuteCmd(h.LastCommand())
}

// listingPopup is the popup of Autechre on its list, its cursor on the
// second row.
func listingPopup(t *testing.T) (*testutil.PopupHarness, *Model, tea.Msg) {
	t.Helper()
	h, m, answer := openPopup("Autechre")
	h.SendMsg(answer)
	h.SendDown()
	if m.cursor != 1 || m.totalItems() != 2 {
		t.Fatalf("cursor %d on %d artists, want the second of two", m.cursor, m.totalItems())
	}
	return h, m, answer
}

func TestSimilarArtists_ShowsTheArtistsOfItsOwnRequest(t *testing.T) {
	t.Parallel()
	h, _, answer := openPopup("Autechre")
	h.SendMsg(answer)
	for _, name := range []string{"Aphex Twin", "Boards of Canada"} {
		if !h.ViewContains(name) {
			t.Errorf("%q not shown:\n%s", name, testutil.StripANSI(h.View()))
		}
	}
}

func TestSimilarArtists_ShowsItsOwnFailure(t *testing.T) {
	t.Parallel()
	h, _, answer := openPopup("Unknown")
	h.SendMsg(answer)
	if !h.ViewContains("last.fm unavailable") {
		t.Errorf("the failure is not shown:\n%s", testutil.StripANSI(h.View()))
	}
}

// #308: the answer of a popup of another artist, closed since, leaves the
// list and its cursor.
func TestSimilarArtists_DropsTheAnswerOfAnotherPopup(t *testing.T) {
	t.Parallel()
	_, _, low := openPopup("Low")
	_, _, failed := openPopup("Unknown")
	h, m, _ := listingPopup(t)

	h.AssertDrops(t, low)
	h.AssertDrops(t, failed)
	if m.cursor != 1 || h.ViewContains("Galaxie 500") {
		t.Errorf("cursor %d, view:\n%s\nwant the list of Autechre, cursor on its second row", m.cursor, testutil.StripANSI(h.View()))
	}
}

func TestSimilarArtists_ALoadingPopupDropsTheAnswerOfAnotherPopup(t *testing.T) {
	t.Parallel()
	_, _, low := openPopup("Low")
	_, _, failed := openPopup("Unknown")
	h, m, own := openPopup("Autechre")

	h.AssertDrops(t, low)
	h.AssertDrops(t, failed)
	if !m.loading {
		t.Fatal("the popup stopped loading")
	}
	h.SendMsg(own)
	if !h.ViewContains("Boards of Canada") {
		t.Errorf("its own answer is not taken:\n%s", testutil.StripANSI(h.View()))
	}
}

func TestSimilarArtists_TakesItsAnswerOnce(t *testing.T) {
	t.Parallel()
	h, m, answer := listingPopup(t)
	h.AssertDrops(t, answer)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want it left on the second row", m.cursor)
	}
}
