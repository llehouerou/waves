package popup_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// A dialog renders at every terminal size without panicking: it draws what
// fits (issues #201 and #202).
func TestDialog_RendersAtAnyTerminalSizeWithoutPanicking(t *testing.T) {
	d := popup.New()
	d.Title = "Error"
	d.Content = "The library could not be scanned"
	d.Footer = "Press any key"
	for width := range 11 {
		if r := testutil.PanicOf(func() { d.Render(width, 20) }); r != nil {
			t.Errorf("width %d panics: %v", width, r)
		}
	}
	for height := range 11 {
		if r := testutil.PanicOf(func() { d.Render(termWidth, height) }); r != nil {
			t.Errorf("height %d panics: %v", height, r)
		}
	}
}

const footer = "Press any key to dismiss"

// termWidth is the width of the terminal the dialog is rendered in: its text
// area is then 74 columns at most.
const termWidth = 80

// renderError renders the dialog the error popup builds, and returns the lines
// of its box without their ANSI codes and without the margin that centres it.
func renderError(t *testing.T, message string, termHeight int) []string {
	t.Helper()
	d := popup.New()
	d.Title = "Error"
	d.Content = message
	d.Footer = footer
	out := d.Render(termWidth, termHeight)
	if !utf8.ValidString(out) {
		t.Fatalf("the dialog is not valid UTF-8:\n%q", out)
	}
	var box []string
	for line := range strings.SplitSeq(testutil.StripANSI(out), "\n") {
		if strings.TrimSpace(line) != "" {
			box = append(box, strings.TrimLeft(line, " "))
		}
	}
	assertBox(t, box)
	return box
}

// assertBox fails unless every line of the box has the same width, at most the
// terminal width minus 2, between a left and a right border.
func assertBox(t *testing.T, box []string) {
	t.Helper()
	if len(box) < 2 || !strings.HasPrefix(box[0], "╭") || !strings.HasPrefix(box[len(box)-1], "╰") {
		t.Fatalf("no box with a top and a bottom border:\n%s", strings.Join(box, "\n"))
	}
	width := ansi.StringWidth(box[0])
	if width > termWidth-2 {
		t.Errorf("the box is %d columns wide, more than %d", width, termWidth-2)
	}
	for i, line := range box {
		if w := ansi.StringWidth(line); w != width {
			t.Errorf("line %d is %d columns wide, the top border %d:\n%s", i, w, width, strings.Join(box, "\n"))
		}
		if i > 0 && i < len(box)-1 && (!strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│")) {
			t.Errorf("line %d is not between two borders: %q", i, line)
		}
	}
}

// messageLines returns the text of the message lines of the box: the lines
// between the blank line under the title and the blank line over the footer.
func messageLines(box []string) []string {
	lines := make([]string, 0, len(box)-6)
	for _, line := range box[3 : len(box)-3] {
		text := strings.TrimSuffix(strings.TrimPrefix(line, "│"), "│")
		lines = append(lines, strings.TrimRight(strings.TrimPrefix(text, " "), " "))
	}
	return lines
}

func TestDialog_ShortMessageKeepsTheFooterOnOneLine(t *testing.T) {
	box := renderError(t, "Failed to play", 24)
	if len(box) != 7 {
		t.Fatalf("the popup is %d lines tall, want 7:\n%s", len(box), strings.Join(box, "\n"))
	}
	if !testutil.ContainsLine(strings.Join(box, "\n"), footer) {
		t.Errorf("the footer is not on one line:\n%s", strings.Join(box, "\n"))
	}
}

func TestDialog_MessageAsWideAsTheTextAreaStaysOnOneLine(t *testing.T) {
	message := strings.Repeat("x", 74)
	box := renderError(t, message, 24)
	if got := messageLines(box); len(got) != 1 || got[0] != message {
		t.Errorf("message lines %q, want the message on one line", got)
	}
}

func TestDialog_MessageWiderThanTheTextAreaIsWrappedWhole(t *testing.T) {
	message := strings.Repeat("x", 70) + " abcd"
	box := renderError(t, message, 24)
	got := messageLines(box)
	want := []string{strings.Repeat("x", 70), "abcd"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("message lines %q, want %q", got, want)
	}
}

func TestDialog_LongMessageIsWrappedAtSpaces(t *testing.T) {
	message := "Failed to play: /music/Some Artist/Some Album With A Long Name (Deluxe Edition)/01 - A Track.flac: open: no such file or directory"
	box := renderError(t, message, 24)
	got := messageLines(box)
	if len(got) < 2 {
		t.Fatalf("message lines %q, want it wrapped", got)
	}
	// Breaking at spaces only, the lines joined by a space are the message.
	if joined := strings.Join(got, " "); joined != message {
		t.Errorf("message lines %q, joined %q, want %q", got, joined, message)
	}
}

func TestDialog_WordWiderThanTheTextAreaGoesOnOverTheNextLine(t *testing.T) {
	word := "/" + strings.Repeat("abcdefghi/", 10)[:99]
	box := renderError(t, word, 24)
	got := messageLines(box)
	if len(got) != 2 || strings.Join(got, "") != word {
		t.Errorf("message lines %q, want the word %q over two lines", got, word)
	}
}

func TestDialog_EachMessageLineIsWrappedOnItsOwn(t *testing.T) {
	first := "The library could not be scanned"
	second := strings.Repeat("word ", 20) + "end"
	box := renderError(t, first+"\n"+second, 24)
	got := messageLines(box)
	if len(got) < 3 || got[0] != first {
		t.Fatalf("message lines %q, want %q then the second line wrapped", got, first)
	}
	if joined := strings.Join(got[1:], " "); joined != second {
		t.Errorf("second line wrapped into %q, want %q", got[1:], second)
	}
}

func TestDialog_WrapsNonASCIITextByDisplayWidth(t *testing.T) {
	message := strings.Repeat("Ошибка воспроизведения файла ", 8)
	box := renderError(t, message, 24)
	got := messageLines(box)
	if len(got) < 2 {
		t.Fatalf("message lines %q, want it wrapped", got)
	}
	for i, line := range got[:len(got)-1] {
		if w := ansi.StringWidth(line); w <= 74/2 {
			t.Errorf("line %d is %d columns wide, no more than half of the text area: %q", i, w, line)
		}
	}
}

func TestDialog_MessageTallerThanTheTerminalIsCut(t *testing.T) {
	// Lines as wide as the text area: the "..." takes the place of their end.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = strings.Repeat(string(rune('a'+i)), 74)
	}
	box := renderError(t, strings.Join(lines, "\n"), 16)
	if len(box) > 16 {
		t.Errorf("the dialog is %d lines tall, more than 16", len(box))
	}
	view := strings.Join(box, "\n")
	if !strings.Contains(box[1], "Error") {
		t.Errorf("no title on the first line inside the border:\n%s", view)
	}
	if !testutil.ContainsLine(box[len(box)-2], footer) {
		t.Errorf("no footer on the last line inside the border:\n%s", view)
	}
	got := messageLines(box)
	if last := got[len(got)-1]; !strings.HasSuffix(last, "...") {
		t.Errorf("the last message line shown %q does not end with ...", last)
	}
}

// A " - " at the end of a line, as in a track file name, is no cause to cut
// the line: the message stays whole.
func TestDialog_HyphenAtTheWrapPointKeepsTheMessageWhole(t *testing.T) {
	for _, n := range []int{46, 47, 48} {
		message := "Failed to play: /music/" + strings.Repeat("a", n) + "/01 - A Track.flac: open: no such file or directory"
		got := messageLines(renderError(t, message, 24))
		if joined := strings.Join(got, " "); joined != message {
			t.Errorf("message lines %q, want %q wrapped at spaces", got, message)
		}
	}
}

// A hyphenated word that fits on the next line is not split at its hyphen.
func TestDialog_HyphenatedWordIsNotSplit(t *testing.T) {
	message := strings.Repeat("x", 70) + " ab-cd"
	got := messageLines(renderError(t, message, 24))
	want := []string{strings.Repeat("x", 70), "ab-cd"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("message lines %q, want %q", got, want)
	}
}

// A view of TextRows rows of TextWidth columns fills the box RenderBordered
// draws: one more row, or one more column, makes it a row taller.
func TestRenderBordered_TextAreaIsTextWidthByTextRows(t *testing.T) {
	const width, height = 40, 12
	row := strings.Repeat("x", popup.TextWidth(width))
	view := strings.TrimSuffix(strings.Repeat(row+"\n", popup.TextRows(height)), "\n")
	if got := testutil.BoxRows(view, width, height); got != height {
		t.Errorf("box of a full view: %d rows, want %d", got, height)
	}
	if got := testutil.BoxRows(view+"\nx", width, height); got != height+1 {
		t.Errorf("box of a view a row taller: %d rows, want %d", got, height+1)
	}
	if got := testutil.BoxRows("x "+view, width, height); got != height+1 {
		t.Errorf("box of a view with a wider row: %d rows, want %d", got, height+1)
	}
}
