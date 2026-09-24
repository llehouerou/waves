package popup

import (
	"testing"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/tags"
)

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
