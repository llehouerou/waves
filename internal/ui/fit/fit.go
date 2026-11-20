// Package fit fits the view of a popup into the rows of its box: a list that
// does not fit scrolls, and in a short box the lines around it give way.
package fit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// ScrollKeys names the keys Page.Scroll takes, for a list scrolled by hand.
const ScrollKeys = "[j/k ctrl+d/u g/G] Scroll"

// Scrolls reports whether key is one Page.Scroll takes.
func Scrolls(key string) bool {
	switch key {
	case "j", "down", "k", "up", "ctrl+d", "ctrl+u", "g", "home", "G", "end":
		return true
	}
	return false
}

// Box is the box of a popup a view is fitted to: the popup's size, or taller
// up to the limit the popup manager gives. A popup embeds it to implement
// popup.Growing; one told no limit does not grow.
type Box struct {
	maxHeight int
}

// SetMaxHeight implements popup.Growing: the box grows up to height rows
// while the view needs them.
func (b *Box) SetMaxHeight(height int) {
	b.maxHeight = height
}

// Fit is v in the box of a popup sized width×height, drawn by
// popup.RenderBordered: as wide as its text area, and as tall as the box may
// grow.
func (b Box) Fit(v View, width, height int) View {
	v.Width = popup.TextWidth(width)
	v.Rows = popup.TextRows(max(height, b.maxHeight))
	return v
}

// Line is a line of a view around its list.
type Line struct {
	Text string
	Kept bool // never gives way: a help line, an error, a choice the user makes
}

// Lines are lines that give way in a short box: the blank ones first, then
// the others from the top of the view.
func Lines(texts ...string) []Line {
	lines := make([]Line, len(texts))
	for i, text := range texts {
		lines[i] = Line{Text: text}
	}
	return lines
}

// Keep is a line that never gives way.
func Keep(text string) Line {
	return Line{Text: text, Kept: true}
}

// View is what a view draws: lines above a list, the list, lines below it,
// in a box of Rows rows of Width columns.
//
// A view that fits is drawn whole. Otherwise its list shows a page, from an
// item on, with a range under it: "(first-last of total)", followed by Keys
// when the list has more than one page. When the lines around and one item
// do not fit, the lines around give way until they do, or until only the kept
// ones are left: the list always shows one item.
type View struct {
	Above []Line
	Items [][]string // the items of the list, each on one line or more
	Below []Line
	Keys  string // the keys that scroll the list; none for a list that scrolls by itself
	Width int    // columns of a row: a wider line takes the rows it wraps to
	Rows  int    // rows the view may take
}

// Page is what a view draws, its list shown from an item on.
type Page struct {
	lines    []string
	first    int // the first item shown
	shown    int // how many items are shown
	lastPage int // the first item shown on the last page: 0 when the list fits
}

// String is the page, a line per row of the box but for lines wider than it.
func (p Page) String() string {
	return strings.Join(p.lines, "\n")
}

// Scroll is the first item shown once key scrolls the page: j/k and
// down/up by one item, ctrl+d/ctrl+u by half a page, g/home and G/end to the
// first and the last page. A key Scrolls does not take leaves it where it is.
func (p Page) Scroll(key string) int {
	half := max(p.shown/2, 1)
	to := p.first
	switch key {
	case "j", "down":
		to++
	case "k", "up":
		to--
	case "ctrl+d":
		to += half
	case "ctrl+u":
		to -= half
	case "g", "home":
		to = 0
	case "G", "end":
		to = p.lastPage
	}
	return min(max(to, 0), p.lastPage)
}

// At is the page of the view whose list starts at item first, or at the
// first item of its last page if that comes before.
func (v View) At(first int) Page {
	l := v.layout()
	return v.page(l, min(max(first, 0), l.lastPage()))
}

// Showing is the page of the view that shows item, as close to the top of
// the list as the item lets it be.
func (v View) Showing(item int) Page {
	l := v.layout()
	if len(l.rows) == 0 {
		return v.page(l, 0)
	}
	item = min(max(item, 0), len(l.rows)-1)
	first, used := item, l.rows[item]
	for first > 0 && used+l.rows[first-1] <= l.list {
		first--
		used += l.rows[first]
	}
	return v.page(l, first)
}

// layout is how a view is laid out in its box.
type layout struct {
	kept    []bool // of the lines around, Above then Below: those drawn
	rows    []int  // rows each item takes
	list    int    // rows the list takes: all its rows when it fits
	scrolls bool   // the list does not fit: it shows a page and a range
}

func (v View) layout() layout {
	around := append(append([]Line{}, v.Above...), v.Below...)
	l := layout{kept: make([]bool, len(around)), rows: make([]int, len(v.Items))}
	all, tallest := 0, 0
	for i, item := range v.Items {
		for _, line := range item {
			l.rows[i] += v.rowsOf(line)
		}
		all += l.rows[i]
		tallest = max(tallest, l.rows[i])
	}
	rangeRows := 0
	if n := len(v.Items); n > 0 {
		rangeRows = v.rowsOf(v.rangeText(n, n, n, true))
	}

	used := 0
	for i, line := range around {
		l.kept[i] = true
		used += v.rowsOf(line.Text)
	}
	for _, i := range giveWayOrder(around) {
		if used+all <= v.Rows {
			l.list = all
			return l
		}
		if len(v.Items) > 0 && used+rangeRows+tallest <= v.Rows {
			break
		}
		l.kept[i] = false
		used -= v.rowsOf(around[i].Text)
	}
	if used+all <= v.Rows {
		l.list = all
		return l
	}
	l.list = max(v.Rows-used-rangeRows, tallest)
	// Nothing left to give way: a list shown whole anyway, one item, needs
	// no range.
	if all <= l.list {
		l.list = all
		return l
	}
	l.scrolls = true
	return l
}

// giveWayOrder is the order in which the lines around a list give way: the
// blank ones first, then the others from the top; the kept ones never.
func giveWayOrder(lines []Line) []int {
	var blanks, others []int
	for i, line := range lines {
		switch {
		case line.Kept:
		case line.Text == "":
			blanks = append(blanks, i)
		default:
			others = append(others, i)
		}
	}
	return append(blanks, others...)
}

// lastPage is the first item of the last page.
func (l layout) lastPage() int {
	used := 0
	for i, rows := range slices.Backward(l.rows) {
		if i < len(l.rows)-1 && used+rows > l.list {
			return i + 1
		}
		used += rows
	}
	return 0
}

// page draws the view with its list from item first.
func (v View) page(l layout, first int) Page {
	p := Page{first: first, lastPage: l.lastPage()}
	used := 0
	for i := first; i < len(l.rows); i++ {
		if p.shown > 0 && used+l.rows[i] > l.list {
			break
		}
		used += l.rows[i]
		p.shown++
	}

	for i, line := range v.Above {
		if l.kept[i] {
			p.lines = append(p.lines, line.Text)
		}
	}
	for _, item := range v.Items[first : first+p.shown] {
		p.lines = append(p.lines, item...)
	}
	if l.scrolls {
		// The rows of the list an item does not fill stay blank: the view
		// keeps its height from a page to the next.
		for ; used < l.list; used++ {
			p.lines = append(p.lines, "")
		}
		text := v.rangeText(first+1, first+p.shown, len(v.Items), p.lastPage > 0)
		p.lines = append(p.lines, styles.T().S().Subtle.Render(text))
	}
	for i, line := range v.Below {
		if l.kept[len(v.Above)+i] {
			p.lines = append(p.lines, line.Text)
		}
	}
	return p
}

// rangeText says which items of the list are shown, and with withKeys the
// keys that scroll it.
func (v View) rangeText(first, last, total int, withKeys bool) string {
	text := fmt.Sprintf("(%d-%d of %d)", first, last, total)
	if withKeys && v.Keys != "" {
		text += "  " + v.Keys
	}
	return text
}

// rowsOf is how many rows a line takes in the box: a line wider than it goes
// on over the next rows, broken as the box breaks it.
func (v View) rowsOf(line string) int {
	if v.Width <= 0 || lipgloss.Width(line) <= v.Width {
		return 1
	}
	return strings.Count(lipgloss.NewStyle().Width(v.Width).Render(line), "\n") + 1
}
