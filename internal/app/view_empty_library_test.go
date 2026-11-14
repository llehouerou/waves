package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/ui/styles"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// The empty-library message advertises the library sources key; that key must
// be highlighted (issue #146) and must come from the key binding, not a stale
// literal.
func TestRenderEmptyLibraryHighlightsLibrarySourcesKey(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(oldProfile) })

	key := ""
	for _, b := range keymap.Bindings {
		if b.Action == keymap.ActionLibrarySources && len(b.Keys) > 0 {
			key = b.Keys[0]
			break
		}
	}
	if key == "" {
		t.Fatal("precondition: the library sources action has no binding")
	}

	m := newIntegrationTestModel(t)
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.loadingState = loadingDone
	m.HasLibrarySources = false

	out := m.renderPanels()
	if !strings.Contains(testutil.StripANSI(out), "Press  "+key+"  to open the library sources manager") {
		t.Fatalf("empty-library message should advertise %q:\n%s", key, out)
	}

	hint := styles.T().S().Base.Bold(true).Foreground(styles.T().Primary).Render(key)
	if !strings.Contains(out, hint) {
		t.Errorf("empty-library message should highlight %q; got:\n%q", key, out)
	}
}
