package core

import (
	"errors"
	"path/filepath"
	"sync/atomic"

	"github.com/llehouerou/waves/internal/export"
)

// The export data a client handles, so that it imports the core only.
type (
	// ExportTarget is a saved place to export to: a folder of a removable
	// device, known by the device's UUID, or a folder of this machine when
	// it has no DeviceUUID (its Subfolder is then the whole path).
	ExportTarget = export.Target

	// ExportVolume is a removable device plugged in and mounted.
	ExportVolume = export.Volume

	// ExportTrack is a library track as an export takes it.
	ExportTrack = export.Track

	// ExportFailure is a track an export could not write, and why.
	ExportFailure = export.TrackError

	// ExportFolderStructure is how an export lays its files out.
	ExportFolderStructure = export.FolderStructure
)

// How an export lays its files out under its target.
const (
	ExportFlat         = export.FolderStructureFlat         // Artist - Album/01 - Track.mp3
	ExportHierarchical = export.FolderStructureHierarchical // Artist/Album/01 - Track.mp3
	ExportSingle       = export.FolderStructureSingle       // Artist - Album - 01 - Track.mp3
)

// ExportCanConvert reports whether an export can convert a file of this
// extension to MP3: a FLAC.
func ExportCanConvert(ext string) bool {
	return export.NeedsConversion(ext)
}

// ErrTargetNotConnected is returned by Start for a target whose device is not
// mounted, and ends an export whose device was unplugged on the way.
var ErrTargetNotConnected = errors.New("the device of the export target is not connected")

// ExportRequest is what an export writes where.
type ExportRequest struct {
	TargetID    int64         // the target exported to
	Tracks      []ExportTrack // as Tracks returned them
	ConvertFLAC bool          // FLAC files are converted to MP3, 320 kbps
}

// Exports is the exports domain of the core: the export targets, and the
// exports to them, which the core runs, several at once when asked.
type Exports struct {
	targets *export.TargetRepository       // nil without a database
	volumes func() ([]ExportVolume, error) // the removable devices mounted now
	core    *Core
	last    atomic.Int64 // the ID of the last export started
}

// Targets returns the saved targets, by name.
func (e *Exports) Targets() ([]ExportTarget, error) {
	return e.targets.List()
}

// CreateTarget saves a new target and returns its ID.
func (e *Exports) CreateTarget(t ExportTarget) (id int64, err error) {
	return e.targets.Create(t)
}

// RenameTarget gives a saved target another name.
func (e *Exports) RenameTarget(id int64, name string) error {
	return e.targets.Rename(id, name)
}

// DeleteTarget forgets a saved target. What was exported to it stays there.
func (e *Exports) DeleteTarget(id int64) error {
	return e.targets.Delete(id)
}

// Volumes returns the removable devices mounted now: those a new target can
// be made on, and those whose targets can be exported to.
func (e *Exports) Volumes() ([]ExportVolume, error) {
	return e.volumes()
}

// Tracks turns library tracks into the tracks an export takes: where the file
// is, the names it is exported under, and how many discs of its album are
// among them. A client shows them, then hands them back in an ExportRequest.
// An ID the library does not have is left out.
func (e *Exports) Tracks(trackIDs []int64) []ExportTrack {
	lib := e.core.library.lib
	type album struct{ artist, name string }
	discs := make(map[album]int)
	tracks := make([]ExportTrack, 0, len(trackIDs))
	for _, id := range trackIDs {
		t, err := lib.TrackByID(id)
		if err != nil {
			continue
		}
		tracks = append(tracks, ExportTrack{
			ID:        id,
			SrcPath:   t.Path,
			Artist:    t.AlbumArtist,
			Album:     t.Album,
			Title:     t.Title,
			TrackNum:  t.TrackNumber,
			DiscNum:   t.DiscNumber,
			Extension: filepath.Ext(t.Path),
		})
		of := album{t.AlbumArtist, t.Album}
		discs[of] = max(discs[of], t.DiscNumber)
	}
	for i := range tracks {
		tracks[i].DiscTotal = discs[album{tracks[i].Artist, tracks[i].Album}]
	}
	return tracks
}

// Start starts an export and returns at once, with the ID its events carry.
// The core copies each track under the target, or converts it:
// ExportProgressed follows it and ExportFinished ends it.
//
// Several exports can run at once.
func (e *Exports) Start(req ExportRequest) (id int64, err error) {
	if len(req.Tracks) == 0 {
		return 0, errors.New("export: no track to export")
	}
	target, err := e.targets.Get(req.TargetID)
	if err != nil {
		return 0, err
	}
	root, err := e.root(&target)
	if err != nil {
		return 0, err
	}

	id = e.last.Add(1)
	batch := export.Batch{
		Tracks:      req.Tracks,
		Root:        root,
		Structure:   target.FolderStructure,
		ConvertFLAC: req.ConvertFLAC,
	}
	// What both events say of the export.
	name, artist, album, total := target.Name, req.Tracks[0].Artist, req.Tracks[0].Album, len(req.Tracks)

	e.core.work.Go(func() {
		ctx, cancel := e.core.untilStopped()
		defer cancel()
		failed, done, err := batch.Run(ctx, throttled(func(done int) {
			e.core.emit(ExportProgressed{ID: id, Target: name, Artist: artist, Album: album, Done: done, Total: total})
		}), func() error {
			// Unplugged since the last track, the device leaves a mount point
			// that may still be a folder: nothing must be written there.
			now, err := e.root(&target)
			if err == nil && now != root {
				err = ErrTargetNotConnected
			}
			return err
		})
		if e.core.stopping() {
			return // cut short by Close: not an export that ended
		}
		e.core.emit(ExportFinished{
			ID:     id,
			Target: name,
			Artist: artist,
			Album:  album,
			Done:   done,
			Total:  total,
			Failed: failed,
			Err:    err,
		})
	})
	return id, nil
}

// root is the folder a target exports under: its own for a folder of this
// machine, its folder on the device for one that is mounted.
func (e *Exports) root(target *ExportTarget) (string, error) {
	if target.DeviceUUID == "" {
		return target.Subfolder, nil
	}
	volumes, err := e.volumes()
	if err != nil {
		return "", err
	}
	for i := range volumes {
		if volumes[i].UUID == target.DeviceUUID {
			return filepath.Join(volumes[i].MountPath, target.Subfolder), nil
		}
	}
	return "", ErrTargetNotConnected
}
