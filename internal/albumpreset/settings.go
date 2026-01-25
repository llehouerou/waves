package albumpreset

// SortCriterion combines a field with its order.
type SortCriterion struct {
	Field SortField
	Order SortOrder
}

// Settings holds the album view configuration.
type Settings struct {
	GroupFields    []GroupField    // Multi-layer grouping (order matters), empty = no grouping
	GroupSortOrder SortOrder       // Asc/Desc for group ordering (by grouping field value)
	GroupDateField DateFieldType   // Which date to use for date grouping (Year/Month/Week)
	SortCriteria   []SortCriterion // Multi-field sorting for albums within groups
}

// DefaultSettings returns the default album view settings.
// Matches the "Newly added" preset: grouped by month (added date), sorted by added date.
func DefaultSettings() Settings {
	return Settings{
		GroupFields:    []GroupField{GroupFieldMonth},
		GroupSortOrder: SortDesc,
		GroupDateField: DateFieldAdded,
		SortCriteria: []SortCriterion{
			{Field: SortFieldAddedAt, Order: SortDesc},
		},
	}
}
