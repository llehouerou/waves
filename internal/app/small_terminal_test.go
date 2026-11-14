package app

import (
	"database/sql"
	"fmt"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/ui/download"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/layout"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// The main screen renders at every terminal size without panicking: every
// panel draws in the room it is given, down to none (issue #179).
func TestMainScreen_RendersAtAnyTerminalSizeWithoutPanicking(t *testing.T) {
	views := []struct {
		name  string
		build func(t *testing.T) Model
	}{
		{"library with no source", func(t *testing.T) Model {
			return newLibraryTestModel(t, nil)
		}},
		{"library in Miller columns", func(t *testing.T) Model {
			return inLibraryMode(t, navctl.LibraryModeMiller)
		}},
		{"library in album view", func(t *testing.T) Model {
			return inLibraryMode(t, navctl.LibraryModeAlbum)
		}},
		{"library browser", func(t *testing.T) Model {
			return inLibraryMode(t, navctl.LibraryModeBrowser)
		}},
		{"file browser", func(t *testing.T) Model {
			return sendKey(t, newSeededTestModel(t), tea.KeyMsg{Type: tea.KeyF2})
		}},
		{"playlists", func(t *testing.T) Model {
			return sendKey(t, newSeededTestModel(t), tea.KeyMsg{Type: tea.KeyF3})
		}},
		{"downloads, slskd not configured", func(t *testing.T) Model {
			return sendKey(t, newSeededTestModel(t), tea.KeyMsg{Type: tea.KeyF4})
		}},
		{"downloads, slskd configured", func(t *testing.T) Model {
			m, _ := newDownloadsTestModel(t)
			return sendKey(t, m, tea.KeyMsg{Type: tea.KeyF4})
		}},
		{"downloads, with a download", func(t *testing.T) Model {
			m, _ := newDownloadsTestModel(t)
			if _, err := m.Core.Downloads().Queue(bobsAlbum()); err != nil {
				t.Fatalf("queue: %v", err)
			}
			m = pumpUntil[ServiceDownloadsChangedMsg](t, m)
			return sendKey(t, m, tea.KeyMsg{Type: tea.KeyF4})
		}},
	}
	playbacks := []struct {
		name string
		set  func(t *testing.T, m Model) Model
	}{
		{"nothing playing", func(_ *testing.T, m Model) Model { return m }},
		{"playing, compact player bar", func(t *testing.T, m Model) Model {
			return playing(t, m, playerbar.ModeCompact)
		}},
		{"playing, expanded player bar", func(t *testing.T, m Model) Model {
			return playing(t, m, playerbar.ModeExpanded)
		}},
		{"playing, with a job bar and a notification", func(t *testing.T, m Model) Model {
			m.LibraryScanJob = &jobbar.Job{ID: libraryRefreshJobID, Label: "Scanning library", Current: 3}
			m.Notifications = append(m.Notifications, Notification{ID: 1, Message: "Hello"})
			return playing(t, m, playerbar.ModeExpanded)
		}},
	}

	for _, view := range views {
		for _, playback := range playbacks {
			for _, queueShown := range []bool{true, false} {
				name := fmt.Sprintf("%s/%s/queue shown %v", view.name, playback.name, queueShown)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					m := playback.set(t, view.build(t))
					if !queueShown {
						m = sendKey(t, m, keyMsg("p"))
					}
					sweepTerminalSizes(t, m)
				})
			}
		}
	}
}

// sweepTerminalSizes resizes m to the narrowest widths, widths on each side of
// the narrow threshold and every height up to 20, renders it at each, and
// fails on the first size that panics.
func sweepTerminalSizes(t *testing.T, m Model) {
	t.Helper()
	widths := []int{0, 1, 2, 3, 4, 5, 80, layout.NarrowThreshold - 1, layout.NarrowThreshold}
	for _, width := range widths {
		for height := range 21 {
			if msg := renderAt(t, m, width, height); msg != nil {
				t.Fatalf("%d×%d panics: %v", width, height, msg)
			}
		}
	}
}

// renderAt resizes m to width×height and renders it: what it panics with, or
// nil.
func renderAt(t *testing.T, m Model, width, height int) (panicked any) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = fmt.Sprintf("%v\n%s", r, debug.Stack())
		}
	}()
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	_ = m.View()
	return nil
}

// newSeededTestModel is a model on a library source holding one album of three
// tracks, and a playlist of them.
func newSeededTestModel(t *testing.T) Model {
	t.Helper()
	source := t.TempDir()
	return newLibraryTestModel(t, func(db *sql.DB) { seedPlaylistOfAlbum(t, db, source) })
}

// inLibraryMode is a seeded model showing its library in mode.
func inLibraryMode(t *testing.T, mode navctl.LibrarySubMode) Model {
	t.Helper()
	m := newSeededTestModel(t)
	if mode == navctl.LibraryModeAlbum {
		// V switches from the browser to the album view, and loads it.
		m.Navigation.SetLibrarySubMode(navctl.LibraryModeBrowser)
		m = sendKey(t, m, keyMsg("V"))
	} else {
		m.Navigation.SetLibrarySubMode(mode)
		m.Navigation.SetFocus(m.Navigation.Focus())
	}
	if got := m.Navigation.LibrarySubMode(); got != mode {
		t.Fatalf("library sub-mode = %v, want %v", got, mode)
	}
	return m
}

// sendKey feeds key to m, without running the commands it returns.
func sendKey(t *testing.T, m Model, key tea.Msg) Model {
	t.Helper()
	m, _ = updateModel(t, m, key)
	return m
}

// playing fills the queue and plays its first track, with the player bar in
// mode: the track has tags, so the player bar draws.
func playing(t *testing.T, m Model, mode playerbar.DisplayMode) Model {
	t.Helper()
	tracks := make([]core.Track, 30)
	for i := range tracks {
		tracks[i] = core.Track{Path: fmt.Sprintf("/music/%02d.mp3", i), Title: fmt.Sprintf("Track %d", i), Artist: "Artist"}
	}
	m.Core.Playback().AddTracks(tracks...)
	m.Core.Playback().QueueMoveTo(0)
	testPlayer(m.Core).SetState(player.Playing)
	testPlayer(m.Core).SetTrackInfo(&tags.FileInfo{
		Tag:       tags.Tag{Title: "Track 0", Artist: "Artist", Album: "Album"},
		AudioInfo: tags.AudioInfo{Duration: 3 * time.Minute},
	})
	m.Layout.SetPlayerDisplayMode(mode)
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// A popup or the search open renders at every terminal size without
// panicking: it draws what fits, or nothing (issue #201).
func TestPopups_RenderAtAnyTerminalSizeWithoutPanicking(t *testing.T) {
	popups := []struct {
		name string
		open func(t *testing.T) Model
	}{
		{"error popup, short message", func(t *testing.T) Model {
			m := newSeededTestModel(t)
			m.Popups.ShowError("Oops")
			return m
		}},
		{"error popup, message wider than the terminal", func(t *testing.T) Model {
			m := newSeededTestModel(t)
			m.Popups.ShowError("The library could not be scanned: permission denied on /music")
			return m
		}},
		{"search with results listed", func(t *testing.T) Model {
			m := sendKey(t, newSeededTestModel(t), keyMsg("/"))
			m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
			// The artist is listed twice: under the cursor of the library and of the search.
			if strings.Count(testutil.StripANSI(m.View()), "> Album Artist") != 2 {
				t.Fatalf("the search lists no result:\n%s", testutil.StripANSI(m.View()))
			}
			return m
		}},
		{"download popup, artist search with an empty input", func(t *testing.T) Model {
			m, _ := newDownloadsTestModel(t)
			m.Popups.ShowDownload(m.Core.Downloads(), m.Core.MusicBrainz(), download.FilterConfig{}, m.Core.Library())
			if dl := m.Popups.Download(); dl == nil || dl.State() != download.StateSearch {
				t.Fatal("the download popup is not on its artist search step")
			}
			return m
		}},
	}

	for _, p := range popups {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			m := p.open(t)
			for width := range 16 {
				for height := range 21 {
					if msg := renderAt(t, m, width, height); msg != nil {
						t.Fatalf("%d×%d panics: %v", width, height, msg)
					}
				}
			}
		})
	}
}
