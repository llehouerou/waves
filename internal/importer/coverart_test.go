package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCoverArt(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR")
	tests := []struct {
		name     string
		existing string // an image already in the album folder, "" for none
		want     string // the cover file expected afterwards
	}{
		{"writes a PNG as cover.png", "", "cover.png"},
		{"keeps the album's own folder art", "Folder.JPG", ""},
		{"keeps an existing cover", "cover.jpeg", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.existing != "" {
				if err := os.WriteFile(filepath.Join(dir, tt.existing), []byte("mine"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := WriteCoverArt(dir, png); err != nil {
				t.Fatal(err)
			}

			entries, _ := os.ReadDir(dir)
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			wantCount := 1 // the existing image or the new cover, never both
			if len(names) != wantCount || (tt.want != "" && names[0] != tt.want) {
				t.Errorf("album folder holds %v, want only %q", names, tt.want+tt.existing)
			}
		})
	}
}
