package albumart

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
)

func trueColorBlocks() *BlockProtocol {
	p := NewBlockProtocol()
	p.profile = colorprofile.TrueColor
	return p
}

// Solid image of the given size, useful to check geometry.
func solid(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func renderTestPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// One terminal cell renders two vertical image pixels via the upper half block.
func TestBlockProtocol_OneCellPerTwoPixelRows(t *testing.T) {
	p := trueColorBlocks()

	if _, err := p.Prepare(solid(3, 4, color.White), 1); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(p.Placeholder(3, 2), "\n")
	if len(lines) != 2 {
		t.Fatalf("4 pixel rows should render as 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if n := strings.Count(line, "▀"); n != 3 {
			t.Errorf("line %d has %d blocks, want 3", i, n)
		}
	}
}

// An odd pixel height still fills its last cell rather than dropping the row.
func TestBlockProtocol_OddHeightKeepsLastRow(t *testing.T) {
	p := trueColorBlocks()

	if _, err := p.Prepare(solid(2, 3, color.White), 1); err != nil {
		t.Fatal(err)
	}

	if got := strings.Count(p.Placeholder(2, 2), "▀"); got != 4 {
		t.Errorf("3 pixel rows should fill 2 lines of 2 blocks, got %d blocks", got)
	}
}

// The upper pixel becomes the foreground, the lower one the background.
func TestBlockProtocol_ColorsUpperAndLowerPixel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})

	p := trueColorBlocks()
	if _, err := p.Prepare(img, 1); err != nil {
		t.Fatal(err)
	}

	line := p.Placeholder(1, 1)
	if !strings.Contains(line, "38;2;255;0;0") {
		t.Errorf("missing red foreground in %q", line)
	}
	if !strings.Contains(line, "48;2;0;0;255") {
		t.Errorf("missing blue background in %q", line)
	}
}

// Colorless terminals still get the block glyphs, just without SGR colors.
func TestBlockProtocol_AsciiProfileEmitsNoColor(t *testing.T) {
	p := NewBlockProtocol()
	p.profile = colorprofile.Ascii

	if _, err := p.Prepare(solid(2, 2, color.White), 1); err != nil {
		t.Fatal(err)
	}

	line := p.Placeholder(2, 1)
	if !strings.Contains(line, "▀") {
		t.Errorf("expected block glyphs, got %q", line)
	}
	if strings.Contains(line, "38;2;") {
		t.Errorf("truecolor SGR leaked into an Ascii profile: %q", line)
	}
}

// The art is part of the view text, not a terminal escape written over it:
// Bubble Tea truncates rendered lines to the terminal width, which would cut a
// block image placed with absolute cursor moves (issue #30).
func TestBlockProtocol_PlaceEmitsNothing(t *testing.T) {
	p := trueColorBlocks()
	if _, err := p.Prepare(solid(2, 4, color.White), 1); err != nil {
		t.Fatal(err)
	}

	if out := p.Place(1, 5, 10, 2, 2); out != "" {
		t.Errorf("Place = %q, want empty: block art renders inline", out)
	}
}

// The art occupies exactly the cells the layout reserved for it, whatever the
// image aspect ratio, so nothing shifts around it.
func TestBlockProtocol_PlaceholderFillsTheReservedArea(t *testing.T) {
	p := trueColorBlocks()
	if _, err := p.Prepare(solid(4, 4, color.White), 1); err != nil {
		t.Fatal(err)
	}

	got := p.Placeholder(8, 4)

	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("Placeholder(8, 4) produced %d lines, want 4", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 8 {
			t.Errorf("line %d is %d cells wide, want 8", i, w)
		}
	}
}

// An image taller than the reserved area must not push the layout around.
func TestBlockProtocol_PlaceholderTruncatesOversizedArt(t *testing.T) {
	p := trueColorBlocks()
	if _, err := p.Prepare(solid(8, 16, color.White), 1); err != nil {
		t.Fatal(err)
	}

	got := p.Placeholder(4, 3)

	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("Placeholder(4, 3) produced %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 4 {
			t.Errorf("line %d is %d cells wide, want 4", i, w)
		}
	}
}

// Without an image the placeholder is blank, exactly as before.
func TestBlockProtocol_PlaceholderBlankWithoutImage(t *testing.T) {
	got := trueColorBlocks().Placeholder(4, 3)

	want := strings.Join([]string{"    ", "    ", "    "}, "\n")
	if got != want {
		t.Errorf("Placeholder(4, 3) = %q, want %q", got, want)
	}
}

func TestBlockProtocol_DeleteDropsTheImage(t *testing.T) {
	p := trueColorBlocks()
	if _, err := p.Prepare(solid(2, 2, color.White), 1); err != nil {
		t.Fatal(err)
	}

	if out := p.Delete(1); out != "" {
		t.Errorf("Delete = %q, want empty (nothing to erase)", out)
	}
	if got := p.Placeholder(2, 1); strings.Contains(got, "▀") {
		t.Errorf("Placeholder still shows art after Delete: %q", got)
	}
}

// Committing a new track prepares the new image before deleting the old one,
// so the delete must not wipe the art that just replaced it.
func TestBlockProtocol_DeleteOfPreviousImageKeepsCurrent(t *testing.T) {
	p := trueColorBlocks()
	if _, err := p.Prepare(solid(2, 2, color.White), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Prepare(solid(2, 2, color.RGBA{G: 255, A: 255}), 2); err != nil {
		t.Fatal(err)
	}

	p.Delete(1)

	if got := p.Placeholder(2, 1); !strings.Contains(got, "▀") {
		t.Errorf("current art was dropped with the previous image: %q", got)
	}
}

// One cell is one pixel wide and two pixels tall.
func TestBlockProtocol_TargetPixelSize(t *testing.T) {
	w, h := trueColorBlocks().TargetPixelSize(20, 10)

	if w != 20 || h != 20 {
		t.Errorf("TargetPixelSize(20, 10) = (%d, %d), want (20, 20)", w, h)
	}
}

func TestBlockProtocol_PrepareFromPNG(t *testing.T) {
	p := trueColorBlocks()
	data := renderTestPNG(t, solid(2, 4, color.RGBA{G: 255, A: 255}))

	if _, err := p.PrepareFromPNG(data, 7); err != nil {
		t.Fatal(err)
	}

	if got := p.Placeholder(2, 2); !strings.Contains(got, "38;2;0;255;0") {
		t.Errorf("PrepareFromPNG produced %q", got)
	}
}
