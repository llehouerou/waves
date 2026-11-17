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
// of 2018. The artist "artist" has that group only. Any other request panics,
// on the nil client it embeds.
type fakeMB struct {
	mbworkflow.Client
	covers    map[string][]byte                // the cover of each release; none for the others
	coverErrs map[string]error                 // the releases whose cover can't be fetched
	noGroup   bool                             // the releases come with no release group
	searches  map[string][]core.MBReleaseGroup // the release groups each query finds
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

// searchedTrust is the release group of "trust" as the search endpoint of
// MusicBrainz returns it: with no genres.
func searchedTrust() core.MBReleaseGroup {
	group := *trustGroup()
	group.Genres = nil
	return group
}

func (f fakeMB) SearchReleaseGroups(query string) ([]core.MBReleaseGroup, error) {
	groups, ok := f.searches[query]
	if !ok {
		return nil, errors.New("unavailable")
	}
	return groups, nil
}

func (f fakeMB) GetArtistReleaseGroups(id string) ([]core.MBReleaseGroup, error) {
	if id != "artist" {
		return nil, errors.New("unavailable")
	}
	return []core.MBReleaseGroup{*trustGroup()}, nil
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

func (f fakeMB) GetCoverArt(releaseID string) ([]byte, error) {
	if err := f.coverErrs[releaseID]; err != nil {
		return nil, err
	}
	return f.covers[releaseID], nil
}

// trustCovers are the covers of the releases of "trust": each its own.
func trustCovers() map[string][]byte {
	return map[string][]byte{"release": []byte("cover"), "other": []byte("cover of other")}
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
	return openPopupOn(lib, fakeMB{covers: trustCovers()})
}

// openPopupOn is openPopup on mb.
func openPopupOn(lib *fakeLibrary, mb fakeMB) (*testutil.PopupHarness, *Model) {
	return openAlbumPopup("trust", lib, mb)
}

// openAlbumPopup is openPopupOn for the album of "low" named album.
func openAlbumPopup(album string, lib *fakeLibrary, mb fakeMB) (*testutil.PopupHarness, *Model) {
	m := New("low", album, mb, lib)
	m.SetSize(100, 40)
	return testutil.NewPopupHarness(m), m
}

// previewingPopup is openPopup taken to the tag preview: the tags are read,
// the release they name is loaded and its cover fetched.
func previewingPopup(t *testing.T) (*testutil.PopupHarness, *Model, *fakeLibrary) {
	t.Helper()
	lib := &fakeLibrary{tags: albumTags()}
	h, m := previewingPopupOn(t, lib)
	return h, m, lib
}

// previewingPopupOn is previewingPopup on the tags lib holds.
func previewingPopupOn(t *testing.T, lib *fakeLibrary) (*testutil.PopupHarness, *Model) {
	t.Helper()
	h, m := openPopup(lib)
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the release they name
	_, cmd = h.ExecuteAndSend(cmd)              // the release, then its cover
	h.ExecuteAndSend(cmd)
	if m.state != StateTagPreview || !m.coverArtFetched {
		t.Fatalf("state = %d, cover fetched %v; want the tag preview with its cover", m.state, m.coverArtFetched)
	}
	h.ClearCommands()
	return h, m
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

// === The cover ===

// previewingReleaseOn is openPopupOn taken to the tag preview of "release",
// the release the tags name: it returns the fetch of its cover, its result
// not delivered.
func previewingReleaseOn(t *testing.T, mb fakeMB) (*testutil.PopupHarness, *Model, *fakeLibrary, tea.Cmd) {
	t.Helper()
	lib := &fakeLibrary{tags: albumTags()}
	h, m := openPopupOn(lib, mb)
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the release they name
	_, cover := h.ExecuteAndSend(cmd)           // the release, then its cover
	if m.state != StateTagPreview || cover == nil {
		t.Fatalf("state = %d, cover fetch %v; want the tag preview fetching its cover", m.state, cover != nil)
	}
	return h, m, lib, cover
}

// listReleases goes back from the tag preview to the releases of the group.
func listReleases(t *testing.T, h *testutil.PopupHarness, m *Model) {
	t.Helper()
	h.ExecuteAndSend(h.SendSpecialKey(tea.KeyBackspace)) // the releases of the group
	if m.state != StateReleaseResults {
		t.Fatalf("state = %d, want the releases of the group", m.state)
	}
}

// pickOther picks "other" in the releases of the group: it returns the fetch
// of its cover, its result not delivered.
func pickOther(t *testing.T, h *testutil.PopupHarness, m *Model) tea.Cmd {
	t.Helper()
	h.SendDown()
	_, cover := h.ExecuteAndSend(h.SendEnter()) // the release, then its cover
	if m.state != StateTagPreview || m.releaseDetails.ID != "other" || cover == nil {
		t.Fatalf("state = %d, want the tag preview of other fetching its cover", m.state)
	}
	return cover
}

// assertWaitsForTheCover checks that the tag preview waits for its cover:
// Enter asks the core nothing.
func assertWaitsForTheCover(t *testing.T, h *testutil.PopupHarness, lib *fakeLibrary) {
	t.Helper()
	assertView(t, h, "Cover Art:    Fetching...", "Waiting for cover art...")
	h.SendEnter()
	if len(lib.retagged) != 0 {
		t.Fatalf("Retag asked for %+v before the cover of the release shown came", lib.retagged)
	}
}

// assertRetaggedAsOtherWith checks that the core was asked once to retag the
// album as "other", with cover.
func assertRetaggedAsOtherWith(t *testing.T, lib *fakeLibrary, cover []byte) {
	t.Helper()
	if len(lib.retagged) != 1 {
		t.Fatalf("Retag asked %d times, want once", len(lib.retagged))
	}
	req := lib.retagged[0]
	if req.Release == nil || req.Release.ID != "other" || !slices.Equal(req.Cover, cover) {
		t.Errorf("Retag asked for release %+v with cover %q, want other with %q", req.Release, req.Cover, cover)
	}
}

// Back from the release the tags name, with its cover, another release is
// picked: it waits for its own cover, and is tagged with it.
func TestRetag_AReleasePickedBackFromThePreviewWaitsForItsOwnCover(t *testing.T) {
	h, m, lib, cover := previewingReleaseOn(t, fakeMB{covers: trustCovers()})
	h.ExecuteAndSend(cover)
	listReleases(t, h, m)
	cover = pickOther(t, h, m)

	assertWaitsForTheCover(t, h, lib)

	h.ExecuteAndSend(cover)
	assertView(t, h, "Found (14 B, will be embedded)", "[Enter] Retag")
	h.SendEnter()
	assertRetaggedAsOtherWith(t, lib, []byte("cover of other"))
}

// The cover of the release picked back can't be fetched: it is retagged with
// no cover, not with the one of the release shown before.
func TestRetag_AReleasePickedBackWhoseCoverCannotBeFetchedIsRetaggedWithNoCover(t *testing.T) {
	mb := fakeMB{covers: trustCovers(), coverErrs: map[string]error{"other": errors.New("unavailable")}}
	h, m, lib, cover := previewingReleaseOn(t, mb)
	h.ExecuteAndSend(cover)
	listReleases(t, h, m)
	h.ExecuteAndSend(pickOther(t, h, m))

	assertView(t, h, "Cover Art:    Not available", "[Enter] Retag")
	h.SendEnter()
	assertRetaggedAsOtherWith(t, lib, nil)
}

// The release picked back has no cover: it is retagged with none.
func TestRetag_AReleasePickedBackWithNoCoverIsRetaggedWithNoCover(t *testing.T) {
	h, m, lib, cover := previewingReleaseOn(t, fakeMB{covers: map[string][]byte{"release": []byte("cover")}})
	h.ExecuteAndSend(cover)
	listReleases(t, h, m)
	h.ExecuteAndSend(pickOther(t, h, m))

	assertView(t, h, "Cover Art:    Not available", "[Enter] Retag")
	h.SendEnter()
	assertRetaggedAsOtherWith(t, lib, nil)
}

// Back from the release the tags name before its cover came, another release
// is picked: the cover of the first one, come late, is not its cover.
func TestRetag_TheLateCoverOfTheReleaseShownBeforeIsNotTheCoverOfTheOnePicked(t *testing.T) {
	h, m, lib, releaseCover := previewingReleaseOn(t, fakeMB{covers: trustCovers()})
	listReleases(t, h, m)
	otherCover := pickOther(t, h, m)

	h.ExecuteAndSend(releaseCover)
	assertWaitsForTheCover(t, h, lib)

	h.ExecuteAndSend(otherCover)
	h.SendEnter()
	assertRetaggedAsOtherWith(t, lib, []byte("cover of other"))
}

// The cover of the release shown before comes while the releases are listed:
// the release picked next waits for its own.
func TestRetag_ACoverThatComesWhileTheReleasesAreListedIsNoReleasesCover(t *testing.T) {
	h, m, lib, releaseCover := previewingReleaseOn(t, fakeMB{covers: trustCovers()})
	listReleases(t, h, m)
	h.ExecuteAndSend(releaseCover)
	otherCover := pickOther(t, h, m)

	assertWaitsForTheCover(t, h, lib)

	h.ExecuteAndSend(otherCover)
	h.SendEnter()
	assertRetaggedAsOtherWith(t, lib, []byte("cover of other"))
}

// The release the tags name, picked back from its own preview, is retagged
// with its own cover.
func TestRetag_TheSameReleasePickedBackIsRetaggedWithItsCover(t *testing.T) {
	h, m, lib, cover := previewingReleaseOn(t, fakeMB{covers: trustCovers()})
	h.ExecuteAndSend(cover)
	listReleases(t, h, m)
	pickFirstRelease(t, h, m)

	h.SendEnter()
	if len(lib.retagged) != 1 || string(lib.retagged[0].Cover) != "cover" {
		t.Errorf("Retag asked for %+v, want release once, with its cover", lib.retagged)
	}
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
		"Release Type":  "album; live",
		"Genre":         "Slowcore",
		"MB Group ID":   "rg",
	} {
		if line := previewed(t, h, field); !strings.HasSuffix(line, "  "+want) {
			t.Errorf("tag preview: %q, want %s to become %q", line, field, want)
		}
	}
}

// The preview compares the release type of the files with the one written,
// exactly: files already tagged "album; live" have nothing to change, files
// waves tagged "album; Live" are rewritten.
func TestRetag_TheReleaseTypeIsComparedWithTheLowerCaseOneWritten(t *testing.T) {
	releaseTypeChanged := func(tagged string) bool {
		fileTags := albumTags()
		for i := range fileTags {
			fileTags[i].ReleaseType = tagged
		}
		_, m := previewingPopupOn(t, &fakeLibrary{tags: fileTags})
		for _, d := range m.tagDiffs {
			if d.Field == "Release Type" {
				return d.Changed
			}
		}
		t.Fatal("no Release Type line in the preview")
		return false
	}

	if releaseTypeChanged("album; live") {
		t.Error("files tagged album; live reported changed")
	}
	if !releaseTypeChanged("album; Live") {
		t.Error("files tagged album; Live not reported changed")
	}
}

// assertRetaggedAsTrust checks that the core was asked once to retag the
// album as release, with the release group of "trust" as MusicBrainz gave it
// with that release.
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
	if req.Release != nil && req.ReleaseGroup != req.Release.ReleaseGroup {
		t.Error("retagged with a release group that is not the one of the release")
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
	pickFirstRelease(t, h, m)

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

// untaggedTags are albumTags with no MusicBrainz ID, and a genre of their own.
func untaggedTags() []core.FileTags {
	fileTags := albumTags()
	for i := range fileTags {
		fileTags[i].MBReleaseID, fileTags[i].Genre = "", "Old Genre"
	}
	return fileTags
}

// pickFirstRelease picks the first release listed and takes the popup to its
// tag preview.
func pickFirstRelease(t *testing.T, h *testutil.PopupHarness, m *Model) {
	t.Helper()
	if m.state != StateReleaseResults {
		t.Fatalf("state = %d, want the releases of the group", m.state)
	}
	_, cmd := h.ExecuteAndSend(h.SendEnter()) // the release, then its cover
	h.ExecuteAndSend(cmd)
	if m.state != StateTagPreview {
		t.Fatalf("state = %d, want the tag preview", m.state)
	}
}

// The tags carry no MusicBrainz ID: the popup searches the album and picks the
// only group that matches it. The search endpoint gives it no genres, the
// release picked carries it with its genres: the album gets them.
func TestRetag_AReleaseOfTheGroupFoundBySearchIsTaggedWithItsReleaseGroup(t *testing.T) {
	lib := &fakeLibrary{tags: untaggedTags()}
	h, m := openPopupOn(lib, fakeMB{searches: map[string][]core.MBReleaseGroup{"low trust": {searchedTrust()}}})
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the search
	_, cmd = h.ExecuteAndSend(cmd)              // the group found, then its releases
	h.ExecuteAndSend(cmd)

	assertView(t, h, "Release Group: Trust")
	pickFirstRelease(t, h, m)

	assertPreviewsTrust(t, h)
	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "release")
}

// The group picked from a search typed in the popup lists its releases only:
// the album is tagged with the group of the release picked.
func TestRetag_AReleaseOfTheGroupPickedFromATypedSearchIsTaggedWithItsReleaseGroup(t *testing.T) {
	lib := &fakeLibrary{tags: untaggedTags()}
	mb := fakeMB{searches: map[string][]core.MBReleaseGroup{
		"low trust (2002)": nil,
		"low trust":        {searchedTrust()},
	}}
	h, m := openAlbumPopup("trust (2002)", lib, mb)
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the search
	h.ExecuteAndSend(cmd)
	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the release groups found", m.state)
	}

	h.SendKey("/")
	h.SendSpecialKey(tea.KeyCtrlU)
	h.SendKey("low trust")
	_, cmd = h.ExecuteAndSend(h.SendEnter()) // the search, then nothing: the group is not the album's name
	if cmd != nil || m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the release groups found", m.state)
	}
	h.ExecuteAndSend(h.SendEnter()) // the group picked, then its releases
	assertView(t, h, "Release Group: Trust")
	pickFirstRelease(t, h, m)

	assertPreviewsTrust(t, h)
	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "release")
}

// The search index lags behind the database: where the group found by a
// search and the group of the release picked differ, the release's wins.
func TestRetag_TheReleaseGroupOfTheReleaseWinsOverTheGroupFoundBySearch(t *testing.T) {
	stale := *trustGroup()
	stale.FirstRelease = "2003-01-01"
	lib := &fakeLibrary{tags: untaggedTags()}
	h, m := openPopupOn(lib, fakeMB{searches: map[string][]core.MBReleaseGroup{"low trust": {stale}}})
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the search
	_, cmd = h.ExecuteAndSend(cmd)              // the group found, then its releases
	h.ExecuteAndSend(cmd)
	pickFirstRelease(t, h, m)

	if line := previewed(t, h, "Original Date"); !strings.HasSuffix(line, "  2002-09-24") {
		t.Errorf("tag preview: %q, want Original Date to become 2002-09-24", line)
	}
	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "release")
}

// The tags name an artist only: the album is tagged with the group of the
// release picked among those of the artist's group.
func TestRetag_AReleaseOfTheArtistsGroupIsTaggedWithItsReleaseGroup(t *testing.T) {
	fileTags := untaggedTags()
	for i := range fileTags {
		fileTags[i].MBArtistID = "artist"
	}
	lib := &fakeLibrary{tags: fileTags}
	h, m := openPopupOn(lib, fakeMB{})
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the artist's groups
	_, cmd = h.ExecuteAndSend(cmd)              // the group of the album, then its releases
	h.ExecuteAndSend(cmd)

	assertView(t, h, "Release Group: Trust")
	pickFirstRelease(t, h, m)

	assertPreviewsTrust(t, h)
	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "release")
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

// The release group list renders with its search input open and emptied at
// every width without panicking: it draws what fits (issue #201).
func TestRetag_ReleaseGroupSearchRendersAtAnyWidthWithoutPanicking(t *testing.T) {
	lib := &fakeLibrary{tags: untaggedTags()}
	h, m := openAlbumPopup("trust (2002)", lib, fakeMB{searches: map[string][]core.MBReleaseGroup{"low trust (2002)": nil}})
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the search
	h.ExecuteAndSend(cmd)
	if m.state != StateReleaseGroupResults {
		t.Fatalf("state = %d, want the release groups found", m.state)
	}
	h.SendKey("/")
	h.SendSpecialKey(tea.KeyCtrlU)
	if m.searchInput.Value() != "" || !m.searchInput.Focused() {
		t.Fatalf("search input %q, focused %v; want it open and empty", m.searchInput.Value(), m.searchInput.Focused())
	}

	for width := range 16 {
		m.SetSize(width, 20)
		if r := testutil.PanicOf(func() { m.View() }); r != nil {
			t.Errorf("width %d panics: %v", width, r)
		}
	}
}

// === MusicBrainz results the popup did not ask for ===

// foreignResults are MusicBrainz results of each kind the retag popup takes,
// one that succeeds and one that fails, for requests about "trust": a popup
// that did not ask for them, or took them already, drops them.
func foreignResults() map[string]tea.Msg {
	other, _ := fakeMB{}.GetRelease("other")
	unavailable := errors.New("unavailable")
	groups := mbworkflow.Request{Kind: mbworkflow.SearchReleaseGroups, Of: "low trust"}
	releases := mbworkflow.Request{Kind: mbworkflow.ReleaseGroupReleases, Of: "rg"}
	details := mbworkflow.Request{Kind: mbworkflow.ReleaseDetails, Of: "other"}
	return map[string]tea.Msg{
		"release groups":         mbworkflow.SearchResultMsg{Request: groups, ReleaseGroups: []core.MBReleaseGroup{searchedTrust()}},
		"failed release groups":  mbworkflow.SearchResultMsg{Request: groups, Err: unavailable},
		"releases":               mbworkflow.ReleasesResultMsg{Request: releases, Releases: []core.MBRelease{{ID: "other", TrackCount: 3}}},
		"failed releases":        mbworkflow.ReleasesResultMsg{Request: releases, Err: unavailable},
		"release details":        mbworkflow.ReleaseDetailsResultMsg{Request: details, Details: other},
		"failed release details": mbworkflow.ReleaseDetailsResultMsg{Request: details, Err: unavailable},
	}
}

// assertDrops checks that the popup drops msg: it starts nothing, and shows
// what it showed.
func assertDrops(t *testing.T, h *testutil.PopupHarness, m *Model, msg tea.Msg) {
	t.Helper()
	state, view := m.state, testutil.StripANSI(h.View())
	if cmd := h.SendMsg(msg); cmd != nil {
		t.Errorf("the popup started %#v", cmd())
	}
	if m.state != state {
		t.Errorf("state = %d, want it left at %d", m.state, state)
	}
	if got := testutil.StripANSI(h.View()); got != view {
		t.Errorf("the view changed:\n%s\nwant:\n%s", got, view)
	}
}

// groupListingPopup is a popup of an album with no MusicBrainz ID in its
// tags, on the release groups its search found: it waits for nothing.
func groupListingPopup(t *testing.T) (*testutil.PopupHarness, *Model, *fakeLibrary) {
	t.Helper()
	lib := &fakeLibrary{tags: untaggedTags()}
	mb := fakeMB{searches: map[string][]core.MBReleaseGroup{"low trust (2002)": {searchedTrust()}}}
	h, m := openAlbumPopup("trust (2002)", lib, mb)
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the search
	h.ExecuteAndSend(cmd)
	if m.state != StateReleaseGroupResults || len(m.releaseGroups) != 1 {
		t.Fatalf("state = %d, want the release group found", m.state)
	}
	h.ClearCommands()
	return h, m, lib
}

func TestRetag_APopupWaitingForNoResultDropsEveryResult(t *testing.T) {
	popups := map[string]func(t *testing.T) (*testutil.PopupHarness, *Model){
		"release groups": func(t *testing.T) (*testutil.PopupHarness, *Model) {
			t.Helper()
			h, m, _ := groupListingPopup(t)
			return h, m
		},
		"releases": func(t *testing.T) (*testutil.PopupHarness, *Model) {
			t.Helper()
			h, m, _ := previewingPopup(t)
			listReleases(t, h, m)
			return h, m
		},
		"tag preview": func(t *testing.T) (*testutil.PopupHarness, *Model) {
			t.Helper()
			h, m, _ := previewingPopup(t)
			return h, m
		},
		"retagging": retaggingPopup,
	}
	for name, open := range popups {
		for result, msg := range foreignResults() {
			t.Run(name+"/"+result, func(t *testing.T) {
				h, m := open(t)
				assertDrops(t, h, m, msg)
			})
		}
	}
}

// The tag preview of the release the tags name drops the details of another
// release: Enter retags the album as the release shown.
func TestRetag_TheTagPreviewRetagsAsItsReleaseAfterTheDetailsOfAnother(t *testing.T) {
	h, _, lib := previewingPopup(t)

	h.SendMsg(foreignResults()["release details"])

	h.SendEnter()
	assertRetaggedAsTrust(t, lib, "release")
}

// pickOtherPending picks "other" in the releases of the group: it returns
// the fetch of its details, its result not delivered.
func pickOtherPending(t *testing.T, h *testutil.PopupHarness, m *Model) tea.Cmd {
	t.Helper()
	h.SendDown()
	cmd := h.SendEnter()
	if m.state != StateReleaseDetailsLoading || cmd == nil {
		t.Fatalf("state = %d, want the details of other loading", m.state)
	}
	return cmd
}

func TestRetag_WaitingForTheDetailsOfOneReleaseDropsThoseOfAnother(t *testing.T) {
	h, m, lib := previewingPopup(t)
	listReleases(t, h, m)
	own := pickOtherPending(t, h, m)

	release, _ := fakeMB{}.GetRelease("release")
	stale := mbworkflow.Request{Kind: mbworkflow.ReleaseDetails, Of: "release"}
	assertDrops(t, h, m, mbworkflow.ReleaseDetailsResultMsg{Request: stale, Details: release})
	assertDrops(t, h, m, mbworkflow.ReleaseDetailsResultMsg{Request: stale, Err: errors.New("unavailable")})

	_, cover := h.ExecuteAndSend(own) // its details, then its cover
	h.ExecuteAndSend(cover)
	h.SendEnter()
	assertRetaggedAsOtherWith(t, lib, []byte("cover of other"))
}

func TestRetag_WaitingForTheReleasesOfOneGroupDropsEveryOtherResult(t *testing.T) {
	h, m, _ := previewingPopup(t)
	own := h.SendSpecialKey(tea.KeyBackspace) // the releases of the group
	if m.state != StateReleaseLoading || own == nil {
		t.Fatalf("state = %d, want the releases of the group loading", m.state)
	}

	unavailable := errors.New("unavailable")
	another := mbworkflow.Request{Kind: mbworkflow.ReleaseGroupReleases, Of: "another group"}
	assertDrops(t, h, m, mbworkflow.ReleasesResultMsg{Request: another, Releases: []core.MBRelease{{ID: "elsewhere", TrackCount: 3}}})
	assertDrops(t, h, m, mbworkflow.ReleasesResultMsg{Request: another, Err: unavailable})
	assertDrops(t, h, m, foreignResults()["release details"])
	assertDrops(t, h, m, foreignResults()["failed release details"])
	assertDrops(t, h, m, foreignResults()["release groups"])
	assertDrops(t, h, m, foreignResults()["failed release groups"])

	h.ExecuteAndSend(own)
	if m.state != StateReleaseResults || len(m.releases) != 2 {
		t.Fatalf("state = %d with %d releases, want the two releases of the group", m.state, len(m.releases))
	}
}

// A popup closed while it loaded the details of a release: its result comes
// to the popup of another album, opened since. That one never previews the
// release, and never retags its album as it.
func TestRetag_APopupOpenedAfterAnotherWasClosedNeverPreviewsItsRelease(t *testing.T) {
	closed, closedModel, _ := previewingPopup(t)
	listReleases(t, closed, closedModel)
	late := testutil.ExecuteCmd(pickOtherPending(t, closed, closedModel))
	closed.SendEscape()

	h, m, lib := groupListingPopup(t)
	assertDrops(t, h, m, late)

	h.SendEnter()
	if m.state == StateTagPreview || len(lib.retagged) != 0 {
		t.Errorf("state = %d, retagged %+v; want no preview and nothing retagged", m.state, lib.retagged)
	}
}

// A result delivered twice is taken once: the release previewed fetches its
// cover once.
func TestRetag_AResultDeliveredTwiceIsTakenOnce(t *testing.T) {
	lib := &fakeLibrary{tags: albumTags()}
	h, m := openPopup(lib)
	_, cmd := h.ExecuteAndSend(h.LastCommand()) // the tags, then the release they name
	details, cover := h.ExecuteAndSend(cmd)     // the release, then its cover
	if m.state != StateTagPreview || cover == nil {
		t.Fatalf("state = %d, want the tag preview fetching its cover", m.state)
	}

	assertDrops(t, h, m, details)
}
