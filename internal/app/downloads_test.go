package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/slskd/slskdtest"
	"github.com/llehouerou/waves/internal/ui/download"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/retag"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// newDownloadsTestModel builds a model whose core downloads from a fake
// slskd.
func newDownloadsTestModel(t *testing.T) (Model, *slskdtest.Server) {
	t.Helper()
	fake := slskdtest.New(t)
	m := newDatabaseTestModel(t, nil, config.SlskdConfig{URL: fake.URL, APIKey: "key", CompletedPath: t.TempDir()})
	return m, fake
}

// bobsAlbum is a download of one file of bob's "Album" folder.
func bobsAlbum() core.Download {
	return core.Download{
		MBReleaseGroupID: "rg", MBArtistName: "Low", MBAlbumTitle: "Trust",
		SlskdUsername: "bob", SlskdDirectory: `@@bob\Music\Album`,
		Files: []core.DownloadFile{{Filename: `@@bob\Music\Album\01.flac`, Size: 3}},
	}
}

// The core syncs the downloads and says when they change: the TUI has no
// polling loop of its own any more.
func TestDownloads_TheCoreSyncsThemAndTheViewFollows(t *testing.T) {
	m, fake := newDownloadsTestModel(t)

	// What the download popup does when the user picks a result.
	if _, err := m.Core.Downloads().Queue(bobsAlbum()); err != nil {
		t.Fatalf("queue: %v", err)
	}
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)
	if got := m.DownloadsView.Downloads(); len(got) != 1 || got[0].Status != core.DownloadPending {
		t.Fatalf("downloads view = %+v, want the download just queued, pending", got)
	}

	// slskd starts the transfer; "r" in the downloads view asks for a Sync
	// rather than wait for the core's clock, which ticks every three seconds.
	fake.SetTransfers(slskd.DownloadFile{
		ID: "t1", Username: "bob", Filename: `@@bob\Music\Album\01.flac`, State: "InProgress", BytesTransferred: 1,
	})
	asked := time.Now()
	m, _ = updateModel(t, m, dlview.ActionMsg(dlview.RefreshRequest{}))
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)

	if got := m.DownloadsView.Downloads(); len(got) != 1 || got[0].Status != core.DownloadDownloading {
		t.Errorf("downloads view = %+v, want the download downloading", got)
	}
	if waited := time.Since(asked); waited > 2*time.Second {
		t.Errorf("the view waited %v for the core's clock: the refresh asked for no Sync", waited)
	}
}

func TestDownloadsChanged(t *testing.T) {
	list := []core.Download{{ID: 7, MBAlbumTitle: "Album"}}
	tests := []struct {
		name      string
		msg       DownloadsChangedMsg
		wantList  int
		wantError string
	}{
		{"operation succeeded", DownloadsChangedMsg{Op: errmsg.OpDownloadDelete, Downloads: list}, 1, ""},
		{"operation failed, list read", DownloadsChangedMsg{Op: errmsg.OpDownloadDelete, Downloads: list, Err: errors.New("boom")}, 1, "delete download"},
		{"list unreadable: view kept", DownloadsChangedMsg{Op: errmsg.OpDownloadClear, Err: errors.New("db")}, 2, "clear completed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.DownloadsView = dlview.New()
			m.DownloadsView.SetDownloads([]core.Download{{ID: 1}, {ID: 2}})

			res, _ := m.handleDownloadMsgCategory(tt.msg)
			got, ok := res.(Model)
			if !ok {
				t.Fatalf("model is %T", res)
			}

			if n := len(got.DownloadsView.Downloads()); n != tt.wantList {
				t.Errorf("view shows %d downloads, want %d", n, tt.wantList)
			}
			if msg := got.Popups.ErrorMsg(); (tt.wantError == "") != (msg == "") || !strings.Contains(msg, tt.wantError) {
				t.Errorf("error = %q, want one mentioning %q", msg, tt.wantError)
			}
		})
	}
}

// The core reports an outage of slskd once; the TUI keeps quiet about it, as
// it did about each failed poll, and goes on watching the core.
func TestSyncFailure_IsNotShown(t *testing.T) {
	m := newIntegrationTestModel(t)

	res, cmd := m.Update(ServiceFailedMsg{Op: errmsg.OpDownloadRefresh, Err: errors.New("connection refused")})

	got, ok := res.(Model)
	if !ok {
		t.Fatalf("model is %T", res)
	}
	if msg := got.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
	if cmd == nil {
		t.Error("the model stopped watching the core")
	}
}

// The events of a slskd search are for the download popup: with none open
// they are dropped, and the model goes on watching the core.
func TestSearchEvents_WithNoDownloadPopupAreDropped(t *testing.T) {
	m := newIntegrationTestModel(t)

	for _, event := range []core.Event{
		core.SearchProgressed{Query: "low trust", Phase: core.SearchSearching},
		core.SearchFinished{Query: "low trust", Err: errors.New("boom")},
	} {
		res, cmd := m.Update(ServiceSearchMsg{Event: event})
		got, ok := res.(Model)
		if !ok {
			t.Fatalf("model is %T", res)
		}
		if msg := got.Popups.ErrorMsg(); msg != "" {
			t.Errorf("%T: error popup = %q, want none", event, msg)
		}
		if cmd == nil {
			t.Errorf("%T: the model stopped watching the core", event)
		}
	}
}

// lowMB is MusicBrainz as a download popup built by a test sees it: the
// artist Low, his album Trust, and its one release.
type lowMB struct{ mbworkflow.Client }

func (lowMB) SearchArtists(string) ([]core.MBArtist, error) {
	return []core.MBArtist{{ID: "low", Name: "Low"}}, nil
}

func (lowMB) GetArtistReleaseGroups(string) ([]core.MBReleaseGroup, error) {
	return []core.MBReleaseGroup{{ID: "rg", Title: "Trust", PrimaryType: "Album"}}, nil
}

func (lowMB) GetReleaseGroupReleases(string) ([]core.MBRelease, error) {
	return []core.MBRelease{{ID: "rel", Title: "Trust", TrackCount: 1}}, nil
}

func (lowMB) GetRelease(string) (*core.MBReleaseDetails, error) {
	return &core.MBReleaseDetails{
		Release:      core.MBRelease{ID: "rel", Title: "Trust", Artist: "Low"},
		ReleaseGroup: &core.MBReleaseGroup{ID: "rg", Title: "Trust", PrimaryType: "Album"},
	}, nil
}

// queueingDownloadPopup is a download popup on the core's downloads, not
// shown, whose user went from the artist search to bob's folder of Low's
// Trust and pressed Enter: it returns the queueing, its outcome not
// delivered.
func queueingDownloadPopup(t *testing.T, m Model) (*download.Model, tea.Cmd) {
	t.Helper()
	dl := download.New(m.Core.Downloads(), lowMB{}, download.FilterConfig{}, m.Core.Library())
	dl.SetSize(100, 40)
	send := func(msg tea.Msg) tea.Cmd {
		_, cmd := dl.Update(msg)
		return cmd
	}
	for _, key := range []string{"l", "o", "w"} {
		send(keyMsg(key))
	}
	for range 4 { // the artists, the release groups, the releases, the release details
		send(send(keyMsg(keyEnter))())
	}
	if dl.State() != download.StateSlskdSearching {
		t.Fatalf("state = %d, want the slskd search", dl.State())
	}
	send(core.SearchFinished{Query: "Low Trust", Responses: []core.SlskdResponse{{
		Username: "bob", HasFreeSlot: true,
		Files: []core.SlskdFile{{Filename: `@@bob\Music\Album\01.flac`, Size: 3}},
	}}})
	cmd := send(keyMsg(keyEnter))
	if dl.State() != download.StateDownloading || cmd == nil {
		t.Fatalf("state = %d, want the download queueing", dl.State())
	}
	return dl, cmd
}

// queueFailure is what the user is told of the refused queueing of Low's
// Trust.
func queueFailure(t *testing.T, msg tea.Msg) string {
	t.Helper()
	queued, ok := msg.(download.SlskdDownloadQueuedMsg)
	if !ok || queued.Err == nil {
		t.Fatalf("outcome = %#v, want a refusal", msg)
	}
	return errmsg.FormatWith(errmsg.OpDownloadQueue, "Low - Trust", queued.Err)
}

// assertToldOnce checks that the model shows want in the error popup, and
// starts nothing that could tell it again.
func assertToldOnce(t *testing.T, m Model, cmd tea.Cmd, want string) {
	t.Helper()
	if got := m.Popups.ErrorMsg(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if cmd != nil {
		t.Errorf("the model started %#v", cmd())
	}
}

// #299: a refused queueing whose popup was closed is told all the same.
func TestDownloadQueueRefused_WithNoPopupOpenIsTold(t *testing.T) {
	m, fake := newDownloadsTestModel(t)
	fake.Refuse(true)
	_, queue := queueingDownloadPopup(t, m)
	msg := queue()
	mode := m.Navigation.ViewMode()

	m, cmd := updateModel(t, m, msg)

	assertToldOnce(t, m, cmd, queueFailure(t, msg))
	if m.Navigation.ViewMode() != mode {
		t.Errorf("view mode = %v, want it left at %v", m.Navigation.ViewMode(), mode)
	}
}

// #298, #299: the outcome of the queueing of a popup closed since comes to a
// download popup opened after it: the popup is neither reset, nor closed, nor
// moved, and the view stays where it is. A refusal is told.
func TestDownloadQueued_APopupOpenedSinceIsLeftAsItIs(t *testing.T) {
	for name, refuse := range map[string]bool{"queued": false, "refused": true} {
		t.Run(name, func(t *testing.T) {
			m, fake := newDownloadsTestModel(t)
			fake.Refuse(refuse)
			_, queue := queueingDownloadPopup(t, m)
			msg := queue()
			m.Popups.ShowDownload(m.Core.Downloads(), m.Core.MusicBrainz(), download.FilterConfig{}, m.Core.Library())
			m, _ = updateModel(t, m, keyMsg("n"))
			before := testutil.StripANSI(m.Popups.Download().View())
			mode := m.Navigation.ViewMode()

			m, cmd := updateModel(t, m, msg)

			want := ""
			if refuse {
				want = queueFailure(t, msg)
			}
			assertToldOnce(t, m, cmd, want)
			dl := m.Popups.Download()
			if dl == nil || !m.Popups.IsVisible(popupctl.Download) {
				t.Fatal("the download popup was closed")
			}
			if got := testutil.StripANSI(dl.View()); got != before || dl.State() != download.StateSearch {
				t.Errorf("the popup changed:\n%s\nwant:\n%s", got, before)
			}
			if m.Navigation.ViewMode() != mode {
				t.Errorf("view mode = %v, want it left at %v", m.Navigation.ViewMode(), mode)
			}
		})
	}
}

// #299: the popup that asked for a refused queueing goes back to its results,
// and the refusal is told once.
func TestDownloadQueueRefused_ItsPopupGoesBackToItsResults(t *testing.T) {
	m, fake := newDownloadsTestModel(t)
	fake.Refuse(true)
	dl, queue := queueingDownloadPopup(t, m)
	m.Popups.Show(popupctl.Download, dl)
	msg := queue()

	m, cmd := updateModel(t, m, msg)

	assertToldOnce(t, m, cmd, queueFailure(t, msg))
	if dl.State() != download.StateSlskdResults {
		t.Errorf("state = %d, want the results", dl.State())
	}
}

// #298: a popup taken back to its results with Backspace while it queued is
// left there by the download queued all the same.
func TestDownloadQueued_APopupTakenBackToItsResultsStaysThere(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	dl, queue := queueingDownloadPopup(t, m)
	m.Popups.Show(popupctl.Download, dl)
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	mode := m.Navigation.ViewMode()

	m, cmd := updateModel(t, m, queue())

	assertToldOnce(t, m, cmd, "")
	if !m.Popups.IsVisible(popupctl.Download) || dl.State() != download.StateSlskdResults {
		t.Errorf("popup visible %v, state %d; want it open on its results", m.Popups.IsVisible(popupctl.Download), dl.State())
	}
	if m.Navigation.ViewMode() != mode {
		t.Errorf("view mode = %v, want it left at %v", m.Navigation.ViewMode(), mode)
	}
}

// The popup that asked for a queueing accepted closes, and the view goes to
// the downloads.
func TestDownloadQueued_ItsPopupClosesOnTheDownloads(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	dl, queue := queueingDownloadPopup(t, m)
	m.Popups.Show(popupctl.Download, dl)

	m, cmd := updateModel(t, m, queue())
	m, _ = updateModel(t, m, cmd())

	if m.Popups.IsVisible(popupctl.Download) || m.Navigation.ViewMode() != navctl.ViewDownloads {
		t.Errorf("popup visible %v, view %v; want the downloads view", m.Popups.IsVisible(popupctl.Download), m.Navigation.ViewMode())
	}
}

// drain runs cmd and every command it batches or sequences, and returns the
// messages they produce. blocked reports a command still running after a
// second: a timer.
func drain(t *testing.T, cmd tea.Cmd) (msgs []tea.Msg, blocked bool) {
	t.Helper()
	if cmd == nil {
		return nil, false
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(time.Second):
		return nil, true
	}
	var sub []tea.Cmd
	switch v := msg.(type) {
	case tea.BatchMsg:
		sub = v
	case nil:
	default:
		// tea.Sequence's message type is unexported; reflect on its shape.
		rv := reflect.ValueOf(msg)
		if rv.Kind() != reflect.Slice || rv.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
			return []tea.Msg{msg}, false
		}
		for i := range rv.Len() {
			sub = append(sub, rv.Index(i).Interface().(tea.Cmd)) //nolint:forcetypeassert // element type checked above
		}
	}
	for _, c := range sub {
		m, b := drain(t, c)
		msgs, blocked = append(msgs, m...), blocked || b
	}
	return msgs, blocked
}

// Opening the downloads view (from a key or at startup) reads the downloads
// once and asks the core for a Sync: it starts no timer. It used to start one
// more polling loop each time.
func TestOpeningDownloadsView_ReadsTheListAndAsksForASync(t *testing.T) {
	m, fake := newDownloadsTestModel(t)
	if _, err := m.Core.Downloads().Queue(bobsAlbum()); err != nil {
		t.Fatalf("queue: %v", err)
	}
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)
	fake.SetTransfers(slskd.DownloadFile{
		ID: "t1", Username: "bob", Filename: `@@bob\Music\Album\01.flac`, State: "InProgress", BytesTransferred: 1,
	})

	opened := time.Now()
	msgs, blocked := drain(t, m.loadAndRefreshDownloads())

	if blocked {
		t.Error("opening the view scheduled a timer")
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want the list read once: %#v", len(msgs), msgs)
	}
	if list, ok := msgs[0].(DownloadsChangedMsg); !ok || len(list.Downloads) != 1 {
		t.Errorf("message = %#v, want the one download", msgs[0])
	}

	// The Sync it asked for, not the core's clock, brings the new state.
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)
	if got := m.DownloadsView.Downloads(); len(got) != 1 || got[0].Status != core.DownloadDownloading {
		t.Errorf("downloads view = %+v, want the download downloading", got)
	}
	if waited := time.Since(opened); waited > 2*time.Second {
		t.Errorf("the view waited %v for the core's clock: opening it asked for no Sync", waited)
	}
}

// trustTags is the library as a retag popup built by a test sees it: Low's
// Trust, one file whose tags name the release "rel".
type trustTags struct{}

func (trustTags) AlbumTags(string, string) ([]core.FileTags, error) {
	return []core.FileTags{{Path: "/music/low/trust/01.mp3", AlbumArtist: "low", Album: "trust", MBReleaseID: "rel"}}, nil
}

func (trustTags) Retag(core.RetagRequest) error { return nil }

// The retag popup was closed while it loaded the details of a release: they
// come once the download popup is opened, which never asked for them. The
// download popup drops them and stays on its artist search.
func TestDownloadPopup_DropsTheReleaseDetailsAskedByAClosedRetagPopup(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	tags := m.Popups.Show(popupctl.Retag, retag.New("low", "trust", lowMB{}, trustTags{}))
	m, details := updateModel(t, m, tags()) // the tags, then the release they name
	if details == nil {
		t.Fatal("the retag popup asked MusicBrainz nothing")
	}
	m, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = updateModel(t, m, cmd())
	if m.Popups.IsVisible(popupctl.Retag) {
		t.Fatal("the retag popup is still open")
	}
	m.Popups.ShowDownload(m.Core.Downloads(), m.Core.MusicBrainz(), download.FilterConfig{}, m.Core.Library())

	late, ok := details().(mbworkflow.ReleaseDetailsResultMsg)
	if !ok || late.Err != nil {
		t.Fatalf("the retag popup's request answered %#v, want the details of rel", late)
	}
	if r := testutil.PanicOf(func() { m, cmd = updateModel(t, m, late) }); r != nil {
		t.Fatalf("Update panics: %v", r)
	}

	if cmd != nil {
		t.Errorf("Update started %#v", cmd())
	}
	if dl := m.Popups.Download(); dl == nil || dl.State() != download.StateSearch {
		t.Error("the download popup left its artist search")
	}
}
