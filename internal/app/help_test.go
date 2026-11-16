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
