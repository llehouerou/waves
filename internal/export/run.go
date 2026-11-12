package export

import (
	"context"
	"path/filepath"
)

// Batch is one export: the tracks, and where they go.
type Batch struct {
	Tracks      []Track
	Root        string          // the folder exported under: on a device, its mount point and the target's subfolder
	Structure   FolderStructure // how the files are laid out under Root
	ConvertFLAC bool            // FLAC files are converted to MP3
}

// Run exports each track under Root, in order. It returns the tracks it could
// not write and how many it reached: a track that fails does not stop the
// others.
//
// report, when not nil, is called before each track with how many are done.
// before, when not nil, is asked before each track whether to go on: its
// error ends the export there, and is returned.
//
// The end of ctx ends the export with ctx's error: before the next track when
// it is on a copy, which is finished, at once when it is on a conversion,
// whose ffmpeg is killed and partial file removed.
func (b Batch) Run(ctx context.Context, report func(done int), before func() error) (failed []TrackError, done int, err error) {
	exporter := NewExporter()
	for i := range b.Tracks {
		if err := ctx.Err(); err != nil {
			return failed, i, err
		}
		if before != nil {
			if err := before(); err != nil {
				return failed, i, err
			}
		}
		if report != nil {
			report(i)
		}
		track := &b.Tracks[i]
		if err := exporter.ExportFile(ctx, track.SrcPath, b.dest(track), b.ConvertFLAC); err != nil {
			if ctx.Err() != nil {
				return failed, i, ctx.Err() // the conversion was killed: not a track that failed
			}
			failed = append(failed, TrackError{Track: *track, Err: err})
		}
	}
	return failed, len(b.Tracks), nil
}

// dest is where a track is exported: under Root, laid out and named after its
// tags, as an MP3 when it is converted.
func (b Batch) dest(track *Track) string {
	info := TrackInfo{
		Artist:      track.Artist,
		Album:       track.Album,
		Title:       track.Title,
		TrackNumber: track.TrackNum,
		DiscNumber:  track.DiscNum,
		TotalDiscs:  track.DiscTotal,
		Extension:   track.Extension,
	}
	if b.ConvertFLAC && NeedsConversion(info.Extension) {
		info.Extension = ".mp3"
	}
	return filepath.Join(b.Root, GenerateExportPath(info, b.Structure))
}
