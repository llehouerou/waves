package rename

import (
	"reflect"
	"testing"
)

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
