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

// followMargin is how many items a page that follows a cursor keeps between
// the cursor and its edges, when it has room for them.
const followMargin = 2

// Widest is the first of texts no wider than width columns: the widest of a
// line's variants that fits, given from the widest. When none fits, the last.
func Widest(width int, texts ...string) string {
	for _, text := range texts {
		if lipgloss.Width(text) <= width {
			return text
		}
	}
	if len(texts) == 0 {
		return ""
	}
	return texts[len(texts)-1]
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
	// Heads are the lines drawn above an item when a page starts with it, and
	// only then: the heading of the group a page starts in the middle of. An
	// item past the end of Heads has none.
	Heads [][]string
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

// First is the first item the page shows.
func (p Page) First() int {
	return p.first
}

// Shown is how many items the page shows.
func (p Page) Shown() int {
	return p.shown
}

// String is the page, a line per row of the box but for lines wider than it.
func (p Page) String() string {
	return strings.Join(p.lines, "\n")
}

// Scroll is the first item shown once key scrolls the page: j/k and
// down/up by one item, ctrl+d/ctrl+u by half a page, g/home and G/end to the
// first and the last page. A key Scrolls does not take leaves it where it is.
func (p Page) Scroll(key string) int {
	return p.move(key, p.first, p.lastPage)
}

// Step is the item the cursor of a list moves to from item cursor once key
// moves it, on a list of total items: by the keys Scroll takes, but G/end
// move it to the last item.
func (p Page) Step(key string, cursor, total int) int {
	return p.move(key, cursor, max(total-1, 0))
}

// move is where key takes from, between 0 and last: j/k and down/up by one
// item, ctrl+d/ctrl+u by half the page, g/home to 0 and G/end to last. A key
// Scrolls does not take leaves it where it is.
func (p Page) move(key string, from, last int) int {
	half := max(p.shown/2, 1)
	to := from
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
		to = last
	}
	return min(max(to, 0), last)
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
	first, used := item, l.rows[item]+l.heads[item]
	for first > 0 && used-l.heads[first]+l.rows[first-1]+l.heads[first-1] <= l.list {
		used += l.rows[first-1] + l.heads[first-1] - l.heads[first]
		first--
	}
	return v.page(l, first)
}

// Following is the page of the view that shows item, the cursor of a list
// the user moves in, after the page that started at item first: it stays
// where it was while the cursor is followMargin items or more from its
// edges, and moves only as far as it takes to keep them, as far as the page
// has room for them.
func (v View) Following(item, first int) Page {
	l := v.layout()
	if len(l.rows) == 0 {
		return v.page(l, 0)
	}
	item = min(max(item, 0), len(l.rows)-1)
	last := l.lastPage()
	first = min(max(first, 0), last)
	margin := func(first int) int {
		return min(followMargin, (l.shownFrom(first)-1)/2)
	}
	for first > 0 && item < first+margin(first) {
		first--
	}
	for first < last && item > first+l.shownFrom(first)-1-margin(first) {
		first++
	}
	return v.page(l, first)
}

// layout is how a view is laid out in its box.
type layout struct {
	kept    []bool // of the lines around, Above then Below: those drawn
	rows    []int  // rows each item takes
	heads   []int  // rows the head of each item takes, when a page starts with it
	list    int    // rows the list takes: all its rows when it fits
	scrolls bool   // the list does not fit: it shows a page and a range
}

func (v View) layout() layout {
	around := append(append([]Line{}, v.Above...), v.Below...)
	l := layout{
		kept:  make([]bool, len(around)),
		rows:  make([]int, len(v.Items)),
		heads: make([]int, len(v.Items)),
	}
	all, tallest := 0, 0
	for i, item := range v.Items {
		for _, line := range item {
			l.rows[i] += v.rowsOf(line)
		}
		if i < len(v.Heads) {
			for _, line := range v.Heads[i] {
				l.heads[i] += v.rowsOf(line)
			}
		}
		all += l.rows[i]
		tallest = max(tallest, l.rows[i]+l.heads[i])
	}
	if len(v.Items) > 0 {
		all += l.heads[0] // the whole list starts with its first item
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
		if i < len(l.rows)-1 && used+rows+l.heads[i] > l.list {
			return i + 1
		}
		used += rows
	}
	return 0
}

// shownFrom is how many items a page that starts at item first shows.
func (l layout) shownFrom(first int) int {
	shown, used := 0, 0
	for i := first; i < len(l.rows); i++ {
		rows := l.rows[i]
		if i == first {
			rows += l.heads[i]
		}
		if shown > 0 && used+rows > l.list {
			break
		}
		used += rows
		shown++
	}
	return shown
}

// page draws the view with its list from item first.
func (v View) page(l layout, first int) Page {
	p := Page{first: first, lastPage: l.lastPage(), shown: l.shownFrom(first)}
	used := 0
	for i := first; i < first+p.shown; i++ {
		used += l.rows[i]
	}

	for i, line := range v.Above {
		if l.kept[i] {
			p.lines = append(p.lines, line.Text)
		}
	}
	if p.shown > 0 && first < len(v.Heads) {
		p.lines = append(p.lines, v.Heads[first]...)
		used += l.heads[first]
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

// rowsOf is how many rows a line takes in the box.
func (v View) rowsOf(line string) int {
	return Rows(line, v.Width)
}

// Rows is how many rows a line takes in a box width columns wide: a line
// wider than it goes on over the next rows, broken as the box breaks it. A
// box of no width breaks nothing.
func Rows(line string, width int) int {
	if width <= 0 || lipgloss.Width(line) <= width {
		return 1
	}
	return strings.Count(lipgloss.NewStyle().Width(width).Render(line), "\n") + 1
}
