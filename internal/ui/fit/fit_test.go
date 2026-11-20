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

// A page that follows a cursor keeps the window it had while the cursor is
// two items or more from its edges, and moves only as far as it takes.
func TestView_FollowingACursorMovesTheWindowOnlyNearItsEdge(t *testing.T) {
	v := sample(20, 12) // 6 items a page
	for _, c := range []struct{ item, from, first int }{
		{0, 0, 0}, {3, 0, 0}, {4, 0, 1}, {5, 1, 2}, {3, 2, 1}, {2, 2, 0},
		{19, 2, 14}, {19, 20, 14}, {0, 14, 0}, {10, 0, 7},
	} {
		if p := v.Following(c.item, c.from); p.First() != c.first {
			t.Errorf("item %d from %d: first %d, want %d", c.item, c.from, p.First(), c.first)
		}
	}
	if p := sample(3, 20).Following(2, 5); p.First() != 0 || p.Shown() != 3 {
		t.Errorf("a list that fits: first %d, %d shown", p.First(), p.Shown())
	}
}

// In a page too short for the margins, the cursor's item is still shown.
func TestView_FollowingInAShortPageShowsTheCursor(t *testing.T) {
	for rows := 1; rows <= 8; rows++ {
		v := sample(20, rows)
		from := 0
		for item := range 20 {
			p := v.Following(item, from)
			if item < p.First() || item >= p.First()+p.Shown() {
				t.Fatalf("%d rows: item %d not in the page from %d of %d", rows, item, p.First(), p.Shown())
			}
			from = p.First()
		}
		for item := 19; item >= 0; item-- {
			p := v.Following(item, from)
			if item < p.First() || item >= p.First()+p.Shown() {
				t.Fatalf("%d rows, back: item %d not in the page from %d of %d", rows, item, p.First(), p.Shown())
			}
			from = p.First()
		}
	}
}

// A head is drawn above its item when a page starts with it, and counted.
func TestView_HeadIsDrawnAboveTheFirstItemOfAPage(t *testing.T) {
	v := View{Items: [][]string{{"-- Monday", "a"}, {"b"}, {"c"}, {"-- Tuesday", "d"}}, Rows: 4}
	v.Heads = [][]string{nil, {"-- Monday"}, {"-- Monday"}, nil}
	assertLines(t, v.At(0), "-- Monday", "a", "b", "(1-2 of 4)")
	assertLines(t, v.At(1), "-- Monday", "b", "c", "(2-3 of 4)")
	assertLines(t, v.At(2), "-- Monday", "c", "", "(3-3 of 4)")
	if p := v.Showing(3); p.First() != 3 {
		t.Errorf("showing d: from %d, want 3", p.First())
	}
	if p := v.At(9); p.First() != 3 {
		t.Errorf("last page from %d, want 3", p.First())
	}
	v.Rows = 6
	assertLines(t, v.At(0), "-- Monday", "a", "b", "c", "-- Tuesday", "d")
}

func TestWidest_IsTheFirstTextThatFits(t *testing.T) {
	if got := Widest(10, "0123456789ab", "0123456789", "0"); got != "0123456789" {
		t.Errorf("got %q", got)
	}
	if got := Widest(2, "abc", "abcd"); got != "abcd" {
		t.Errorf("none fits: %q, want the last", got)
	}
}

func TestPage_StepMovesACursorByTheScrollKeys(t *testing.T) {
	p := sample(20, 12).At(0) // 6 items a page
	for _, c := range []struct {
		key      string
		from, to int
	}{
		{"j", 0, 1}, {"down", 19, 19}, {"k", 0, 0}, {"up", 5, 4},
		{"ctrl+d", 5, 8}, {"ctrl+u", 2, 0}, {"G", 3, 19}, {"end", 0, 19},
		{"g", 12, 0}, {"home", 12, 0}, {"enter", 7, 7},
	} {
		if got := p.Step(c.key, c.from, 20); got != c.to {
			t.Errorf("%s from %d: %d, want %d", c.key, c.from, got, c.to)
		}
	}
}

func TestRows_CountsTheRowsALineWrapsTo(t *testing.T) {
	if got := Rows(strings.Repeat("long ", 12), 40); got != 2 {
		t.Errorf("60 columns in 40: %d rows, want 2", got)
	}
	if got := Rows(strings.Repeat("x", 60), 0); got != 1 {
		t.Errorf("no width: %d rows, want 1", got)
	}
}
