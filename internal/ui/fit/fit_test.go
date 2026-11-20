package fit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/ui/testutil"
)

// numbered is a list of n items of one line each: "item 1" to "item n".
func numbered(n int) [][]string {
	items := make([][]string, n)
	for i := range items {
		items[i] = []string{fmt.Sprintf("item %d", i+1)}
	}
	return items
}

// sample is a view of n items between a title and a help line, each set
// apart by a blank line, in a box 40 columns wide that many rows tall.
func sample(n, rows int) View {
	return View{
		Above: append(Lines("Title", ""), Keep("Pick one:")),
		Items: numbered(n),
		Below: append(Lines(""), Keep("[Enter] Go")),
		Keys:  "[j/k] Scroll",
		Width: 40,
		Rows:  rows,
	}
}

func lines(p Page) []string {
	return strings.Split(testutil.StripANSI(p.String()), "\n")
}

func assertLines(t *testing.T, p Page, want ...string) {
	t.Helper()
	got := lines(p)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("page:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestView_ThatFitsIsDrawnWholeWithNoRange(t *testing.T) {
	p := sample(3, 8).At(2)
	assertLines(t, p, "Title", "", "Pick one:", "item 1", "item 2", "item 3", "", "[Enter] Go")
	if p.first != 0 || p.shown != 3 || p.lastPage != 0 {
		t.Errorf("page = %+v, want the 3 items from the first, no last page", p)
	}
}

func TestView_ListThatDoesNotFitShowsAPageAndItsRange(t *testing.T) {
	v := sample(10, 9)
	assertLines(t, v.At(0), "Title", "", "Pick one:", "item 1", "item 2", "item 3", "(1-3 of 10)  [j/k] Scroll", "", "[Enter] Go")
	// Past the last page, the last page.
	p := v.At(20)
	assertLines(t, p, "Title", "", "Pick one:", "item 8", "item 9", "item 10", "(8-10 of 10)  [j/k] Scroll", "", "[Enter] Go")
	if p.first != 7 || p.lastPage != 7 {
		t.Errorf("page = %+v, want the last page, from item 8", p)
	}
}

func TestView_ListThatScrollsByItselfNamesNoKey(t *testing.T) {
	v := sample(10, 9)
	v.Keys = ""
	if got := lines(v.At(0))[6]; got != "(1-3 of 10)" {
		t.Errorf("range = %q, want no key", got)
	}
}

// A line wider than the box takes the rows it wraps to.
func TestView_CountsTheRowsOfAWrappedLine(t *testing.T) {
	v := sample(10, 9)
	v.Items[0] = []string{strings.Repeat("long ", 12)} // 60 columns: 2 rows
	p := v.At(0)
	if p.shown != 2 {
		t.Errorf("shown %d items, want 2: the first takes 2 rows", p.shown)
	}
	v.Above[0] = Line{Text: strings.Repeat("title ", 10)} // 2 rows too
	if p := v.At(0); p.shown != 1 {
		t.Errorf("shown %d items, want 1", p.shown)
	}
}

// In a short box the lines around the list give way, blank lines first, and
// one item at least is shown.
func TestView_LinesAroundGiveWayInAShortBox(t *testing.T) {
	assertLines(t, sample(10, 5).At(0), "Title", "Pick one:", "item 1", "(1-1 of 10)  [j/k] Scroll", "[Enter] Go")
	assertLines(t, sample(10, 4).At(0), "Pick one:", "item 1", "(1-1 of 10)  [j/k] Scroll", "[Enter] Go")
	// The kept lines and the range never go.
	assertLines(t, sample(10, 1).At(3), "Pick one:", "item 4", "(4-4 of 10)  [j/k] Scroll", "[Enter] Go")
}

// A single item that does not fit even once every line that may give way
// did is shown with no range: it would tell nothing.
func TestView_SingleItemThatDoesNotFitHasNoRange(t *testing.T) {
	assertLines(t, sample(1, 1).At(0), "Pick one:", "item 1", "[Enter] Go")
}

// Items of different heights leave the rows they do not fill blank: the view
// keeps its height from a page to the next.
func TestView_PageKeepsItsHeightWithItemsOfDifferentHeights(t *testing.T) {
	v := sample(10, 9) // 3 rows of list
	v.Items[1] = []string{"item 2", "more of item 2"}
	v.Items[2] = []string{"item 3", "more of item 3"}
	assertLines(t, v.At(0), "Title", "", "Pick one:", "item 1", "item 2", "more of item 2", "(1-2 of 10)  [j/k] Scroll", "", "[Enter] Go")
	assertLines(t, v.At(1), "Title", "", "Pick one:", "item 2", "more of item 2", "", "(2-2 of 10)  [j/k] Scroll", "", "[Enter] Go")
}

// When the lines that give way leave room for every item, there is no range.
func TestView_ThatFitsOnceLinesGaveWayHasNoRange(t *testing.T) {
	assertLines(t, sample(1, 4).At(0), "Title", "Pick one:", "item 1", "[Enter] Go")
}

func TestView_WithNoListGivesWayToo(t *testing.T) {
	v := sample(0, 3)
	assertLines(t, v.At(0), "Title", "Pick one:", "[Enter] Go")
}

// Showing an item places the page so that it holds it, as high as it can.
func TestView_ShowingAnItemHoldsIt(t *testing.T) {
	v := sample(10, 9) // 3 items a page
	for _, c := range []struct{ item, first int }{{0, 0}, {2, 0}, {3, 1}, {9, 7}, {20, 7}} {
		if p := v.Showing(c.item); p.first != c.first {
			t.Errorf("showing item %d: first %d, want %d", c.item, p.first, c.first)
		}
	}
	if p := sample(3, 20).Showing(2); p.first != 0 || p.shown != 3 {
		t.Errorf("a list that fits: %+v", p)
	}
}

func TestPage_ScrollKeys(t *testing.T) {
	v := sample(10, 12) // 6 items a page, the last page from item 5
	for _, c := range []struct {
		from int
		key  string
		to   int
	}{
		{0, "j", 1}, {0, "down", 1}, {3, "k", 2}, {3, "up", 2}, {0, "k", 0},
		{0, "ctrl+d", 3}, {3, "ctrl+d", 4}, {4, "ctrl+u", 1}, {1, "ctrl+u", 0},
		{0, "G", 4}, {0, "end", 4}, {4, "g", 0}, {4, "home", 0}, {4, "j", 4},
		{9, "k", 3}, // from past the last page, from the last page
	} {
		if !Scrolls(c.key) {
			t.Errorf("%s does not scroll", c.key)
		}
		if got := v.At(c.from).Scroll(c.key); got != c.to {
			t.Errorf("%s from %d: %d, want %d", c.key, c.from, got, c.to)
		}
	}
	if Scrolls("enter") {
		t.Error("enter scrolls")
	}
}
