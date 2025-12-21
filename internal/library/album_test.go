package library

import (
	"testing"
	"time"
)

func TestParseDatePrecision(t *testing.T) {
	tests := []struct {
		name     string
		date     string
		expected DatePrecision
	}{
		{"empty string", "", PrecisionNone},
		{"year only", "2024", PrecisionYear},
		{"year and month", "2024-05", PrecisionMonth},
		{"full date", "2024-05-15", PrecisionDay},
		{"invalid short", "24", PrecisionNone},
		{"invalid medium", "2024-5", PrecisionNone},
		{"invalid long", "2024-05-1", PrecisionNone},
		{"too long", "2024-05-15T00:00:00", PrecisionNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDatePrecision(tt.date)
			if result != tt.expected {
				t.Errorf("ParseDatePrecision(%q) = %d, want %d", tt.date, result, tt.expected)
			}
		})
	}
}

func TestParseDate_ValidDates(t *testing.T) {
	tests := []struct {
		name              string
		date              string
		expectedYear      int
		expectedMonth     time.Month
		expectedDay       int
		expectedPrecision DatePrecision
	}{
		{"year only", "2024", 2024, time.January, 1, PrecisionYear},
		{"year and month", "2024-05", 2024, time.May, 1, PrecisionMonth},
		{"full date", "2024-05-15", 2024, time.May, 15, PrecisionDay},
		{"old year", "1985", 1985, time.January, 1, PrecisionYear},
		{"december", "2020-12", 2020, time.December, 1, PrecisionMonth},
		{"leap year date", "2024-02-29", 2024, time.February, 29, PrecisionDay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, precision := ParseDate(tt.date)
			if precision != tt.expectedPrecision {
				t.Errorf("ParseDate(%q) precision = %d, want %d", tt.date, precision, tt.expectedPrecision)
			}
			if result.Year() != tt.expectedYear {
				t.Errorf("ParseDate(%q) year = %d, want %d", tt.date, result.Year(), tt.expectedYear)
			}
			if result.Month() != tt.expectedMonth {
				t.Errorf("ParseDate(%q) month = %v, want %v", tt.date, result.Month(), tt.expectedMonth)
			}
			if result.Day() != tt.expectedDay {
				t.Errorf("ParseDate(%q) day = %d, want %d", tt.date, result.Day(), tt.expectedDay)
			}
		})
	}
}

func TestParseDate_InvalidDates(t *testing.T) {
	tests := []struct {
		name string
		date string
	}{
		{"empty string", ""},
		{"invalid format", "not-a-date"},
		{"invalid year format", "20X4"},
		{"invalid month", "2024-13"},
		{"invalid day", "2024-05-32"},
		{"wrong separator", "2024/05/15"},
		{"too short", "202"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, precision := ParseDate(tt.date)
			if precision != PrecisionNone {
				t.Errorf("ParseDate(%q) precision = %d, want PrecisionNone", tt.date, precision)
			}
			if !result.IsZero() {
				t.Errorf("ParseDate(%q) returned non-zero time", tt.date)
			}
		})
	}
}

func TestAlbumEntry_BestDate(t *testing.T) {
	tests := []struct {
		name         string
		originalDate string
		releaseDate  string
		expected     string
	}{
		// Original date preferred when equal or better precision
		{"both empty", "", "", ""},
		{"only original", "2024", "", "2024"},
		{"only release", "", "2024", "2024"},
		{"same precision prefer original", "2020", "2024", "2020"},
		{"original has better precision", "2024-05-15", "2024", "2024-05-15"},
		{"original month vs release year", "2024-05", "2024", "2024-05"},

		// Release date wins when it has better precision
		{"release has better precision day", "2024", "2024-05-15", "2024-05-15"},
		{"release has better precision month", "2024", "2024-05", "2024-05"},
		{"release day vs original month", "2024-05", "2024-05-15", "2024-05-15"},

		// Edge cases
		{"original year release day", "1985", "2020-01-01", "2020-01-01"},
		{"original day release year", "1985-06-15", "2020", "1985-06-15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			album := AlbumEntry{
				OriginalDate: tt.originalDate,
				ReleaseDate:  tt.releaseDate,
			}
			result := album.BestDate()
			if result != tt.expected {
				t.Errorf("BestDate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAlbumEntry_Year(t *testing.T) {
	tests := []struct {
		name         string
		originalDate string
		releaseDate  string
		expectedYear int
	}{
		{"both empty", "", "", 0},
		{"year only", "2024", "", 2024},
		{"month precision", "2024-05", "", 2024},
		{"day precision", "2024-05-15", "", 2024},
		{"uses best date", "2020", "2024-05-15", 2024}, // Release has better precision
		{"prefers original at equal", "2020", "2024", 2020},
		{"old year", "1985", "", 1985},
		{"future year", "2030", "", 2030},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			album := AlbumEntry{
				OriginalDate: tt.originalDate,
				ReleaseDate:  tt.releaseDate,
			}
			result := album.Year()
			if result != tt.expectedYear {
				t.Errorf("Year() = %d, want %d", result, tt.expectedYear)
			}
		})
	}
}

func TestAlbumEntry_Year_InvalidDates(t *testing.T) {
	tests := []struct {
		name         string
		originalDate string
		releaseDate  string
	}{
		{"invalid original", "not-a-date", ""},
		{"invalid both", "foo", "bar"},
		{"too short", "20", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			album := AlbumEntry{
				OriginalDate: tt.originalDate,
				ReleaseDate:  tt.releaseDate,
			}
			result := album.Year()
			if result != 0 {
				t.Errorf("Year() = %d, want 0 for invalid date", result)
			}
		})
	}
}

func TestDatePrecision_Ordering(t *testing.T) {
	// Verify precision values are ordered correctly for comparisons
	if PrecisionNone >= PrecisionYear {
		t.Error("PrecisionNone should be less than PrecisionYear")
	}
	if PrecisionYear >= PrecisionMonth {
		t.Error("PrecisionYear should be less than PrecisionMonth")
	}
	if PrecisionMonth >= PrecisionDay {
		t.Error("PrecisionMonth should be less than PrecisionDay")
	}
}
