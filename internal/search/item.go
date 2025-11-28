package search

// Item represents a searchable item.
type Item interface {
	// FilterValue returns the string to match against.
	FilterValue() string
	// DisplayText returns the string to display in results.
	DisplayText() string
}

// items wraps a slice of Item for fuzzy matching.
type items []Item

func (it items) String(i int) string {
	return it[i].FilterValue()
}

func (it items) Len() int {
	return len(it)
}
