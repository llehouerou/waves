package retag

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeLibrary is the core's library as the popup sees it: it hands out the
// tags it holds and records what it is asked.
type fakeLibrary struct {
	tags     []core.FileTags
	tagsErr  error
	retagErr error
	read     [][2]string         // the albums AlbumTags was asked for
	retagged []core.RetagRequest // what Retag was asked
}

func (f *fakeLibrary) AlbumTags(albumArtist, album string) ([]core.FileTags, error) {
	f.read = append(f.read, [2]string{albumArtist, album})
	return f.tags, f.tagsErr
}

func (f *fakeLibrary) Retag(req core.RetagRequest) error {
	if f.retagErr != nil {
		return f.retagErr
	}
	f.retagged = append(f.retagged, req)
	return nil
}

// fakeMB knows the release group "rg" of "trust" by "low", first released in
// 2002, and two releases of it: "release", a reissue of 2011, and "other", one
// of 2018. Any other request panics, on the nil client it embeds.
type fakeMB struct {
	mbworkflow.Client
	cover   []byte
	noGroup bool // the releases come with no release group
}

// trustGroup is the release group fakeMB knows, as MusicBrainz returns it.
func trustGroup() *core.MBReleaseGroup {
	return &core.MBReleaseGroup{
		ID: "rg", Title: "Trust", Artist: "Low", PrimaryType: "Album", SecondaryTypes: []string{"Live"},
		FirstRelease: "2002-09-24", Genres: []string{"Slowcore"},
	}
}

func (f fakeMB) GetRelease(id string) (*core.MBReleaseDetails, error) {
	dates := map[string]string{"release": "2011-11-08", "other": "2018-05-01"}
	if dates[id] == "" {
		return nil, errors.New("unavailable")
	}
	details := &core.MBReleaseDetails{
		Release:      core.MBRelease{ID: id, Title: "Trust", Artist: "Low", Date: dates[id], ReleaseType: "Album"},
		Tracks:       []core.MBTrack{{Position: 1, Title: "Amazing Grace"}, {Position: 2, Title: "Canada"}, {Position: 3, Title: "Candy Girl"}},
		ReleaseGroup: trustGroup(),
	}
	if f.noGroup {
		details.ReleaseGroup = nil
	}
	return details, nil
}

func (f fakeMB) GetReleaseGroupReleases(id string) ([]core.MBRelease, error) {
	if id != "rg" {
		return nil, errors.New("unavailable")
	}
	return []core.MBRelease{
		{ID: "other", Title: "Trust", Date: "2018-05-01", TrackCount: 3},
		{ID: "release", Title: "Trust", Date: "2011-11-08", TrackCount: 3},
	}, nil
}

func (f fakeMB) GetCoverArt(string) ([]byte, error) {
	return f.cover, nil
}

// albumTags are the tags of the three files of "trust" by "low", which name
// the release fakeMB knows.
func albumTags() []core.FileTags {
	fileTags := make([]core.FileTags, 3)
	for i, title := range []string{"amazing grace", "canada", "candy girl"} {
		fileTags[i] = core.FileTags{
			Path: "/music/low/trust/" + title + ".mp3", Title: title, AlbumArtist: "low", Album: "trust",
			TrackNumber: i + 1, MBReleaseID: "release",
		}
	}
	return fileTags
}

// openPopup opens the retag popup of "trust" by "low" on lib. Its Init is in
// the harness, not run yet.
func openPopup(lib *fakeLibrary) (*testutil.PopupHarness, *Model) {
	return openPopupOn(lib, fakeMB{cover: []byte("cover")})
}

// openPopupOn is openPopup on mb.
func openPopupOn(lib *fakeLibrary, mb fakeMB) (*testutil.PopupHarness, *Model) {
	m := New("low", "trust", mb, lib)
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m), m
}

// previewingPopup is openPopup taken to the tag preview: the tags are read,
// the release they name is loaded and its cover fetched.
func previewingPopup(t *testing.T) (*testutil.PopupHarness, *Model, *fakeLibrary) {
	t.Helper()
	lib := &fakeLibrary{tags: albumTags()}
	h, m := openPopup(lib)
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the release they name
	_, cmd = h.ExecuteAndSend(cmd)              // the release, then its cover
	h.ExecuteAndSend(cmd)
	if m.state != StateTagPreview || !m.coverArtFetched {
		t.Fatalf("state = %d, cover fetched %v; want the tag preview with its cover", m.state, m.coverArtFetched)
	}
	h.ClearCommands()
	return h, m, lib
}

// retaggingPopup is previewingPopup once the user confirmed and the core
// started the retag.
func retaggingPopup(t *testing.T) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m, _ := previewingPopup(t)
	h.SendEnter()
	if m.state != StateRetagging {
		t.Fatalf("retag not started: state %d", m.state)
	}
	h.ClearCommands()
	return h, m
}

// progress is where the retag of albumTags' files is: one state per file.
func progress(states ...core.FileState) []core.FileProgress {
	fileTags := albumTags()
	files := make([]core.FileProgress, len(states))
	for i, state := range states {
		files[i] = core.FileProgress{Path: fileTags[i].Path, State: state}
		if state == core.FileFailed {
			files[i].Err = errors.New("read-only file")
		}
	}
	return files
}

func lastAction(t *testing.T, h *testutil.PopupHarness) action.Action {
	t.Helper()
	msg, ok := testutil.ExecuteCmd(h.LastCommand()).(action.Msg)
	if !ok {
		t.Fatalf("the popup emitted %#v, want an action", msg)
	}
	if msg.Source != "retag" {
		t.Errorf("action source = %q, want retag", msg.Source)
	}
	return msg.Action
}

func assertView(t *testing.T, h *testutil.PopupHarness, contains ...string) {
	t.Helper()
	for _, s := range contains {
		if err := h.AssertViewContains(s); err != "" {
			t.Error(err)
		}
	}
}

// === Reading the album ===

func TestRetag_ReadsTheAlbumsTagsThroughTheCoreAndLoadsTheReleaseTheyName(t *testing.T) {
	lib := &fakeLibrary{tags: albumTags()}
	h, m := openPopup(lib)

	assertView(t, h, "Retag: low - trust", "Reading tags from files...")
	if len(lib.read) != 0 {
		t.Error("the tags were read on the UI goroutine")
	}

	_, cmd := h.ExecuteAndSend(h.LastCommand())
	if want := [][2]string{{"low", "trust"}}; !slices.Equal(lib.read, want) {
		t.Errorf("AlbumTags asked for %v, want %v", lib.read, want)
	}
	_, cmd = h.ExecuteAndSend(cmd)
	h.ExecuteAndSend(cmd)

	if m.state != StateTagPreview {
		t.Fatalf("state = %d, want the tag preview", m.state)
	}
	assertView(t, h, "Release: Trust", "Tag Changes Preview", "Album Artist", "3 files will be retagged",
		"Found (5 B, will be embedded)", "[Enter] Retag")
}

func TestRetag_TagsThatCannotBeReadAreSaid(t *testing.T) {
	lib := &fakeLibrary{tagsErr: errors.New("no track of low - trust in the library")}
	h, _ := openPopup(lib)

	if _, cmd := h.ExecuteAndSend(h.LastCommand()); cmd != nil {
		t.Errorf("a search was started for an album that can't be read: %#v", cmd())
	}

	assertView(t, h, "Error: no track of low - trust in the library")
}

// === Starting the retag ===

func TestRetag_EnterInTheTagPreviewAsksTheCoreToRetagTheAlbum(t *testing.T) {
	h, m, lib := previewingPopup(t)

	h.SendEnter()

	if len(lib.retagged) != 1 {
		t.Fatalf("Retag asked %d times, want once", len(lib.retagged))
	}
	req := lib.retagged[0]
	if req.AlbumArtist != "low" || req.Album != "trust" || req.Release == nil || req.Release.ID != "release" ||
		req.ReleaseGroup == nil || string(req.Cover) != "cover" {
		t.Errorf("Retag asked for %+v, want low - trust as the release shown, with its group and its cover", req)
	}
	if m.state != StateRetagging {
		t.Errorf("state = %d, want retagging", m.state)
	}
	assertView(t, h, "Retagging...", "amazing grace.mp3", "Pending", "Progress: 0/3 files")
}

func TestRetag_ARetagTheCoreRefusesIsSaid(t *testing.T) {
	h, m, lib := previewingPopup(t)
	lib.retagErr = core.ErrRetagRunning

	h.SendEnter()

	if m.state != StateTagPreview {
		t.Errorf("state = %d, want to stay on the tag preview", m.state)
	}
	assertView(t, h, "another retag is running")
}

// === The release group ===

// previewed is the line of the tag preview about field, trimmed.
func previewed(t *testing.T, h *testutil.PopupHarness, field string) string {
	t.Helper()
	for line := range strings.SplitSeq(testutil.StripANSI(h.View()), "\n") {
		if strings.HasPrefix(line, fmt.Sprintf("%-14s  ", field)) {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("no line about %s in the tag preview:\n%s", field, testutil.StripANSI(h.View()))
	return ""
}

// assertPreviewsTrust checks that the tag preview shows what the release
// group of "trust" says.
func assertPreviewsTrust(t *testing.T, h *testutil.PopupHarness) {
	t.Helper()
	for field, want := range map[string]string{
		"Original Date": "2002-09-24",
		"Original Year": "2002",
		"Release Type":  "album; Live",
		"Genre":         "Slowcore",
		"MB Group ID":   "rg",
	} {
		if line := previewed(t, h, field); !strings.HasSuffix(line, "  "+want) {
			t.Errorf("tag preview: %q, want %s to become %q", line, field, want)
		}
	}
}

// assertRetaggedAsTrust checks that the core was asked once to retag the
// album as release, with the release group of "trust" as MusicBrainz gave it.
func assertRetaggedAsTrust(t *testing.T, lib *fakeLibrary, release string) {
	t.Helper()
	if len(lib.retagged) != 1 {
		t.Fatalf("Retag asked %d times, want once", len(lib.retagged))
	}
	req := lib.retagged[0]
	if req.Release == nil || req.Release.ID != release {
		t.Errorf("retagged as release %+v, want %s", req.Release, release)
	}
	if !reflect.DeepEqual(req.ReleaseGroup, trustGroup()) {
		t.Errorf("retagged with release group %+v, want %+v", req.ReleaseGroup, trustGroup())
	}
}

// The tags name a reissue: the album is tagged with the release group of the
// reissue, first released years before it.
func TestRetag_TheReleaseTheTagsNameIsTaggedWithItsReleaseGroup(t *testing.T) {
	h, _, lib := previewingPopup(t)

	assertPreviewsTrust(t, h)
	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "release")
}

// The tags name a release group only: the release picked among its releases
// is tagged with it.
func TestRetag_AReleaseOfTheGroupTheTagsNameIsTaggedWithItsReleaseGroup(t *testing.T) {
	fileTags := albumTags()
	for i := range fileTags {
		fileTags[i].MBReleaseID, fileTags[i].MBReleaseGroupID = "", "rg"
	}
	lib := &fakeLibrary{tags: fileTags}
	h, m := openPopup(lib)
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the releases of the group
	h.ExecuteAndSend(cmd)
	if m.state != StateReleaseResults {
		t.Fatalf("state = %d, want the releases of the group", m.state)
	}

	_, cmd = h.ExecuteAndSend(h.SendEnter()) // the release, then its cover
	h.ExecuteAndSend(cmd)

	assertPreviewsTrust(t, h)
	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "release")
}

// Back from the release the tags name, another release of its group is picked:
// it is tagged with the group too, not with the date of either release.
func TestRetag_AnotherReleaseOfTheGroupPickedBackFromThePreviewIsTaggedWithItsReleaseGroup(t *testing.T) {
	h, m, lib := previewingPopup(t)

	h.ExecuteAndSend(h.SendSpecialKey(tea.KeyBackspace)) // the releases of the group
	if m.state != StateReleaseResults {
		t.Fatalf("state = %d, want the releases of the group", m.state)
	}
	h.SendDown()
	_, cmd := h.ExecuteAndSend(h.SendEnter()) // the release, then its cover
	h.ExecuteAndSend(cmd)

	assertPreviewsTrust(t, h)
	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "other")
}

func TestRetag_AReleaseWithNoReleaseGroupIsNotPreviewed(t *testing.T) {
	lib := &fakeLibrary{tags: albumTags()}
	h, m := openPopupOn(lib, fakeMB{noGroup: true})
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the release they name
	h.ExecuteAndSend(cmd)

	if m.state == StateTagPreview {
		t.Fatal("a release with no release group is previewed")
	}
	assertView(t, h, "Error: MusicBrainz gave the release no release group")
	h.SendEnter()
	if len(lib.retagged) != 0 {
		t.Errorf("Retag asked for %+v, want nothing retagged", lib.retagged)
	}
}

// === Retagging ===

func TestRetag_ShowsWhereTheCoreSaysTheRetagIs(t *testing.T) {
	h, _ := retaggingPopup(t)

	h.SendMsg(core.RetagProgressed{AlbumArtist: "low", Album: "trust", Files: progress(core.FileDone, core.FileWorking, core.FilePending)})

	assertView(t, h, "Done", "Retagging...", "Pending", "Progress: 1/3 files")
}

// A popup closed while its album was retagged leaves the retag running: its
// events must not land in the retag popup of another album.
func TestRetag_IgnoresTheRetagOfAnotherAlbum(t *testing.T) {
	h, m := retaggingPopup(t)

	h.SendMsg(core.RetagProgressed{AlbumArtist: "low", Album: "secret name", Files: progress(core.FileFailed, core.FileFailed, core.FileFailed)})
	h.SendMsg(core.RetagFinished{AlbumArtist: "low", Album: "secret name", Files: progress(core.FileFailed, core.FileFailed, core.FileFailed)})

	if m.state != StateRetagging {
		t.Errorf("state = %d after another album's retag finished, want still retagging", m.state)
	}
	if err := h.AssertViewNotContains("read-only file"); err != "" {
		t.Errorf("shows another album's files: %s", err)
	}
}

// The core finishes the retag without the popup: Esc may close it.
func TestRetag_EscapeWhileRetaggingCloses(t *testing.T) {
	h, _ := retaggingPopup(t)
	assertView(t, h, "[Esc] Close (retag continues in background)")

	h.SendEscape()

	if _, ok := lastAction(t, h).(Close); !ok {
		t.Error("Esc does not close the popup while retagging")
	}
}

// === Complete ===

func TestRetag_AFinishedRetagWithNoFailureClosesAndTellsTheAlbum(t *testing.T) {
	h, _ := retaggingPopup(t)

	h.SendMsg(core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: progress(core.FileDone, core.FileDone, core.FileDone)})

	done, ok := lastAction(t, h).(Complete)
	if !ok || done.AlbumArtist != "low" || done.AlbumName != "trust" {
		t.Errorf("the popup emitted %#v, want Complete for low - trust", done)
	}
}

func TestRetag_AFinishedRetagShowsTheFilesThatFailed(t *testing.T) {
	h, m := retaggingPopup(t)

	h.SendMsg(core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: progress(core.FileDone, core.FileFailed, core.FileDone)})

	if m.state != StateComplete || h.LastCommand() != nil {
		t.Fatalf("state = %d, want the complete screen, left open", m.state)
	}
	assertView(t, h, "Retag Completed with Errors", "2 files retagged successfully", "1 files failed",
		"canada.mp3: read-only file", "[Enter] Close")
	h.SendEnter()
	if _, ok := lastAction(t, h).(Complete); !ok {
		t.Error("Enter does not complete the popup")
	}
}

func TestRetag_AFinishedRetagSaysTheLibraryWasNotUpdated(t *testing.T) {
	h, m := retaggingPopup(t)

	h.SendMsg(core.RetagFinished{
		AlbumArtist: "low", Album: "trust",
		Files: progress(core.FileDone, core.FileDone, core.FileDone),
		Err:   errors.New("database is locked"),
	})

	if m.state != StateComplete {
		t.Fatalf("state = %d, want the complete screen", m.state)
	}
	assertView(t, h, "Library update failed: database is locked")
}

// === Edge cases ===

func TestRetag_EmptyViewWhenZeroSize(t *testing.T) {
	m := New("low", "trust", fakeMB{}, &fakeLibrary{})

	if view := m.View(); view != "" {
		t.Errorf("View() with zero size = %q, want it empty", view)
	}
}

func TestRetag_SetSizeLeavesAMarginToTheSearchInput(t *testing.T) {
	m := New("low", "trust", fakeMB{}, &fakeLibrary{})

	m.SetSize(100, 50)

	if m.Width() != 100 || m.Height() != 50 || m.searchInput.Width != 96 {
		t.Errorf("size = %dx%d, search input %d; want 100x50 and 96", m.Width(), m.Height(), m.searchInput.Width)
	}
}
