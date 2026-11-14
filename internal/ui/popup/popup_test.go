package popup_test

import (
	"testing"

	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// A dialog renders at every terminal width without panicking: it draws what
// fits (issue #201).
func TestDialog_RendersAtAnyTerminalWidthWithoutPanicking(t *testing.T) {
	d := popup.New()
	d.Title = "Error"
	d.Content = "The library could not be scanned"
	d.Footer = "Press any key"
	for width := range 11 {
		if r := testutil.PanicOf(func() { d.Render(width, 20) }); r != nil {
			t.Errorf("width %d panics: %v", width, r)
		}
	}
}
