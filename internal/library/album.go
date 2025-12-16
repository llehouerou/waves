package library

import "time"

// AlbumEntry represents a complete album with aggregated track data.
type AlbumEntry struct {
	AlbumArtist  string
	Album        string
	OriginalDate string    // Best date from tracks (YYYY-MM-DD, YYYY-MM, or YYYY)
	ReleaseDate  string    // Best date from tracks
	AddedAt      time.Time // When first track was added to library
	TrackCount   int
	Genre        string // Most common genre from tracks
}

// DatePrecision indicates the granularity of a date string.
type DatePrecision int

const (
	PrecisionNone  DatePrecision = iota // No date or invalid
	PrecisionYear                       // "2024"
	PrecisionMonth                      // "2024-05"
	PrecisionDay                        // "2024-05-15"
)

// ParseDatePrecision returns the precision level of a date string.
func ParseDatePrecision(date string) DatePrecision {
	switch len(date) {
	case 4:
		return PrecisionYear
	case 7:
		return PrecisionMonth
	case 10:
		return PrecisionDay
	default:
		return PrecisionNone
	}
}

// ParseDate attempts to parse a date string with variable precision.
// Returns the time and precision level.
func ParseDate(date string) (time.Time, DatePrecision) {
	precision := ParseDatePrecision(date)
	var t time.Time
	var err error

	switch precision {
	case PrecisionNone:
		return time.Time{}, PrecisionNone
	case PrecisionDay:
		t, err = time.Parse("2006-01-02", date)
	case PrecisionMonth:
		t, err = time.Parse("2006-01", date)
	case PrecisionYear:
		t, err = time.Parse("2006", date)
	}

	if err != nil {
		return time.Time{}, PrecisionNone
	}
	return t, precision
}

// BestDate returns the best available date.
// Prefers the date with better precision; at equal precision, prefers original date.
func (a *AlbumEntry) BestDate() string {
	origPrecision := ParseDatePrecision(a.OriginalDate)
	relPrecision := ParseDatePrecision(a.ReleaseDate)

	// If release has better precision, use it
	if relPrecision > origPrecision {
		return a.ReleaseDate
	}
	// If original has better or equal precision (and is not empty), use it
	if a.OriginalDate != "" {
		return a.OriginalDate
	}
	return a.ReleaseDate
}

// Year returns the year from the best available date, or 0 if none.
func (a *AlbumEntry) Year() int {
	date := a.BestDate()
	if len(date) < 4 {
		return 0
	}
	t, precision := ParseDate(date)
	if precision == PrecisionNone {
		return 0
	}
	return t.Year()
}
