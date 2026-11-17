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
