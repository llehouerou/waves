package albumart

import (
	"os"
	"strings"
	"sync"
	"testing"
)

const testTrackPath = "/track.mp3"

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}

	// Initial state should be empty
	if r.currentPath != "" {
		t.Errorf("initial currentPath = %q, want empty", r.currentPath)
	}
	if r.currentImageID != 0 {
		t.Errorf("initial currentImageID = %d, want 0", r.currentImageID)
	}
	if r.width != 0 || r.height != 0 {
		t.Errorf("initial dimensions = %dx%d, want 0x0", r.width, r.height)
	}
}

func TestRenderer_SetSize(t *testing.T) {
	r := New()

	r.SetSize(8, 4)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.width != 8 {
		t.Errorf("width = %d, want 8", r.width)
	}
	if r.height != 4 {
		t.Errorf("height = %d, want 4", r.height)
	}
}

func TestRenderer_SetSize_Multiple(t *testing.T) {
	r := New()

	r.SetSize(8, 4)
	r.SetSize(16, 8)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.width != 16 {
		t.Errorf("width = %d, want 16", r.width)
	}
	if r.height != 8 {
		t.Errorf("height = %d, want 8", r.height)
	}
}

func TestRenderer_Apply_WithImage(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	pngData := createTestPNG(t, 64, 64)
	processed := &ProcessedImage{Data: pngData}

	cmd := r.Apply(testTrackPath, processed)

	// Should have transmitted the image
	if !strings.Contains(cmd, escStart) {
		t.Error("Apply should return transmit command")
	}
	if !strings.Contains(cmd, "a=t") {
		t.Error("command should contain transmit action")
	}

	// State should be updated
	if r.currentPath != testTrackPath {
		t.Errorf("currentPath = %q, want %s", r.currentPath, testTrackPath)
	}
	if r.currentImageID == 0 {
		t.Error("currentImageID should be non-zero after Apply")
	}
}

func TestRenderer_Apply_NoImage(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	// Apply with nil processed
	cmd := r.Apply(testTrackPath, nil)

	// Should not transmit
	if strings.Contains(cmd, "a=t") {
		t.Error("Apply with nil should not transmit")
	}

	// State should be updated but no image
	if r.currentPath != testTrackPath {
		t.Errorf("currentPath = %q, want %s", r.currentPath, testTrackPath)
	}
	if r.currentImageID != 0 {
		t.Errorf("currentImageID = %d, want 0", r.currentImageID)
	}
}

func TestRenderer_Apply_EmptyData(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	processed := &ProcessedImage{Data: []byte{}}
	cmd := r.Apply(testTrackPath, processed)

	// Should not transmit
	if strings.Contains(cmd, "a=t") {
		t.Error("Apply with empty data should not transmit")
	}

	if r.currentImageID != 0 {
		t.Error("currentImageID should be 0 with empty data")
	}
}

func TestRenderer_Apply_DeletesOldImage(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	// First apply
	pngData := createTestPNG(t, 64, 64)
	_ = r.Apply("/track1.mp3", &ProcessedImage{Data: pngData})
	oldID := r.currentImageID

	// Second apply
	cmd := r.Apply("/track2.mp3", &ProcessedImage{Data: pngData})

	// Should delete old image
	if !strings.Contains(cmd, "a=d") {
		t.Error("Apply should delete old image")
	}
	expectedDelete := "i=" + itoa(oldID)
	if !strings.Contains(cmd, expectedDelete) {
		t.Errorf("delete command should reference old ID %d", oldID)
	}
}

func TestRenderer_Clear(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	// Set up an image
	pngData := createTestPNG(t, 64, 64)
	_ = r.Apply(testTrackPath, &ProcessedImage{Data: pngData})
	oldID := r.currentImageID

	// Clear
	cmd := r.Clear()

	// Should delete the image
	if !strings.Contains(cmd, "a=d") {
		t.Error("Clear should return delete command")
	}
	expectedDelete := "i=" + itoa(oldID)
	if !strings.Contains(cmd, expectedDelete) {
		t.Errorf("delete command should reference old ID %d", oldID)
	}

	// State should be reset
	if r.currentPath != "" {
		t.Errorf("currentPath = %q, want empty", r.currentPath)
	}
	if r.currentImageID != 0 {
		t.Errorf("currentImageID = %d, want 0", r.currentImageID)
	}
}

func TestRenderer_Clear_NoImage(t *testing.T) {
	r := New()

	cmd := r.Clear()

	// Should return empty string
	if cmd != "" {
		t.Errorf("Clear with no image should return empty, got %q", cmd)
	}
}

func TestRenderer_HasImage(t *testing.T) {
	r := New()

	// Initially no image
	if r.HasImage() {
		t.Error("HasImage should be false initially")
	}

	// After apply with image
	r.SetSize(8, 4)
	pngData := createTestPNG(t, 64, 64)
	_ = r.Apply(testTrackPath, &ProcessedImage{Data: pngData})

	if !r.HasImage() {
		t.Error("HasImage should be true after Apply")
	}

	// After clear
	_ = r.Clear()

	if r.HasImage() {
		t.Error("HasImage should be false after Clear")
	}
}

func TestRenderer_CurrentPath(t *testing.T) {
	r := New()

	// Initially empty
	if r.CurrentPath() != "" {
		t.Errorf("CurrentPath initially = %q, want empty", r.CurrentPath())
	}

	// After apply
	r.SetSize(8, 4)
	pngData := createTestPNG(t, 64, 64)
	_ = r.Apply(testTrackPath, &ProcessedImage{Data: pngData})

	if r.CurrentPath() != testTrackPath {
		t.Errorf("CurrentPath = %q, want %s", r.CurrentPath(), testTrackPath)
	}

	// After apply with no image (still updates path)
	_ = r.Apply("/track2.mp3", nil)

	if r.CurrentPath() != "/track2.mp3" {
		t.Errorf("CurrentPath = %q, want /track2.mp3", r.CurrentPath())
	}
}

func TestRenderer_GetPlacementCmd_HasImage(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	pngData := createTestPNG(t, 64, 64)
	_ = r.Apply(testTrackPath, &ProcessedImage{Data: pngData})

	cmd := r.GetPlacementCmd(5, 10, testTrackPath)

	// Should return placement command
	if !strings.Contains(cmd, "a=p") {
		t.Error("GetPlacementCmd should return placement command")
	}
	if !strings.Contains(cmd, "\x1b[5;10H") {
		t.Error("placement should be at row 5, col 10")
	}
}

func TestRenderer_GetPlacementCmd_NoImage(t *testing.T) {
	r := New()

	cmd := r.GetPlacementCmd(5, 10, testTrackPath)

	if cmd != "" {
		t.Errorf("GetPlacementCmd with no image should return empty, got %q", cmd)
	}
}

func TestRenderer_GetPlacementCmd_WrongPath(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	pngData := createTestPNG(t, 64, 64)
	_ = r.Apply("/track1.mp3", &ProcessedImage{Data: pngData})

	// Request placement with wrong path
	cmd := r.GetPlacementCmd(5, 10, "/track2.mp3")

	if cmd != "" {
		t.Errorf("GetPlacementCmd with wrong path should return empty, got %q", cmd)
	}
}

func TestRenderer_GetPlaceholder(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	placeholder := r.GetPlaceholder()
	lines := strings.Split(placeholder, "\n")

	if len(lines) != 4 {
		t.Errorf("placeholder has %d lines, want 4", len(lines))
	}
	for i, line := range lines {
		if len(line) != 8 {
			t.Errorf("line %d has width %d, want 8", i, len(line))
		}
	}
}

func TestRenderer_GetPlaceholder_ZeroSize(t *testing.T) {
	r := New()
	// Don't set size

	placeholder := r.GetPlaceholder()

	if placeholder != "" {
		t.Errorf("placeholder with zero size should be empty, got %q", placeholder)
	}
}

func TestRenderer_ProcessTrackAsync_EmptyPath(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	result := r.ProcessTrackAsync("")

	if result != nil {
		t.Errorf("ProcessTrackAsync with empty path should return nil, got %v", result)
	}
}

func TestRenderer_ProcessTrackAsync_NonexistentPath(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	result := r.ProcessTrackAsync("/nonexistent/track.mp3")

	// Should return empty ProcessedImage (not nil)
	if result == nil {
		t.Fatal("ProcessTrackAsync should return ProcessedImage for nonexistent path")
	}
	if len(result.Data) != 0 {
		t.Errorf("ProcessTrackAsync for nonexistent path should have empty data, got %d bytes", len(result.Data))
	}
}

func TestRenderer_UniqueImageIDs(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	pngData := createTestPNG(t, 64, 64)
	ids := make(map[uint32]bool)

	// Apply multiple times and collect IDs
	for range 10 {
		_ = r.Apply(testTrackPath, &ProcessedImage{Data: pngData})
		if ids[r.currentImageID] {
			t.Errorf("duplicate image ID: %d", r.currentImageID)
		}
		ids[r.currentImageID] = true
	}
}

func TestRenderer_Concurrent_SetSize(t *testing.T) {
	t.Parallel()
	r := New()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(w, h int) {
			defer wg.Done()
			r.SetSize(w, h)
		}(i%20, i%10)
	}
	wg.Wait()

	// Should not panic
}

func TestRenderer_Concurrent_HasImage(t *testing.T) {
	r := New()
	r.SetSize(8, 4)

	var wg sync.WaitGroup

	// One goroutine applying images
	pngData := createTestPNG(t, 64, 64)
	wg.Add(1)
	go func() { //nolint:modernize // WaitGroup.Go() not in stdlib
		defer wg.Done()
		for range 50 {
			r.Apply(testTrackPath, &ProcessedImage{Data: pngData})
		}
	}()

	// Multiple goroutines reading
	for range 10 {
		wg.Add(1)
		go func() { //nolint:modernize // WaitGroup.Go() not in stdlib
			defer wg.Done()
			for range 50 {
				_ = r.HasImage()
				_ = r.CurrentPath()
				_ = r.GetPlaceholder()
			}
		}()
	}

	wg.Wait()
	// Should not panic or race
}

func TestIsKittySupported_EnvVariables(t *testing.T) {
	// Save original env
	origKittyWindowID := os.Getenv("KITTY_WINDOW_ID")
	origTerm := os.Getenv("TERM")
	origTermProgram := os.Getenv("TERM_PROGRAM")
	origGhostty := os.Getenv("GHOSTTY_RESOURCES_DIR")
	origKonsole := os.Getenv("KONSOLE_VERSION")

	// Restore after test
	defer func() {
		os.Setenv("KITTY_WINDOW_ID", origKittyWindowID)
		os.Setenv("TERM", origTerm)
		os.Setenv("TERM_PROGRAM", origTermProgram)
		os.Setenv("GHOSTTY_RESOURCES_DIR", origGhostty)
		os.Setenv("KONSOLE_VERSION", origKonsole)
	}()

	// Clear all
	os.Unsetenv("KITTY_WINDOW_ID")
	os.Unsetenv("TERM")
	os.Unsetenv("TERM_PROGRAM")
	os.Unsetenv("GHOSTTY_RESOURCES_DIR")
	os.Unsetenv("KONSOLE_VERSION")

	tests := []struct {
		name   string
		envVar string
		envVal string
		want   bool
	}{
		{"KITTY_WINDOW_ID", "KITTY_WINDOW_ID", "1", true},
		{"TERM xterm-kitty", "TERM", "xterm-kitty", true},
		{"TERM_PROGRAM WezTerm", "TERM_PROGRAM", "WezTerm", true},
		{"GHOSTTY_RESOURCES_DIR", "GHOSTTY_RESOURCES_DIR", "/some/path", true},
		{"KONSOLE_VERSION 2204xx", "KONSOLE_VERSION", "220400", true},
		{"KONSOLE_VERSION old", "KONSOLE_VERSION", "210000", false},
		{"TERM contains kitty", "TERM", "something-kitty", true},
		{"no support", "TERM", "xterm-256color", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all
			os.Unsetenv("KITTY_WINDOW_ID")
			os.Unsetenv("TERM")
			os.Unsetenv("TERM_PROGRAM")
			os.Unsetenv("GHOSTTY_RESOURCES_DIR")
			os.Unsetenv("KONSOLE_VERSION")

			// Set test env
			os.Setenv(tt.envVar, tt.envVal)

			got := IsKittySupported()
			if got != tt.want {
				t.Errorf("IsKittySupported() with %s=%s = %v, want %v",
					tt.envVar, tt.envVal, got, tt.want)
			}
		})
	}
}

func TestProcessedImage(t *testing.T) {
	// ProcessedImage is just a data container
	img := &ProcessedImage{Data: []byte("test data")}

	if string(img.Data) != "test data" {
		t.Errorf("ProcessedImage.Data = %q, want %q", img.Data, "test data")
	}

	// Empty ProcessedImage
	empty := &ProcessedImage{}
	if len(empty.Data) != 0 {
		t.Error("empty ProcessedImage should have nil/empty Data")
	}
}

func TestGetNextImageID_Increments(t *testing.T) {
	id1 := getNextImageID()
	id2 := getNextImageID()
	id3 := getNextImageID()

	if id2 != id1+1 {
		t.Errorf("id2 = %d, want %d", id2, id1+1)
	}
	if id3 != id2+1 {
		t.Errorf("id3 = %d, want %d", id3, id2+1)
	}
}

func TestGetNextImageID_Concurrent(t *testing.T) {
	ids := make(chan uint32, 1000)

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() { //nolint:modernize // WaitGroup.Go() not in stdlib
			defer wg.Done()
			for range 10 {
				ids <- getNextImageID()
			}
		}()
	}
	wg.Wait()
	close(ids)

	// All IDs should be unique
	seen := make(map[uint32]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate ID: %d", id)
		}
		seen[id] = true
	}
}
