package export

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// StartExportMsg initiates an export operation.
type StartExportMsg struct {
	Target      Target
	Tracks      []Track
	ConvertFLAC bool
	MountPath   string
}

// FileExportedMsg reports progress on a single file.
type FileExportedMsg struct {
	JobID   string
	Current int
	Track   Track
	Err     error
}

// CompleteMsg signals the export finished.
type CompleteMsg struct {
	JobID      string
	Artist     string
	Album      string
	TargetName string
	Failed     int
	Total      int
	Errors     []TrackError
}

// Params contains parameters for the export command.
type Params struct {
	Job         *Job
	Exporter    *Exporter
	ConvertFLAC bool
	BasePath    string
}

// ProgressMsg reports export progress for UI updates.
type ProgressMsg struct {
	JobID   string
	Current int
	Total   int
}

// BatchCmd starts the export and returns commands for progress updates.
func BatchCmd(params Params) tea.Cmd {
	return exportNextFile(params, 0)
}

// exportNextFile exports a single file and returns a command to continue.
func exportNextFile(params Params, index int) tea.Cmd {
	return func() tea.Msg {
		job := params.Job
		tracks := job.Tracks()

		// Check if done
		if index >= len(tracks) || job.IsCanceled() {
			job.Complete()
			// Get artist/album from first track for notification
			var artist, album string
			if len(tracks) > 0 {
				artist = tracks[0].Artist
				album = tracks[0].Album
			}
			return CompleteMsg{
				JobID:      job.JobBar().ID,
				Artist:     artist,
				Album:      album,
				TargetName: job.Target().Name,
				Failed:     len(job.Errors()),
				Total:      len(tracks),
				Errors:     job.Errors(),
			}
		}

		track := tracks[index]
		target := job.Target()

		// Generate track info for path
		info := TrackInfo{
			Artist:      track.Artist,
			Album:       track.Album,
			Title:       track.Title,
			TrackNumber: track.TrackNum,
			DiscNumber:  track.DiscNum,
			TotalDiscs:  track.DiscTotal,
			Extension:   track.Extension,
		}

		// Change extension if converting
		if params.ConvertFLAC && NeedsConversion(info.Extension) {
			info.Extension = ".mp3"
		}

		// Generate relative path
		relPath := GenerateExportPath(info, target.FolderStructure)

		// Build full destination path
		var dstPath string
		if target.DeviceUUID == "" {
			// Custom folder target - BasePath is the full path
			dstPath = filepath.Join(params.BasePath, relPath)
		} else {
			// Device target - combine mount path, subfolder, and relative path
			dstPath = filepath.Join(params.BasePath, target.Subfolder, relPath)
		}

		// Export the file
		err := params.Exporter.ExportFile(track.SrcPath, dstPath, params.ConvertFLAC)
		if err != nil {
			job.RecordError(track, err)
		}

		job.Progress(index + 1)

		// Return progress message - handler will chain to next file
		return ProgressMsg{
			JobID:   job.JobBar().ID,
			Current: index + 1,
			Total:   len(tracks),
		}
	}
}

// ContinueExportCmd returns a command to export the next file.
func ContinueExportCmd(params Params, nextIndex int) tea.Cmd {
	return exportNextFile(params, nextIndex)
}
