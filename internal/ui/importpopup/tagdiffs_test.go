package importpopup

import (
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/core"
)

// previewLines are the lines of the tag preview of plan, by field.
func previewLines(t *testing.T, plan core.ImportPlan) map[string]TagDiff {
	t.Helper()
	_, m, _ := preparedPopup(t, plan)
	lines := make(map[string]TagDiff, len(m.tagDiffs))
	for _, d := range m.tagDiffs {
		lines[d.Field] = d
	}
	return lines
}

// taggedPlan is samplePlan on a release credited to idA, a label and a
// genre, with files holding every value the import writes on them.
func taggedPlan() core.ImportPlan {
	plan := samplePlan()
	plan.Release.ArtistID, plan.Release.Label, plan.Release.Genres = "idA", "Sub Pop", []string{"rock"}
	for i := range plan.Files {
		f := &plan.Files[i].Tags
		f.Artist, f.AlbumArtist, f.Album, f.Title = "Test Artist", "Test Artist", "Test Album", plan.Release.Tracks[i].Title
		f.Date, f.OriginalDate, f.Genre, f.ReleaseType = "2024-01-15", "2024-01-15", "Rock", "album"
		f.Label, f.MBArtistID, f.MBReleaseID = "Sub Pop", "idA", "abc123"
	}
	return plan
}

func TestImport_FilesHoldingWhatTheImportWritesHaveNoLineChanged(t *testing.T) {
	for field, d := range previewLines(t, taggedPlan()) {
		if d.Changed {
			t.Errorf("%s: %q -> %q marked changed", field, d.OldValue, d.NewValue)
		}
	}
}

// A file with no track is not imported: tagged as another album, it changes
// no line and shows in no current value.
func TestImport_AFileWithNoTrackCountsInNoLine(t *testing.T) {
	alone := previewLines(t, taggedPlan())
	plan := taggedPlan()
	other := plan.Files[0]
	other.Track = -1
	other.Tags = core.FileTags{
		Artist: "Slowdive", AlbumArtist: "Slowdive", Album: "Souvlaki", Title: "Alison",
		Date: "1993-05-17", OriginalDate: "1993-05-17", Genre: "Shoegaze", ReleaseType: "ep",
		Label: "Creation", Media: "CD", Barcode: "123", CatalogNumber: "CRE", ReleaseStatus: "official",
		Country: "GB", Script: "Latn", MBArtistID: "idS", MBReleaseID: "souvlaki",
	}
	plan.Files = append(plan.Files, other)

	for field, d := range previewLines(t, plan) {
		if d.Changed {
			t.Errorf("%s: %q -> %q marked changed", field, d.OldValue, d.NewValue)
		}
		if d.OldValue != alone[field].OldValue {
			t.Errorf("%s: current value %q, want %q as with the three files alone", field, d.OldValue, alone[field].OldValue)
		}
	}
}

// One file holding what the import writes and two holding nothing: the
// import writes it on the two others.
func TestImport_FilesWithNoValueAreChanged(t *testing.T) {
	plan := samplePlan()
	plan.Release.ArtistID = "idA"
	plan.Files[0].Tags = core.FileTags{Artist: "Test Artist", MBArtistID: "idA", MBReleaseID: "abc123", Date: "2024-01-15"}
	lines := previewLines(t, plan)

	for _, field := range []string{"Artist", "MB Artist ID", "MB Release ID", "Date"} {
		if d := lines[field]; d.OldValue != "(2 different)" || !d.Changed {
			t.Errorf("%s: %q -> %q (changed %v), want (2 different), changed", field, d.OldValue, d.NewValue, d.Changed)
		}
	}
}

// Each file is compared with its own track: files tagged with each other's
// artist are changed.
func TestImport_FilesTaggedWithEachOthersArtistAreChanged(t *testing.T) {
	plan := taggedPlan()
	plan.Release.Tracks[0].Artist, plan.Release.Tracks[0].ArtistIDs = "Artist A", []string{"idA"}
	plan.Release.Tracks[1].Artist, plan.Release.Tracks[1].ArtistIDs = "Artist B", []string{"idB"}
	plan.Files = plan.Files[:2]
	plan.Files[0].Tags.Artist, plan.Files[0].Tags.MBArtistID = "Artist B", "idB"
	plan.Files[1].Tags.Artist, plan.Files[1].Tags.MBArtistID = "Artist A", "idA"
	lines := previewLines(t, plan)

	for _, field := range []string{"Artist", "MB Artist ID"} {
		if d := lines[field]; !d.Changed {
			t.Errorf("%s: %q -> %q not marked changed", field, d.OldValue, d.NewValue)
		}
	}
}

// Files whose release ID tag holds the release imported and another are
// shown as they are, the list against the one ID written (issue #355).
func TestImport_AReleaseIDTagWithSeveralValuesIsChangedToTheOneWritten(t *testing.T) {
	plan := taggedPlan()
	for i := range plan.Files {
		plan.Files[i].Tags.MBReleaseID = "abc123; other"
	}

	d := previewLines(t, plan)["MB Release ID"]
	if !strings.HasPrefix(d.OldValue, "abc123; ") || d.NewValue != "abc123" || !d.Changed {
		t.Errorf("MB Release ID: %q -> %q (changed %v), want the files' list to become abc123", d.OldValue, d.NewValue, d.Changed)
	}
}
