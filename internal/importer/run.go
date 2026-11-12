package importer

import (
	"errors"
	"path/filepath"
	"slices"

	"github.com/llehouerou/waves/internal/rename"
)

// FileState is where one file of an import or of a retag is.
type FileState int

const (
	FilePending         FileState = iota // not reached yet
	FileWorking                          // being tagged, and moved for an import
	FileDone                             // tagged, and in the library for an import
	FileAlreadyImported                  // moved by an earlier import: a success
	FileFailed
)

// FileProgress is one file of an import or of a retag, and where it is.
type FileProgress struct {
	Path  string // the file: for an import, where it is in the download folder
	State FileState
	Dest  string // for an import, where the file is in the library once there
	Err   error  // why it failed
}

// errNoTrack fails a file the release has no track for: it has nowhere to go,
// and must never land on another file's track.
var errNoTrack = errors.New("no MusicBrainz track for this file")

// Run carries out a plan into the library source destRoot, one file after the
// other in the plan's order, then writes the cover in the album folder. It
// returns how each file ended.
//
// report, when not nil, is called before each file with where every file is.
// stop, when not nil, is asked before each file whether to end there: the
// files not reached stay pending.
func Run(plan Plan, destRoot string, cfg rename.Config, report func([]FileProgress), stop func() bool) []FileProgress {
	files := make([]FileProgress, len(plan.Files))
	for i := range plan.Files {
		files[i] = FileProgress{Path: plan.Files[i].Source}
	}

	albumDir := ""
	for i := range plan.Files {
		if stop != nil && stop() {
			break
		}
		files[i].State = FileWorking
		if report != nil {
			report(slices.Clone(files))
		}

		if plan.Files[i].Track < 0 {
			files[i].State, files[i].Err = FileFailed, errNoTrack
			continue
		}
		result, err := Import(plan.params(i, destRoot, cfg))
		if err != nil {
			files[i].State, files[i].Err = FileFailed, err
			continue
		}
		files[i].State, files[i].Dest = FileDone, result.DestPath
		if result.AlreadyImported {
			files[i].State = FileAlreadyImported
		}
		if albumDir == "" {
			albumDir = filepath.Dir(result.DestPath)
		}
	}

	// The fetched cover, never an image from the download folder: waves only
	// downloads audio, so any image there is another download's.
	if albumDir != "" {
		_ = WriteCoverArt(albumDir, plan.Cover) //nolint:errcheck // the tracks embed it already; the file is a nicety
	}
	return files
}
