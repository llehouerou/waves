// Package albumpreset defines album view configuration types shared between
// persistence and UI layers.
package albumpreset

// GroupField represents a single grouping field for multi-layer grouping.
type GroupField int

const (
	GroupFieldArtist  GroupField = iota // Album Artist
	GroupFieldGenre                     // Genre
	GroupFieldLabel                     // Label/Publisher
	GroupFieldYear                      // Year from BestDate
	GroupFieldMonth                     // Month from BestDate
	GroupFieldWeek                      // Week from BestDate
	GroupFieldAddedAt                   // When added (Today, This Week, etc.)
)

// GroupFieldCount is the total number of group fields.
const GroupFieldCount = 7

// SortField represents a single sort field for multi-field sorting.
type SortField int

const (
	SortFieldOriginalDate SortField = iota
	SortFieldReleaseDate
	SortFieldAddedAt
	SortFieldArtist
	SortFieldAlbum
	SortFieldTrackCount
	SortFieldLabel
)

// SortFieldCount is the total number of sort fields.
const SortFieldCount = 7

// SortOrder specifies ascending or descending.
type SortOrder int

const (
	SortDesc SortOrder = iota // Newest first (default)
	SortAsc                   // Oldest first
)

// DateFieldType specifies which date field to use for date-based grouping.
type DateFieldType int

const (
	DateFieldBest     DateFieldType = iota // Use BestDate (OriginalDate > ReleaseDate)
	DateFieldOriginal                      // Use OriginalDate only
	DateFieldRelease                       // Use ReleaseDate only
	DateFieldAdded                         // Use AddedAt
)

// DateFieldTypeCount is the total number of date field types.
const DateFieldTypeCount = 4
