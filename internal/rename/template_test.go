package rename

import (
	"reflect"
	"testing"
)

func TestResolvePlaceholder(t *testing.T) {
	meta := TrackMetadata{
		Artist:       "Pink Floyd",
		AlbumArtist:  "Pink Floyd",
		Album:        "The Dark Side of the Moon",
		Title:        "Time",
		TrackNumber:  4,
		DiscNumber:   1,
		TotalDiscs:   1,
		Date:         "1973-03-01",
		OriginalDate: "1973-03-01",
	}
	cfg := DefaultConfig()

	tests := []struct {
		name        string
		placeholder string
		want        string
	}{
		{"artist", "artist", "Pink Floyd"},
		{"albumartist", "albumartist", "Pink Floyd"},
		{"album", "album", "The Dark Side of the Moon"},
		{"title", "title", "Time"},
		{"year", "year", "1973"},
		{"tracknumber", "tracknumber", "04"},
		{"discnumber", "discnumber", "1"},
		{"date", "date", "1973-03-01"},
		{"originalyear", "originalyear", "1973"},
		{"unknown", "unknown", "{unknown}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePlaceholder(tt.placeholder, meta, cfg)
			if got != tt.want {
				t.Errorf("resolvePlaceholder(%q) = %q, want %q", tt.placeholder, got, tt.want)
			}
		})
	}
}

func TestResolvePlaceholderMultiDisc(t *testing.T) {
	meta := TrackMetadata{
		Artist:      "The Beatles",
		AlbumArtist: "The Beatles",
		Album:       "The White Album",
		Title:       "Back in the U.S.S.R.",
		TrackNumber: 1,
		DiscNumber:  1,
		TotalDiscs:  2,
		Date:        "1968",
	}
	cfg := DefaultConfig()

	got := resolvePlaceholder("tracknumber", meta, cfg)
	if got != "01.01" {
		t.Errorf("tracknumber for multi-disc = %q, want %q", got, "01.01")
	}
}

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []segment
	}{
		{
			name:     "simple placeholder",
			template: "{artist}",
			want:     []segment{{isPlaceholder: true, value: "artist"}},
		},
		{
			name:     "literal only",
			template: "Music",
			want:     []segment{{isPlaceholder: false, value: "Music"}},
		},
		{
			name:     "mixed",
			template: "{artist} - {album}",
			want: []segment{
				{isPlaceholder: true, value: "artist"},
				{isPlaceholder: false, value: " - "},
				{isPlaceholder: true, value: "album"},
			},
		},
		{
			name:     "folder template",
			template: "{albumartist}/{year} • {album}",
			want: []segment{
				{isPlaceholder: true, value: "albumartist"},
				{isPlaceholder: false, value: "/"},
				{isPlaceholder: true, value: "year"},
				{isPlaceholder: false, value: " • "},
				{isPlaceholder: true, value: "album"},
			},
		},
		{
			name:     "escaped braces",
			template: "{{literal}}",
			want:     []segment{{isPlaceholder: false, value: "{literal}"}},
		},
		{
			name:     "empty template",
			template: "",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplate(tt.template)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTemplate(%q) = %v, want %v", tt.template, got, tt.want)
			}
		})
	}
}
