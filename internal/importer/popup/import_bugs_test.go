package popup

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// importAll drives h through a whole import in which every file lands in
// albumDir, and returns the command the last file's result produced.
func importAll(t *testing.T, h *testutil.PopupHarness, albumDir string) tea.Cmd {
	t.Helper()
	h.SendEnter() // path preview
	h.SendEnter() // start import
	m := getModel(t, h)
	if m.state != StateImporting {
		t.Fatalf("import not started: state %d", m.state)
	}
	var cmd tea.Cmd
	for i := range m.importStatus {
		cmd = h.SendMsg(FileImportedMsg{Index: i, DestPath: filepath.Join(albumDir, filepath.Base(m.importStatus[i].Filename))})
	}
	return cmd
}

// Closing mid-import stopped the import after the file in flight, and that
// file's late result could land in the next import popup opened. The import
// always ends by itself: Esc waits for it.
func TestImport_EscapeWhileImportingDoesNotClose(t *testing.T) {
	h := newImportPopupWithTags()
	m := getModel(t, h)
	m.coverArtFetched = true
	h.SendEnter() // path preview
	h.SendEnter() // start import
	h.ClearCommands()

	h.SendEscape()

	if cmd := h.LastCommand(); cmd != nil {
		t.Errorf("Esc while importing emitted %#v", cmd())
	}
	if m.state != StateImporting {
		t.Errorf("state = %d, want StateImporting", m.state)
	}
}

// A popup closed in tag preview still has its tag read, MusicBrainz refresh
// and cover fetch in flight; their results must not land in the next import
// popup, where they would switch its release or give it the wrong cover.
func TestImport_IgnoresAnotherDownloadsResults(t *testing.T) {
	h := newImportPopup() // download 1
	other := int64(2)

	if cmd := h.SendMsg(TagsReadMsg{DownloadID: other, Tags: []tags.FileInfo{{Tag: tags.Tag{MBReleaseID: "elsewhere"}}}}); cmd != nil {
		t.Error("another download's tags started a release refresh")
	}
	h.SendMsg(MBReleaseRefreshedMsg{DownloadID: other, Release: &musicbrainz.ReleaseDetails{Release: musicbrainz.Release{ID: "elsewhere"}}})
	h.SendMsg(CoverArtFetchedMsg{DownloadID: other, Data: []byte("not ours")})

	m := getModel(t, h)
	if id := m.download.MBReleaseDetails.ID; id != "abc123" {
		t.Errorf("release switched to %q by another download's refresh", id)
	}
	if m.currentTags != nil || m.coverArtFetched || m.coverArt != nil {
		t.Errorf("took another download's results: tags %v, cover fetched %v", m.currentTags, m.coverArtFetched)
	}

	h.SendMsg(CoverArtFetchedMsg{DownloadID: 1})
	if !m.coverArtFetched {
		t.Error("ignored its own cover art")
	}
}

// Waves only downloads audio, so an image in the (possibly shared) source
// folder is never this download's: the album gets the cover fetched from the
// Cover Art Archive, and the source folder is left alone.
func TestImport_AlbumGetsTheFetchedCover(t *testing.T) {
	completed, library := t.TempDir(), t.TempDir()
	dl := sampleDownload()
	foreign := filepath.Join(completed, dl.SlskdDirectory, "cover.jpg")
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("another download's cover"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetched := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 16)...) // a JPEG header
	h := testutil.NewPopupHarness(New(dl, completed, []string{library}, nil, rename.Config{}))
	h.SendMsg(TagsReadMsg{DownloadID: dl.ID})
	h.SendMsg(CoverArtFetchedMsg{DownloadID: dl.ID, Data: fetched})
	albumDir := filepath.Join(library, "Test Artist", "Test Album")

	if cmd := importAll(t, h, albumDir); cmd != nil {
		cmd()
	}

	if got, err := os.ReadFile(filepath.Join(albumDir, "cover.jpg")); err != nil || !bytes.Equal(got, fetched) {
		t.Errorf("album cover = %q, %v; want the fetched cover", got, err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("the source folder's image was taken: %v", err)
	}
}

// A download with more files than the release has tracks used to map every
// extra file onto the last track, and the move overwrote it. An extra file
// has no destination: the preview says so, and its import fails untouched.
func TestImport_ExtraFileHasNoTrack(t *testing.T) {
	dl := sampleDownload() // 3 tracks
	dl.Files = append(dl.Files, downloads.DownloadFile{ID: 4, Filename: "04 - Bonus.flac"})
	m := New(dl, t.TempDir(), sampleLibrarySources(), nil, rename.Config{})
	m.SetSize(100, 40)
	h := testutil.NewPopupHarness(m)
	h.SendMsg(TagsReadMsg{DownloadID: dl.ID})
	h.SendMsg(CoverArtFetchedMsg{DownloadID: dl.ID})

	h.SendEnter() // path preview
	if err := h.AssertViewContains("no MusicBrainz track"); err != "" {
		t.Error(err)
	}

	h.SendEnter() // start import
	h.SendMsg(FileImportedMsg{Index: 0, DestPath: "/music/library1/01.flac"})
	h.SendMsg(FileImportedMsg{Index: 1, DestPath: "/music/library1/02.flac"})
	next := h.SendMsg(FileImportedMsg{Index: 2, DestPath: "/music/library1/03.flac"})
	if next == nil {
		t.Fatal("the extra file was never imported")
	}

	res, ok := next().(FileImportedMsg)
	if !ok || res.Index != 3 || res.Err == nil || !strings.Contains(res.Err.Error(), "no MusicBrainz track") {
		t.Errorf("extra file's import = %#v, want it failed for having no MusicBrainz track", res)
	}
}

// Files import in track order; a failure must name the file that failed, not
// the one at the same position in the download's own (unsorted) order.
func TestImport_FailureNamesTheFileThatFailed(t *testing.T) {
	dl := sampleDownload()
	dl.Files[0], dl.Files[1] = dl.Files[1], dl.Files[0] // 02, 01, 03
	m := New(dl, t.TempDir(), sampleLibrarySources(), nil, rename.Config{})
	m.SetSize(100, 40)
	h := testutil.NewPopupHarness(m)
	h.SendMsg(TagsReadMsg{DownloadID: dl.ID})
	h.SendMsg(CoverArtFetchedMsg{DownloadID: dl.ID})
	h.SendEnter() // path preview: 01, 02, 03
	h.SendEnter() // start import with 01

	h.SendMsg(FileImportedMsg{Index: 0, Err: errors.New("disk full")})
	h.SendMsg(FileImportedMsg{Index: 1, DestPath: "/music/library1/02.flac"})
	h.SendMsg(FileImportedMsg{Index: 2, DestPath: "/music/library1/03.flac"})
	h.SendMsg(LibraryRefreshedMsg{DownloadID: dl.ID})

	if err := h.AssertViewContains("01 - Track One.flac"); err != "" {
		t.Errorf("the failure doesn't name the file that failed: %s", err)
	}
	if err := h.AssertViewNotContains("02 - Track Two.flac"); err != "" {
		t.Errorf("a file that imported is reported as failed: %s", err)
	}
}
