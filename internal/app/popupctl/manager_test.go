package popupctl

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// growing is a popup that records the size and the limit it was given.
type growing struct {
	width, height, maxHeight int
}

func (g *growing) Init() tea.Cmd { return nil }

func (g *growing) Update(tea.Msg) (popup.Popup, tea.Cmd) { return g, nil }

func (g *growing) View() string { return "" }

func (g *growing) SetSize(width, height int) { g.width, g.height = width, height }

func (g *growing) SetMaxHeight(height int) { g.maxHeight = height }

func (g *growing) assert(t *testing.T, width, height, limit int) {
	t.Helper()
	if g.width != width || g.height != height || g.maxHeight != limit {
		t.Errorf("popup sized %dx%d up to %d rows, want %dx%d up to %d",
			g.width, g.height, g.maxHeight, width, height, limit)
	}
}

// The import and retag popups may grow up to the rows above the player bar:
// the manager tells them when it shows them, and again on every resize.
func TestManager_TellsAGrowingPopupTheRowsAboveThePlayerBar(t *testing.T) {
	for _, typ := range []Type{Import, Retag} {
		p := New()
		p.SetSize(80, 24)
		g := &growing{}
		p.Show(typ, g)
		g.assert(t, 64, 16, 24)

		p.SetSize(120, 40)
		g.assert(t, 96, 28, 40)

		p.SetBottomMargin(10)
		g.assert(t, 96, 21, 30)
	}
}
