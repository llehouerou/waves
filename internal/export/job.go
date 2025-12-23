package export

import (
	"fmt"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/ui/jobbar"
)

// Track contains info needed to export a single track.
type Track struct {
	ID        int64
	SrcPath   string
	Artist    string
	Album     string
	Title     string
	TrackNum  int
	DiscNum   int
	DiscTotal int
	Extension string
}

// TrackError records a failed export.
type TrackError struct {
	Track Track
	Err   error
}

// Job tracks the progress of an export operation.
type Job struct {
	mu       sync.Mutex
	bar      *jobbar.Job
	target   Target
	tracks   []Track
	failed   int
	errors   []TrackError
	canceled bool
}

// NewJob creates a new export job.
func NewJob(target Target, tracks []Track) *Job {
	// Build label with artist/album info from first track
	label := "Exporting"
	if len(tracks) > 0 {
		t := tracks[0]
		switch {
		case t.Artist != "" && t.Album != "":
			label = fmt.Sprintf("%s - %s → %s", t.Artist, t.Album, target.Name)
		case t.Artist != "":
			label = fmt.Sprintf("%s → %s", t.Artist, target.Name)
		default:
			label = "Exporting → " + target.Name
		}
	}

	return &Job{
		bar: &jobbar.Job{
			ID:    fmt.Sprintf("export-%d-%d", target.ID, time.Now().UnixNano()),
			Label: label,
			Total: len(tracks),
		},
		target: target,
		tracks: tracks,
	}
}

// JobBar returns the jobbar.Job for display.
func (j *Job) JobBar() *jobbar.Job {
	return j.bar
}

// Target returns the export target.
func (j *Job) Target() Target {
	return j.target
}

// Tracks returns the tracks to export.
func (j *Job) Tracks() []Track {
	return j.tracks
}

// Progress updates job progress.
func (j *Job) Progress(current int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.bar.Current = current
}

// RecordError records a failed export.
func (j *Job) RecordError(track Track, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.failed++
	j.errors = append(j.errors, TrackError{Track: track, Err: err})
}

// Complete marks the job as done.
func (j *Job) Complete() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.bar.Done = true
	if j.failed > 0 {
		j.bar.Label = fmt.Sprintf("Export complete: %d/%d (%d failed)",
			len(j.tracks)-j.failed, len(j.tracks), j.failed)
	} else {
		j.bar.Label = fmt.Sprintf("Export complete: %d files", len(j.tracks))
	}
}

// Cancel marks the job as canceled.
func (j *Job) Cancel() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.canceled = true
	j.bar.Done = true
	j.bar.Label = "Export canceled"
}

// IsCanceled returns true if the job was canceled.
func (j *Job) IsCanceled() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.canceled
}

// Errors returns all export errors.
func (j *Job) Errors() []TrackError {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.errors
}
