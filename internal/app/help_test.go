package app

import (
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// With the queue focused, the help lists its cursor keys, and tells them
// apart from the keys that move the selected tracks.
func TestHelp_FocusedQueueListsCursorKeys(t *testing.T) {
	m := newTestModel(t)
	m.Navigation.SetFocus(navctl.FocusQueue)

	help := helpbindings.New()
	help.SetContexts(m.applicableContexts())
	help.SetSize(120, 500)
	view := testutil.StripANSI(help.View())

	for keys, desc := range map[string]string{
		"j, down":       "Move down",
		"k, up":         "Move up",
		"J, shift+down": "Move selected down",
		"K, shift+up":   "Move selected up",
	} {
		line := testutil.FindLine(view, keys+" ")
		if line == "" {
			t.Errorf("no help line for %q in:\n%s", keys, view)
			continue
		}
		if got := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), keys)); got != desc {
			t.Errorf("help line for %q reads %q, want %q", keys, got, desc)
		}
	}
}

// An export goes to a device or a folder: the help says so in the queue and in
// the Library view, the two places where e exports.
func TestHelp_ExportKeyNamesDeviceOrFolder(t *testing.T) {
	for name, setup := range map[string]func(m *Model){
		"queue":   func(m *Model) { m.Navigation.SetFocus(navctl.FocusQueue) },
		"library": func(m *Model) { m.Navigation.SetViewMode(navctl.ViewLibrary) },
	} {
		t.Run(name, func(t *testing.T) {
			m := newTestModel(t)
			setup(m)

			descs := helpDescriptions(helpView(m), "e")
			if len(descs) != 1 || descs[0] != "Export to a device or a folder" {
				t.Errorf("help lines for e read %q, want one %q", descs, "Export to a device or a folder")
			}
		})
	}
}

// helpDescriptions returns the description of every line of the help view
// that names exactly keys.
func helpDescriptions(view, keys string) []string {
	var descs []string
	for line := range strings.SplitSeq(view, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│"))
		rest, ok := strings.CutPrefix(line, keys+"  ")
		if !ok {
			continue
		}
		descs = append(descs, strings.TrimSpace(rest))
	}
	return descs
}

// helpView renders the help popup for the contexts the model applies.
func helpView(m *Model) string {
	help := helpbindings.New()
	help.SetContexts(m.applicableContexts())
	help.SetSize(120, 500)
	return testutil.StripANSI(help.View())
}

// The downloads view is no navigator: its help lists the keys its list acts
// on, and one enter line, its own.
func TestHelp_DownloadsViewListsOnlyItsKeys(t *testing.T) {
	m := newTestModel(t)
	m.Navigation.SetViewMode(navctl.ViewDownloads)
	view := helpView(m)

	for _, keys := range []string{"h, left", "l, right", "a", "ctrl+a"} {
		if descs := helpDescriptions(view, keys); len(descs) != 0 {
			t.Errorf("help lists %q as %q in the downloads view:\n%s", keys, descs, view)
		}
	}
	if descs := helpDescriptions(view, "enter"); len(descs) != 1 || descs[0] != "Expand/collapse files" {
		t.Errorf("help lines for enter read %q, want one %q", descs, "Expand/collapse files")
	}
	for keys, desc := range map[string]string{
		"j, down": "Move down",
		"k, up":   "Move up",
		"g":       "First item",
		"G":       "Last item",
		"ctrl+d":  "Half page down",
		"ctrl+u":  "Half page up",
	} {
		if descs := helpDescriptions(view, keys); len(descs) != 1 || descs[0] != desc {
			t.Errorf("help lines for %q read %q, want one %q", keys, descs, desc)
		}
	}
}

// ctrl+a adds to a playlist from the Library view only: the help lists it
// there, not in the Files and Playlists views.
func TestHelp_AddToPlaylistOnlyInLibraryView(t *testing.T) {
	for mode, want := range map[navctl.ViewMode][]string{
		navctl.ViewLibrary:     {"Add to playlist"},
		navctl.ViewFileBrowser: nil,
		navctl.ViewPlaylists:   nil,
	} {
		m := newTestModel(t)
		m.Navigation.SetViewMode(mode)
		descs := helpDescriptions(helpView(m), "ctrl+a")
		if len(descs) != len(want) || (len(want) == 1 && descs[0] != want[0]) {
			t.Errorf("view %v: help lines for ctrl+a read %q, want %q", mode, descs, want)
		}
	}
}

// / searches the view shown, with the navigator or the queue focused: the
// help lists it there, and not in the Downloads view, which has no search.
func TestHelp_SearchKeyOnlyWhereItSearches(t *testing.T) {
	for name, setup := range map[string]func(m *Model){
		"files":     func(m *Model) { m.Navigation.SetViewMode(navctl.ViewFileBrowser) },
		"library":   func(m *Model) { m.Navigation.SetViewMode(navctl.ViewLibrary) },
		"playlists": func(m *Model) { m.Navigation.SetViewMode(navctl.ViewPlaylists) },
		"queue":     func(m *Model) { m.Navigation.SetFocus(navctl.FocusQueue) },
		"downloads": func(m *Model) { m.Navigation.SetViewMode(navctl.ViewDownloads) },
	} {
		t.Run(name, func(t *testing.T) {
			m := newTestModel(t)
			setup(m)

			descs := helpDescriptions(helpView(m), "/")
			if name == "downloads" {
				if len(descs) != 0 {
					t.Errorf("help lists / as %q in the downloads view", descs)
				}
				return
			}
			if len(descs) != 1 || descs[0] != "Search" {
				t.Errorf("help lines for / read %q, want one %q", descs, "Search")
			}
		})
	}
}
