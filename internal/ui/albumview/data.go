package albumview

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/llehouerou/waves/internal/library"
)

const (
	unknownGroupKey    = "unknown"
	unknownGroupHeader = "Unknown"
)

// NestedGroup represents a group that may contain sub-groups.
type NestedGroup struct {
	Key       string
	Header    string
	Level     int
	Albums    []library.AlbumEntry
	SubGroups []NestedGroup
}

// Refresh reloads albums from the library and rebuilds groups.
func (m *Model) Refresh() error {
	albums, err := m.lib.AllAlbums()
	if err != nil {
		return err
	}

	// Apply multi-field sorting first
	m.sortAlbums(albums)

	// Then group with multi-layer support
	nestedGroups := m.groupAlbumsMultiLevel(albums)
	m.flatList = m.buildFlatListFromNested(nestedGroups)
	m.ensureCursorInBounds()
	return nil
}

// sortAlbums sorts albums according to multiple sort criteria.
func (m *Model) sortAlbums(albums []library.AlbumEntry) {
	if len(m.settings.SortCriteria) == 0 {
		return
	}

	sort.SliceStable(albums, func(i, j int) bool {
		for _, criterion := range m.settings.SortCriteria {
			cmp := m.compareByField(albums[i], albums[j], criterion.Field)
			if cmp != 0 {
				if criterion.Order == SortAsc {
					return cmp < 0
				}
				return cmp > 0
			}
		}
		return false // Equal
	})
}

// compareByField compares two albums by a single field.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func (m *Model) compareByField(a, b library.AlbumEntry, field SortField) int {
	switch field {
	case SortFieldOriginalDate:
		return strings.Compare(a.OriginalDate, b.OriginalDate)
	case SortFieldReleaseDate:
		return strings.Compare(a.ReleaseDate, b.ReleaseDate)
	case SortFieldAddedAt:
		return a.AddedAt.Compare(b.AddedAt)
	case SortFieldArtist:
		return strings.Compare(
			strings.ToLower(a.AlbumArtist),
			strings.ToLower(b.AlbumArtist),
		)
	case SortFieldAlbum:
		return strings.Compare(
			strings.ToLower(a.Album),
			strings.ToLower(b.Album),
		)
	case SortFieldTrackCount:
		if a.TrackCount < b.TrackCount {
			return -1
		}
		if a.TrackCount > b.TrackCount {
			return 1
		}
		return 0
	case SortFieldLabel:
		return strings.Compare(
			strings.ToLower(a.Label),
			strings.ToLower(b.Label),
		)
	default:
		return 0
	}
}

// groupAlbumsMultiLevel groups albums according to multiple grouping levels.
func (m *Model) groupAlbumsMultiLevel(albums []library.AlbumEntry) []NestedGroup {
	if len(m.settings.GroupFields) == 0 {
		// No grouping - return single group with all albums
		return []NestedGroup{{Albums: albums}}
	}

	return m.groupByField(albums, 0)
}

// groupByField recursively groups albums by the current field level.
func (m *Model) groupByField(albums []library.AlbumEntry, level int) []NestedGroup {
	if level >= len(m.settings.GroupFields) || len(albums) == 0 {
		return nil
	}

	field := m.settings.GroupFields[level]

	// Group albums by key
	groups := make(map[string][]library.AlbumEntry)
	keyOrder := make([]string, 0)
	keyToHeader := make(map[string]string)

	for i := range albums {
		album := albums[i]
		key, header := m.groupKeyAndHeader(album, field)

		if _, exists := groups[key]; !exists {
			keyOrder = append(keyOrder, key)
			keyToHeader[key] = header
		}
		groups[key] = append(groups[key], album)
	}

	// Sort groups by their natural order (asc/desc)
	m.sortGroupKeys(keyOrder)

	// Build nested groups
	result := make([]NestedGroup, 0, len(keyOrder))
	for _, key := range keyOrder {
		albumsInGroup := groups[key]
		header := keyToHeader[key]

		ng := NestedGroup{
			Key:    key,
			Header: header,
			Level:  level,
		}

		if level < len(m.settings.GroupFields)-1 {
			// More grouping levels - recurse
			ng.SubGroups = m.groupByField(albumsInGroup, level+1)
		} else {
			// Final level - attach albums
			ng.Albums = albumsInGroup
		}

		result = append(result, ng)
	}

	return result
}

// groupKeyAndHeader returns the grouping key and display header for an album.
func (m *Model) groupKeyAndHeader(album library.AlbumEntry, field GroupField) (key, header string) {
	switch field {
	case GroupFieldArtist:
		key = album.AlbumArtist
		if key == "" {
			key = "Unknown Artist"
		}
		header = key

	case GroupFieldGenre:
		key = album.Genre
		if key == "" {
			key = "Unknown Genre"
		}
		header = key

	case GroupFieldLabel:
		key = album.Label
		if key == "" {
			key = "Unknown Label"
		}
		header = key

	case GroupFieldYear:
		date := m.getGroupDate(album)
		if m.settings.GroupDateField == DateFieldAdded {
			// For AddedAt, use the year directly
			key = strconv.Itoa(album.AddedAt.Year())
			header = key
		} else {
			precision := library.ParseDatePrecision(date)
			if precision == library.PrecisionNone {
				key = unknownGroupKey
				header = unknownGroupHeader
			} else {
				t, _ := library.ParseDate(date)
				key = strconv.Itoa(t.Year())
				header = key
			}
		}

	case GroupFieldMonth:
		date := m.getGroupDate(album)
		if m.settings.GroupDateField == DateFieldAdded {
			t := album.AddedAt
			key = fmt.Sprintf("%d-%02d", t.Year(), t.Month())
			header = t.Format("January 2006")
		} else {
			precision := library.ParseDatePrecision(date)
			switch precision {
			case library.PrecisionNone:
				key = unknownGroupKey
				header = unknownGroupHeader
			case library.PrecisionDay, library.PrecisionMonth:
				t, _ := library.ParseDate(date)
				key = fmt.Sprintf("%d-%02d", t.Year(), t.Month())
				header = t.Format("January 2006")
			case library.PrecisionYear:
				key = date + "-yearonly"
				header = date
			}
		}

	case GroupFieldWeek:
		date := m.getGroupDate(album)
		if m.settings.GroupDateField == DateFieldAdded {
			t := album.AddedAt
			year, week := t.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", year, week)
			start := weekStart(t)
			end := start.AddDate(0, 0, 6)
			header = formatWeekRange(start, end)
		} else {
			precision := library.ParseDatePrecision(date)
			switch precision {
			case library.PrecisionNone:
				key = unknownGroupKey
				header = unknownGroupHeader
			case library.PrecisionDay, library.PrecisionMonth:
				t, _ := library.ParseDate(date)
				year, week := t.ISOWeek()
				key = fmt.Sprintf("%d-W%02d", year, week)
				start := weekStart(t)
				end := start.AddDate(0, 0, 6)
				header = formatWeekRange(start, end)
			case library.PrecisionYear:
				key = date + "-yearonly"
				header = date
			}
		}

	case GroupFieldAddedAt:
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		thisWeekStart := today.AddDate(0, 0, -int(today.Weekday()))
		lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastMonthStart := thisMonthStart.AddDate(0, -1, 0)

		addedAt := album.AddedAt
		switch {
		case addedAt.After(today) || addedAt.Equal(today):
			key = "0-today"
			header = "Today"
		case addedAt.After(thisWeekStart) || addedAt.Equal(thisWeekStart):
			key = "1-this-week"
			header = "This Week"
		case addedAt.After(lastWeekStart) || addedAt.Equal(lastWeekStart):
			key = "2-last-week"
			header = "Last Week"
		case addedAt.After(thisMonthStart) || addedAt.Equal(thisMonthStart):
			key = "3-this-month"
			header = "This Month"
		case addedAt.After(lastMonthStart) || addedAt.Equal(lastMonthStart):
			key = "4-last-month"
			header = "Last Month"
		default:
			key = "5-" + addedAt.Format("2006-01")
			header = addedAt.Format("January 2006")
		}
	}

	return key, header
}

// sortGroupKeys sorts the keys by their natural order (alphabetical/chronological).
// GroupSortOrder determines ascending or descending. Unknown groups always sort last.
func (m *Model) sortGroupKeys(keys []string) {
	// Keys are designed to sort correctly as strings:
	// - Year: "2024", "2023"
	// - Month: "2024-12", "2024-11"
	// - Week: "2024-W51", "2024-W50"
	// - Artist/Genre/Label: alphabetical
	// - AddedAt: "0-today", "1-this-week", etc.
	// - Unknown: always last
	sort.Slice(keys, func(i, j int) bool {
		// Unknown groups always sort last
		if keys[i] == unknownGroupKey {
			return false
		}
		if keys[j] == unknownGroupKey {
			return true
		}

		if m.settings.GroupSortOrder == SortAsc {
			return keys[i] < keys[j]
		}
		return keys[i] > keys[j]
	})
}

// buildFlatListFromNested flattens nested groups into a single list.
func (m *Model) buildFlatListFromNested(groups []NestedGroup) []AlbumItem {
	var items []AlbumItem
	m.flattenNestedGroups(groups, &items)
	return items
}

func (m *Model) flattenNestedGroups(groups []NestedGroup, items *[]AlbumItem) {
	for i := range groups {
		g := &groups[i]
		if g.Header != "" {
			*items = append(*items, AlbumItem{
				IsHeader:    true,
				Header:      g.Header,
				HeaderLevel: g.Level,
			})
		}
		if len(g.SubGroups) > 0 {
			m.flattenNestedGroups(g.SubGroups, items)
		}
		for j := range g.Albums {
			*items = append(*items, AlbumItem{
				IsHeader: false,
				Album:    &g.Albums[j],
			})
		}
	}
}

// getGroupDate returns the date string to use for date-based grouping.
func (m *Model) getGroupDate(album library.AlbumEntry) string {
	switch m.settings.GroupDateField {
	case DateFieldBest:
		return album.BestDate()
	case DateFieldOriginal:
		return album.OriginalDate
	case DateFieldRelease:
		return album.ReleaseDate
	case DateFieldAdded:
		// AddedAt is handled specially in groupKeyAndHeader
		return ""
	default:
		return album.BestDate()
	}
}

// weekStart returns the Monday of the week containing t.
func weekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

// formatWeekRange formats a date range like "Dec 9 - Dec 15, 2024"
func formatWeekRange(start, end time.Time) string {
	if start.Month() == end.Month() {
		return fmt.Sprintf("%s %d - %d, %d",
			start.Format("Jan"), start.Day(), end.Day(), start.Year())
	}
	if start.Year() == end.Year() {
		return fmt.Sprintf("%s %d - %s %d, %d",
			start.Format("Jan"), start.Day(),
			end.Format("Jan"), end.Day(), start.Year())
	}
	return fmt.Sprintf("%s %d, %d - %s %d, %d",
		start.Format("Jan"), start.Day(), start.Year(),
		end.Format("Jan"), end.Day(), end.Year())
}
