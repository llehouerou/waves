package albumview

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/llehouerou/waves/internal/library"
)

const (
	unknownGroupKey    = "unknown"
	unknownGroupHeader = "Unknown"
)

// Refresh reloads albums from the library and rebuilds groups.
func (m *Model) Refresh() error {
	albums, err := m.lib.AllAlbums()
	if err != nil {
		return err
	}

	m.groups = m.groupAlbums(albums)
	m.flatList = m.buildFlatList()
	m.ensureCursorInBounds()
	return nil
}

// groupAlbums groups albums according to current settings.
func (m *Model) groupAlbums(albums []library.AlbumEntry) []Group {
	switch m.settings.GroupBy {
	case GroupByWeek:
		return m.groupByWeek(albums)
	case GroupByMonth:
		return m.groupByMonth(albums)
	case GroupByYear:
		return m.groupByYear(albums)
	case GroupByArtist:
		return m.groupByArtist(albums)
	case GroupByGenre:
		return m.groupByGenre(albums)
	case GroupByAddedAt:
		return m.groupByAddedAt(albums)
	case GroupByNone:
		return []Group{{Header: "", Albums: albums}}
	}
	return nil
}

// groupByWeek groups albums by week of original_date.
// Albums with year-only dates get their own group "2024" instead of being
// placed into a specific week.
func (m *Model) groupByWeek(albums []library.AlbumEntry) []Group {
	groups := make(map[string]*Group)
	var groupOrder []string

	for i := range albums {
		album := albums[i]
		date := album.BestDate()

		precision := library.ParseDatePrecision(date)
		var key, header string

		switch precision {
		case library.PrecisionNone:
			// No date - "Unknown" group
			key = unknownGroupKey
			header = unknownGroupHeader
		case library.PrecisionDay, library.PrecisionMonth:
			// Full date or month - group by week
			t, _ := library.ParseDate(date)
			year, week := t.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", year, week)

			// Calculate week range for header
			start := weekStart(t)
			end := start.AddDate(0, 0, 6)
			header = formatWeekRange(start, end)

		case library.PrecisionYear:
			// Year only - special group
			key = date + "-yearonly"
			header = date
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &Group{Header: header}
			groupOrder = append(groupOrder, key)
		}
		groups[key].Albums = append(groups[key].Albums, album)
	}

	// Convert to slice in order
	result := make([]Group, 0, len(groupOrder))
	for _, key := range groupOrder {
		result = append(result, *groups[key])
	}
	return result
}

// groupByMonth groups albums by month of original_date.
func (m *Model) groupByMonth(albums []library.AlbumEntry) []Group {
	groups := make(map[string]*Group)
	var groupOrder []string

	for i := range albums {
		album := albums[i]
		date := album.BestDate()

		precision := library.ParseDatePrecision(date)
		var key, header string

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

		if _, ok := groups[key]; !ok {
			groups[key] = &Group{Header: header}
			groupOrder = append(groupOrder, key)
		}
		groups[key].Albums = append(groups[key].Albums, album)
	}

	result := make([]Group, 0, len(groupOrder))
	for _, key := range groupOrder {
		result = append(result, *groups[key])
	}
	return result
}

// groupByYear groups albums by year.
func (m *Model) groupByYear(albums []library.AlbumEntry) []Group {
	groups := make(map[string]*Group)
	var groupOrder []string

	for i := range albums {
		album := albums[i]
		year := album.Year()

		var key, header string
		if year > 0 {
			key = strconv.Itoa(year)
			header = key
		} else {
			key = unknownGroupKey
			header = unknownGroupHeader
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &Group{Header: header}
			groupOrder = append(groupOrder, key)
		}
		groups[key].Albums = append(groups[key].Albums, album)
	}

	result := make([]Group, 0, len(groupOrder))
	for _, key := range groupOrder {
		result = append(result, *groups[key])
	}
	return result
}

// groupByArtist groups albums by album artist.
func (m *Model) groupByArtist(albums []library.AlbumEntry) []Group {
	groups := make(map[string]*Group)
	var groupOrder []string

	for i := range albums {
		album := albums[i]
		key := album.AlbumArtist
		if key == "" {
			key = "Unknown Artist"
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &Group{Header: key}
			groupOrder = append(groupOrder, key)
		}
		groups[key].Albums = append(groups[key].Albums, album)
	}

	// Sort artist groups alphabetically
	sort.Strings(groupOrder)

	result := make([]Group, 0, len(groupOrder))
	for _, key := range groupOrder {
		result = append(result, *groups[key])
	}
	return result
}

// groupByGenre groups albums by genre.
func (m *Model) groupByGenre(albums []library.AlbumEntry) []Group {
	groups := make(map[string]*Group)
	var groupOrder []string

	for i := range albums {
		album := albums[i]
		key := album.Genre
		if key == "" {
			key = "Unknown Genre"
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &Group{Header: key}
			groupOrder = append(groupOrder, key)
		}
		groups[key].Albums = append(groups[key].Albums, album)
	}

	// Sort genre groups alphabetically
	sort.Strings(groupOrder)

	result := make([]Group, 0, len(groupOrder))
	for _, key := range groupOrder {
		result = append(result, *groups[key])
	}
	return result
}

// groupByAddedAt groups albums by when they were added to the library.
func (m *Model) groupByAddedAt(albums []library.AlbumEntry) []Group {
	groups := make(map[string]*Group)
	var groupOrder []string

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	thisWeekStart := today.AddDate(0, 0, -int(today.Weekday()))
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := thisMonthStart.AddDate(0, -1, 0)

	for i := range albums {
		album := albums[i]
		addedAt := album.AddedAt

		var key, header string
		switch {
		case addedAt.After(today) || addedAt.Equal(today):
			key = "today"
			header = "Today"
		case addedAt.After(thisWeekStart) || addedAt.Equal(thisWeekStart):
			key = "this-week"
			header = "This Week"
		case addedAt.After(lastWeekStart) || addedAt.Equal(lastWeekStart):
			key = "last-week"
			header = "Last Week"
		case addedAt.After(thisMonthStart) || addedAt.Equal(thisMonthStart):
			key = "this-month"
			header = "This Month"
		case addedAt.After(lastMonthStart) || addedAt.Equal(lastMonthStart):
			key = "last-month"
			header = "Last Month"
		default:
			key = addedAt.Format("2006-01")
			header = addedAt.Format("January 2006")
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &Group{Header: header}
			groupOrder = append(groupOrder, key)
		}
		groups[key].Albums = append(groups[key].Albums, album)
	}

	result := make([]Group, 0, len(groupOrder))
	for _, key := range groupOrder {
		result = append(result, *groups[key])
	}
	return result
}

// buildFlatList creates a flattened list for cursor navigation.
func (m *Model) buildFlatList() []AlbumItem {
	var items []AlbumItem
	for i := range m.groups {
		group := &m.groups[i]
		if group.Header != "" {
			items = append(items, AlbumItem{IsHeader: true, Header: group.Header})
		}
		for j := range group.Albums {
			items = append(items, AlbumItem{IsHeader: false, Album: &group.Albums[j]})
		}
	}
	return items
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
