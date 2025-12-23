package export

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeedsConversion(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".flac", true},
		{".FLAC", true},
		{".Flac", true},
		{".mp3", false},
		{".MP3", false},
		{".wav", false},
		{".m4a", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := NeedsConversion(tt.ext)
			if got != tt.want {
				t.Errorf("NeedsConversion(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	e := NewExporter()

	t.Run("copies file successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "source.txt")
		dst := filepath.Join(tmpDir, "subdir", "dest.txt")

		// Create source file
		content := []byte("test content")
		if err := os.WriteFile(src, content, 0o600); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		// Copy file
		if err := e.CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}

		// Verify destination
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read destination: %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("content mismatch: got %q, want %q", got, content)
		}
	})

	t.Run("skips if destination exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "source.txt")
		dst := filepath.Join(tmpDir, "dest.txt")

		// Create source and destination
		if err := os.WriteFile(src, []byte("source"), 0o600); err != nil {
			t.Fatalf("failed to create source: %v", err)
		}
		if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
			t.Fatalf("failed to create destination: %v", err)
		}

		// Copy should skip
		if err := e.CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}

		// Verify destination unchanged
		got, _ := os.ReadFile(dst)
		if string(got) != "existing" {
			t.Errorf("destination was overwritten: got %q", got)
		}
	})

	t.Run("returns error for missing source", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "nonexistent.txt")
		dst := filepath.Join(tmpDir, "dest.txt")

		err := e.CopyFile(src, dst)
		if err == nil {
			t.Error("expected error for missing source")
		}
	})
}

func TestExportFile(t *testing.T) {
	e := NewExporter()

	t.Run("copies non-flac file directly", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "song.mp3")
		dst := filepath.Join(tmpDir, "output", "song.mp3")

		if err := os.WriteFile(src, []byte("mp3 data"), 0o600); err != nil {
			t.Fatalf("failed to create source: %v", err)
		}

		if err := e.ExportFile(src, dst, true); err != nil {
			t.Fatalf("ExportFile() error = %v", err)
		}

		// Verify copied to original path
		if _, err := os.Stat(dst); err != nil {
			t.Errorf("destination file not created: %v", err)
		}
	})

	t.Run("copies flac without conversion when convert=false", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "song.flac")
		dst := filepath.Join(tmpDir, "output", "song.flac")

		if err := os.WriteFile(src, []byte("flac data"), 0o600); err != nil {
			t.Fatalf("failed to create source: %v", err)
		}

		if err := e.ExportFile(src, dst, false); err != nil {
			t.Fatalf("ExportFile() error = %v", err)
		}

		// Verify copied to flac path (not mp3)
		if _, err := os.Stat(dst); err != nil {
			t.Errorf("destination file not created at flac path: %v", err)
		}
	})
}

func TestGenerateExportPath(t *testing.T) {
	tests := []struct {
		name      string
		track     TrackInfo
		structure FolderStructure
		want      string
	}{
		{
			name: "flat structure",
			track: TrackInfo{
				Artist:      "Artist",
				Album:       "Album",
				Title:       "Song",
				TrackNumber: 1,
				Extension:   ".mp3",
			},
			structure: FolderStructureFlat,
			want:      "Artist - Album/01 - Song.mp3",
		},
		{
			name: "hierarchical structure",
			track: TrackInfo{
				Artist:      "Artist",
				Album:       "Album",
				Title:       "Song",
				TrackNumber: 5,
				Extension:   ".flac",
			},
			structure: FolderStructureHierarchical,
			want:      "Artist/Album/05 - Song.flac",
		},
		{
			name: "single folder structure",
			track: TrackInfo{
				Artist:      "Artist",
				Album:       "Album",
				Title:       "Song",
				TrackNumber: 3,
				Extension:   ".mp3",
			},
			structure: FolderStructureSingle,
			want:      "Artist - Album - 03 - Song.mp3",
		},
		{
			name: "multi-disc album",
			track: TrackInfo{
				Artist:      "Artist",
				Album:       "Album",
				Title:       "Song",
				TrackNumber: 5,
				DiscNumber:  2,
				TotalDiscs:  3,
				Extension:   ".mp3",
			},
			structure: FolderStructureFlat,
			want:      "Artist - Album/2-05 - Song.mp3",
		},
		{
			name: "sanitizes illegal characters",
			track: TrackInfo{
				Artist:      "Art/ist",
				Album:       "Al:bum",
				Title:       "So?ng",
				TrackNumber: 1,
				Extension:   ".mp3",
			},
			structure: FolderStructureHierarchical,
			want:      "Art-ist/Al-bum/01 - So-ng.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateExportPath(tt.track, tt.structure)
			// Normalize path separators for cross-platform testing
			got = strings.ReplaceAll(got, "\\", "/")
			if got != tt.want {
				t.Errorf("GenerateExportPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"with/slash", "with-slash"},
		{"with\\backslash", "with-backslash"},
		{"with:colon", "with-colon"},
		{"with*asterisk", "with-asterisk"},
		{"with?question", "with-question"},
		{"with\"quote", "with-quote"},
		{"with<less", "with-less"},
		{"with>greater", "with-greater"},
		{"with|pipe", "with-pipe"},
		{"multi/char:test*file", "multi-char-test-file"},
		{strings.Repeat("a", 250), strings.Repeat("a", 200)}, // truncation
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(20, len(tt.input))], func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatTrackNumber(t *testing.T) {
	tests := []struct {
		track      int
		disc       int
		totalDiscs int
		want       string
	}{
		{1, 0, 0, "01"},
		{5, 0, 0, "05"},
		{12, 0, 0, "12"},
		{1, 1, 1, "01"},   // single disc, no disc prefix
		{5, 2, 3, "2-05"}, // multi-disc
		{1, 1, 2, "1-01"}, // multi-disc
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatTrackNumber(tt.track, tt.disc, tt.totalDiscs)
			if got != tt.want {
				t.Errorf("formatTrackNumber(%d, %d, %d) = %q, want %q",
					tt.track, tt.disc, tt.totalDiscs, got, tt.want)
			}
		})
	}
}

func TestVolumeString(t *testing.T) {
	tests := []struct {
		name   string
		volume Volume
		want   string
	}{
		{
			name:   "with label and UUID",
			volume: Volume{Label: "USB Drive", MountPath: "/media/usb", UUID: "1234-ABCD"},
			want:   "USB Drive (/media/usb) [1234-ABCD]",
		},
		{
			name:   "with UUID only",
			volume: Volume{MountPath: "/mnt/disk", UUID: "5678-EFGH"},
			want:   "/mnt/disk [5678-EFGH]",
		},
		{
			name:   "without UUID",
			volume: Volume{MountPath: "/mnt/disk"},
			want:   "/mnt/disk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.volume.String()
			if got != tt.want {
				t.Errorf("Volume.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMountLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantDevice string
		wantPath   string
		wantOk     bool
	}{
		{
			name:       "removable media in /media",
			line:       `/dev/sdb1 /media/user/USB\040Drive vfat rw 0 0`,
			wantDevice: "/dev/sdb1",
			wantPath:   "/media/user/USB Drive",
			wantOk:     true,
		},
		{
			name:       "removable media in /mnt",
			line:       "/dev/sdc1 /mnt/external ext4 rw 0 0",
			wantDevice: "/dev/sdc1",
			wantPath:   "/mnt/external",
			wantOk:     true,
		},
		{
			name:       "removable media in /run/media",
			line:       "/dev/sdd1 /run/media/user/drive ntfs rw 0 0",
			wantDevice: "/dev/sdd1",
			wantPath:   "/run/media/user/drive",
			wantOk:     true,
		},
		{
			name:   "root filesystem - not removable",
			line:   "/dev/sda1 / ext4 rw 0 0",
			wantOk: false,
		},
		{
			name:   "home filesystem - not removable",
			line:   "/dev/sda2 /home ext4 rw 0 0",
			wantOk: false,
		},
		{
			name:   "empty line",
			line:   "",
			wantOk: false,
		},
		{
			name:   "incomplete line",
			line:   "/dev/sda1",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device, path, ok := parseMountLine(tt.line)
			if ok != tt.wantOk {
				t.Errorf("parseMountLine() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok {
				if device != tt.wantDevice {
					t.Errorf("device = %q, want %q", device, tt.wantDevice)
				}
				if path != tt.wantPath {
					t.Errorf("path = %q, want %q", path, tt.wantPath)
				}
			}
		})
	}
}

func TestUnescapeMountPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/simple/path", "/simple/path"},
		{`/path/with\040space`, "/path/with space"},
		{`/path/with\011tab`, "/path/with\ttab"},
		{`/multiple\040spaces\040here`, "/multiple spaces here"},
		{`/no\escape`, `/no\escape`}, // not valid octal
		{`/partial\04`, `/partial\04`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := unescapeMountPath(tt.input)
			if got != tt.want {
				t.Errorf("unescapeMountPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsOctal(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"040", true},
		{"000", true},
		{"777", true},
		{"089", false}, // 8 and 9 are not octal
		{"12a", false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isOctal(tt.s)
			if got != tt.want {
				t.Errorf("isOctal(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}
