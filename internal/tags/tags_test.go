package tags

import "testing"

func TestTag_Year(t *testing.T) {
	tests := []struct {
		name string
		date string
		want int
	}{
		{"empty", "", 0},
		{"year only", "2023", 2023},
		{"full date", "2023-06-15", 2023},
		{"partial date", "2023-06", 2023},
		{"invalid", "invalid", 0},
		{"short", "23", 23},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := &Tag{Date: tt.date}
			if got := tag.Year(); got != tt.want {
				t.Errorf("Year() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTag_OriginalYear(t *testing.T) {
	tests := []struct {
		name         string
		originalDate string
		want         string
	}{
		{"empty", "", ""},
		{"year only", "1999", "1999"},
		{"full date", "1999-12-31", "1999"},
		{"partial date", "1999-12", "1999"},
		{"short", "99", "99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := &Tag{OriginalDate: tt.originalDate}
			if got := tag.OriginalYear(); got != tt.want {
				t.Errorf("OriginalYear() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsMusicFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"song.mp3", true},
		{"song.MP3", true},
		{"song.flac", true},
		{"song.FLAC", true},
		{"song.opus", true},
		{"song.ogg", true},
		{"song.oga", true},
		{"song.OGA", true},
		{"song.m4a", true},
		{"song.mp4", true},
		{"song.wav", false},
		{"song.txt", false},
		{"song", false},
		{"/path/to/music.flac", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsMusicFile(tt.path); got != tt.want {
				t.Errorf("IsMusicFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
