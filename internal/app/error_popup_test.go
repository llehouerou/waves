package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/llehouerou/waves/internal/ui/testutil"
)

const errorPopupFooter = "Press any key to dismiss"

// errorPopupBox shows message in the error popup of a model sized to
// width×height, and returns the lines of the popup's box in the view, without
// their ANSI codes.
func errorPopupBox(t *testing.T, message string, width, height int) []string {
	t.Helper()
	m := newSeededTestModel(t)
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m.Popups.ShowError(message)
	view := strings.Split(testutil.StripANSI(m.View()), "\n")

	// The title row locates the box: its top border is the row above it, from
	// the last corner left of the title to the first corner right of it.
	top, left, right := -1, -1, -1
	for i := 1; i < len(view) && top < 0; i++ {
		at := strings.Index(view[i], " Error ")
		if at < 0 {
			continue
		}
		col := ansi.StringWidth(view[i][:at])
		border := view[i-1]
		for c := col; c >= 0 && left < 0; c-- {
			if ansi.Cut(border, c, c+1) == "╭" {
				left = c
			}
		}
		for c := col; c < width && right < 0; c++ {
			if ansi.Cut(border, c, c+1) == "╮" {
				right = c
			}
		}
		top = i - 1
	}
	if top < 0 || left < 0 || right < 0 {
		t.Fatalf("no error popup in the view:\n%s", strings.Join(view, "\n"))
	}
	var box []string
	for _, line := range view[top:] {
		row := ansi.Cut(line, left, right+1)
		box = append(box, row)
		if strings.HasPrefix(row, "╰") {
			return box
		}
	}
	t.Fatalf("the error popup has no bottom border:\n%s", strings.Join(view, "\n"))
	return nil
}

// errorPopupMessage returns the text of the message lines of the box, failing
// unless every line has the width of the box between two borders.
func errorPopupMessage(t *testing.T, box []string) []string {
	t.Helper()
	width := ansi.StringWidth(box[0])
	var lines []string
	for i, row := range box {
		if w := ansi.StringWidth(row); w != width {
			t.Errorf("line %d is %d columns wide, the top border %d:\n%s", i, w, width, strings.Join(box, "\n"))
		}
		if i == 0 || i == len(box)-1 {
			continue
		}
		if !strings.HasPrefix(row, "│") || !strings.HasSuffix(row, "│") {
			t.Errorf("line %d is not between two borders: %q", i, row)
		}
		if i >= 3 && i < len(box)-3 {
			text := strings.TrimSuffix(strings.TrimPrefix(row, "│"), "│")
			lines = append(lines, strings.TrimSpace(text))
		}
	}
	return lines
}

// The error popup keeps its footer on one line under a short message, and
// shows a message wider than the popup whole over several lines (issue #202).
func TestErrorPopup_FitsItsLinesInTheBox(t *testing.T) {
	sizes := []struct{ width, height int }{{80, 16}, {120, 40}}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("short message at %dx%d", size.width, size.height), func(t *testing.T) {
			box := errorPopupBox(t, "Failed to play", size.width, size.height)
			got := errorPopupMessage(t, box)
			if len(box) != 7 || len(got) != 1 || got[0] != "Failed to play" {
				t.Errorf("%d×%d: box of %d lines, message %q, want 7 lines and the message on one:\n%s",
					size.width, size.height, len(box), got, strings.Join(box, "\n"))
			}
			if !testutil.ContainsLine(box[len(box)-2], errorPopupFooter) {
				t.Errorf("%d×%d: the footer is not on one line:\n%s", size.width, size.height, strings.Join(box, "\n"))
			}
		})
		t.Run(fmt.Sprintf("long message at %dx%d", size.width, size.height), func(t *testing.T) {
			message := "Failed to play: /music/Some Artist/Some Album With A Long Name (Deluxe Edition)/" +
				"01 - A Track Whose Title Goes On And On.flac: open: no such file or directory"
			box := errorPopupBox(t, message, size.width, size.height)
			got := errorPopupMessage(t, box)
			if joined := strings.Join(got, " "); len(got) < 2 || joined != message {
				t.Errorf("%d×%d: message lines %q, want %q wrapped at spaces", size.width, size.height, got, message)
			}
		})
	}
}
