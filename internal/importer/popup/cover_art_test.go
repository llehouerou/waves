package popup

import (
	"errors"
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeMB serves releases and covers by release ID, and records the covers asked for.
type fakeMB struct {
	releases map[string]*musicbrainz.ReleaseDetails
	covers   map[string][]byte
	fetched  []string
}

func (f *fakeMB) GetRelease(id string) (*musicbrainz.ReleaseDetails, error) {
	if r, ok := f.releases[id]; ok {
		return r, nil
	}
	return nil, errors.New("unavailable")
}

func (f *fakeMB) GetCoverArt(id string) ([]byte, error) {
	f.fetched = append(f.fetched, id)
	return f.covers[id], nil
}

func releaseWithTracks(id string) *musicbrainz.ReleaseDetails {
	return &musicbrainz.ReleaseDetails{
		Release: musicbrainz.Release{ID: id, Title: "Album " + id, Artist: "Artist", ArtistID: "artist", Status: "Official"},
		Tracks:  []musicbrainz.Track{{Position: 1, Title: "One"}, {Position: 2, Title: "Two"}, {Position: 3, Title: "Three"}},
	}
}

// run executes cmd, and every command of a batch, and returns the messages.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, run(c)...)
	}
	return msgs
}

// runOne executes cmd, which must produce a single T.
func runOne[T tea.Msg](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	msgs := run(cmd)
	if len(msgs) != 1 {
		t.Fatalf("got messages %#v, want one %T", msgs, *new(T))
	}
	msg, ok := msgs[0].(T)
	if !ok {
		t.Fatalf("got %T, want %T", msgs[0], *new(T))
	}
	return msg
}

// openImport opens the import popup of sampleDownload (release abc123)
// against fake, and returns the cover fetch its Init started.
func openImport(t *testing.T, fake *fakeMB) (*testutil.PopupHarness, *Model, CoverArtFetchedMsg) {
	t.Helper()
	m := New(sampleDownload(), t.TempDir(), sampleLibrarySources(), nil, rename.Config{})
	m.mbClient = fake
	m.SetSize(100, 40)
	h := testutil.NewPopupHarness(m)
	var cover *CoverArtFetchedMsg
	for _, msg := range run(h.LastCommand()) {
		if c, ok := msg.(CoverArtFetchedMsg); ok {
			cover = &c
		}
	}
	if cover == nil {
		t.Fatal("Init fetched no cover")
	}
	return h, m, *cover
}

// readTags delivers tags carrying releaseID, runs the release refresh they
// start, and returns what the refresh result asked for.
func readTags(t *testing.T, h *testutil.PopupHarness, releaseID string) tea.Cmd {
	t.Helper()
	fileTags := make([]tags.FileInfo, 3)
	for i := range fileTags {
		fileTags[i].MBReleaseID = releaseID
	}
	refresh := h.SendMsg(TagsReadMsg{DownloadID: 1, Tags: fileTags})
	return h.SendMsg(runOne[MBReleaseRefreshedMsg](t, refresh))
}

func importedCover(m *Model) string {
	return string(m.importParams("/music/library1", 0, "/src/01.flac").CoverArt)
}

// File tags switching the import to another release must bring that
// release's cover; the first release's cover, whenever it arrives, is dropped.
func TestImport_SwitchedReleaseImportsItsOwnCover(t *testing.T) {
	fake := &fakeMB{
		releases: map[string]*musicbrainz.ReleaseDetails{"B": releaseWithTracks("B")},
		covers:   map[string][]byte{"abc123": []byte("cover A"), "B": []byte("cover B")},
	}
	h, m, coverA := openImport(t, fake)
	h.SendMsg(coverA) // A's cover arrives before the switch

	coverB := runOne[CoverArtFetchedMsg](t, readTags(t, h, "B"))
	if want := []string{"abc123", "B"}; !slices.Equal(fake.fetched, want) {
		t.Fatalf("covers fetched for %v, want %v", fake.fetched, want)
	}

	h.SendMsg(coverA) // and again after it
	if m.coverArtFetched || m.coverArt != nil {
		t.Errorf("kept release A's cover after switching to B: fetched %v, cover %q", m.coverArtFetched, m.coverArt)
	}
	h.SendEnter() // path preview
	h.SendEnter()
	if m.state != StatePathPreview {
		t.Fatalf("import started before B's cover arrived: state %d", m.state)
	}

	h.SendMsg(coverB)
	if got := importedCover(m); got != "cover B" {
		t.Errorf("import carries cover %q, want %q", got, "cover B")
	}
	if id := m.importParams("/music/library1", 0, "/src/01.flac").Release.ID; id != "B" {
		t.Errorf("import carries release %q, want B", id)
	}
	h.SendEnter()
	if m.state != StateImporting {
		t.Errorf("B's cover did not unblock the import: state %d", m.state)
	}
}

// A refresh that keeps the release (tags naming it, or naming none) must not
// fetch its cover a second time.
func TestImport_SameReleaseFetchesCoverOnce(t *testing.T) {
	for _, tagged := range []string{"abc123", ""} {
		t.Run("tagged "+tagged, func(t *testing.T) {
			fake := &fakeMB{
				releases: map[string]*musicbrainz.ReleaseDetails{"abc123": releaseWithTracks("abc123")},
				covers:   map[string][]byte{"abc123": []byte("cover A")},
			}
			h, m, coverA := openImport(t, fake)

			if cmd := readTags(t, h, tagged); cmd != nil {
				t.Errorf("refreshing the same release emitted %#v", run(cmd))
			}
			h.SendMsg(coverA)

			if want := []string{"abc123"}; !slices.Equal(fake.fetched, want) {
				t.Errorf("covers fetched for %v, want %v", fake.fetched, want)
			}
			if got := importedCover(m); got != "cover A" {
				t.Errorf("import carries cover %q, want %q", got, "cover A")
			}
		})
	}
}

// A failed switch keeps the original release, and so its cover.
func TestImport_FailedSwitchKeepsOriginalCover(t *testing.T) {
	fake := &fakeMB{covers: map[string][]byte{"abc123": []byte("cover A")}}
	h, m, coverA := openImport(t, fake)

	if cmd := readTags(t, h, "B"); cmd != nil {
		t.Errorf("a failed refresh emitted %#v", run(cmd))
	}
	h.SendMsg(coverA)

	if id := m.releaseID(); id != "abc123" {
		t.Errorf("release = %q, want abc123", id)
	}
	if got := importedCover(m); got != "cover A" {
		t.Errorf("import carries cover %q, want %q", got, "cover A")
	}
}
