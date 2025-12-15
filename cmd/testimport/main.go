// Test program to import AC/DC - Back In Black album
package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/llehouerou/waves/internal/importer"
	"github.com/llehouerou/waves/internal/musicbrainz"
)

const (
	sourceDir = "/media/srv02/downloads/#soulseek/AC-DC - Back In Black"
	destRoot  = "/home/laurent/test"
)

func main() {
	log.Println("Starting import test for AC/DC - Back In Black")

	// Create MusicBrainz client
	mbClient := musicbrainz.NewClient()

	// Search for the release
	log.Println("Searching for AC/DC Back In Black...")
	releases, err := mbClient.SearchReleases("artist:AC/DC release:\"Back In Black\"")
	if err != nil {
		log.Fatalf("Failed to search releases: %v", err)
	}
	log.Printf("Found %d releases:", len(releases))
	for i := range releases {
		if i < 5 { // Show first 5
			r := &releases[i]
			log.Printf("  [%d] %s - %s (%s) %d tracks - ID: %s",
				r.Score, r.Artist, r.Title, r.Date, r.TrackCount, r.ID)
		}
	}

	// Use the first result (highest score)
	if len(releases) == 0 {
		log.Fatalf("No releases found")
	}
	selectedRelease := releases[0]
	log.Printf("\nUsing release: %s - %s (%s)", selectedRelease.Artist, selectedRelease.Title, selectedRelease.ID)

	// Create a minimal release group with the data we need
	releaseGroup := &musicbrainz.ReleaseGroup{
		ID:           "3f5eb351-66b6-327c-ae1d-e7353a4eb1f1",
		Title:        "Back in Black",
		PrimaryType:  "Album",
		FirstRelease: "1980-07-25",
		Artist:       "AC/DC",
		Genres:       []string{"hard rock", "rock"},
	}

	// Get release details (tracks)
	log.Println("Fetching release details...")
	release, err := mbClient.GetRelease(selectedRelease.ID)
	if err != nil {
		log.Fatalf("Failed to get release: %v", err)
	}
	log.Printf("Found release: %s (%s) - %d tracks - Genres: %v",
		release.Title, release.Date, len(release.Tracks), release.Genres)

	// List tracks
	for _, track := range release.Tracks {
		log.Printf("  Track %d: %s", track.Position, track.Title)
	}

	// Fetch cover art
	log.Println("Fetching cover art...")
	coverArt, err := mbClient.GetCoverArt(selectedRelease.ID)
	switch {
	case err != nil:
		log.Printf("Warning: Failed to get cover art: %v", err)
	case coverArt == nil:
		log.Println("No cover art available")
	default:
		log.Printf("Got cover art: %d bytes", len(coverArt))
	}

	// Get source files
	files, err := getSourceFiles(sourceDir)
	if err != nil {
		log.Fatalf("Failed to read source directory: %v", err)
	}
	log.Printf("Found %d source files", len(files))

	if len(files) != len(release.Tracks) {
		log.Printf("Warning: file count (%d) doesn't match track count (%d)",
			len(files), len(release.Tracks))
	}

	// Create destination directory
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		log.Fatalf("Failed to create destination: %v", err)
	}

	// Import each track
	log.Println("\nImporting tracks...")
	for i, file := range files {
		if i >= len(release.Tracks) {
			log.Printf("Skipping extra file: %s", file)
			continue
		}

		log.Printf("Importing track %d: %s", i+1, filepath.Base(file))

		result, err := importer.Import(importer.ImportParams{
			SourcePath:   file,
			DestRoot:     destRoot,
			ReleaseGroup: releaseGroup,
			Release:      release,
			TrackIndex:   i,
			DiscNumber:   1,
			TotalDiscs:   1,
			CoverArt:     coverArt,
			CopyMode:     true, // Copy, don't move
		})
		if err != nil {
			log.Printf("  ERROR: %v", err)
			continue
		}
		log.Printf("  -> %s", result.DestPath)
	}

	log.Println("\nImport complete!")
}

// getSourceFiles returns sorted list of audio files in directory
func getSourceFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".mp3" || ext == ".flac" {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	// Sort by filename (which includes track number prefix)
	sort.Strings(files)
	return files, nil
}
