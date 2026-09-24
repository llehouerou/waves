package popup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
