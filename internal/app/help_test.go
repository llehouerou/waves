package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// With the queue focused, the help lists its cursor keys, and tells them
// apart from the keys that move the selected tracks.
func TestHelp_FocusedQueueListsCursorKeys(t *testing.T) {
	m := newTestModel(t)
	m.Navigation.SetFocus(navctl.FocusQueue)

	help := helpbindings.New()
	help.SetContexts(m.helpScope())
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
	help.SetContexts(m.helpScope())
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

// focusTargets names the two focus targets, for the tests that run under each.
var focusTargets = map[string]navctl.FocusTarget{
	"navigator": navctl.FocusNavigator,
	"queue":     navctl.FocusQueue,
}

// The search keys, / and "f f", search the view shown, with the navigator or
// the queue focused: the help lists each once where they act, and neither in
// the Downloads view, which has no search; there it keeps the other f keys
// and the queue's keys.
func TestHelp_SearchKeyOnlyWhereItSearches(t *testing.T) {
	views := map[string]func(m *Model){
		"files":   func(m *Model) { m.Navigation.SetViewMode(navctl.ViewFileBrowser) },
		"library": func(m *Model) { m.Navigation.SetViewMode(navctl.ViewLibrary) },
		"library album": func(m *Model) {
			m.Navigation.SetViewMode(navctl.ViewLibrary)
			m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
		},
		"library browser": func(m *Model) {
			m.Navigation.SetViewMode(navctl.ViewLibrary)
			m.Navigation.SetLibrarySubMode(navctl.LibraryModeBrowser)
		},
		"playlists": func(m *Model) { m.Navigation.SetViewMode(navctl.ViewPlaylists) },
		"downloads": func(m *Model) { m.Navigation.SetViewMode(navctl.ViewDownloads) },
	}
	for name, setView := range views {
		for focusName, focus := range focusTargets {
			t.Run(name+"/"+focusName, func(t *testing.T) {
				m := newTestModel(t)
				setView(m)
				m.Navigation.SetFocus(focus)
				view := helpView(m)

				var want map[string][]string
				if name == "downloads" {
					want = map[string][]string{"/": nil, "f f": nil}
				} else {
					want = map[string][]string{"/": {"Search"}, "f f": {"Deep search"}}
				}
				for keys, descs := range want {
					if got := helpDescriptions(view, keys); !slices.Equal(got, descs) {
						t.Errorf("help lines for %q read %q, want %q", keys, got, descs)
					}
				}
				if name != "downloads" {
					return
				}
				kept := []string{"f", "f d", "f l", "f n", "f y", "f1", "f2", "f3", "f4"}
				if focus == navctl.FocusQueue {
					kept = append(kept, "x", "L")
				}
				for _, keys := range kept {
					if got := helpDescriptions(view, keys); len(got) != 1 {
						t.Errorf("help lines for %q read %q, want one", keys, got)
					}
				}
			})
		}
	}
}

// The search keys, / and "f f", start a search in the Playlists view, with
// either focus, and do nothing in the Downloads view: no search, no popup, no
// message.
func TestSearchKeys_SearchOnlyWhereThereIsASearch(t *testing.T) {
	for _, view := range []navctl.ViewMode{navctl.ViewPlaylists, navctl.ViewDownloads} {
		for focusName, focus := range focusTargets {
			for _, keys := range [][]string{{"/"}, {"f", "f"}} {
				t.Run(string(view)+"/"+focusName+"/"+strings.Join(keys, " "), func(t *testing.T) {
					m := newLibraryTestModel(t, nil)
					m.Navigation.SetViewMode(view)
					m.Layout.ShowQueue()
					m.SetFocus(focus)

					for _, key := range keys {
						m, _ = updateModel(t, m, keyMsg(key))
					}

					if got, want := m.Input.IsSearchActive(), view != navctl.ViewDownloads; got != want {
						t.Errorf("search active = %v, want %v", got, want)
					}
					if m.Popups.ActivePopup() != popupctl.None {
						t.Errorf("popup %v shown, want none", m.Popups.ActivePopup())
					}
					if len(m.Notifications) != 0 {
						t.Errorf("notifications = %+v, want none", m.Notifications)
					}
				})
			}
		}
	}
}
