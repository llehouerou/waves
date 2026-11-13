package core

import (
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
)

// restore puts back the volume and the queue saved by an earlier run. It runs
// before the loop starts: restoring is not a queue edit, and emits nothing but
// its failures.
func (c *Core) restore(lib *library.Library) {
	if vol, err := c.state.GetVolume(); err != nil {
		c.emit(Failed{Op: errmsg.OpVolumeLoad, Err: err})
	} else {
		c.player.SetVolume(vol.Volume)
		c.player.SetMuted(vol.Muted)
	}

	saved, err := c.state.GetQueue()
	if err != nil {
		c.emit(Failed{Op: errmsg.OpQueueLoad, Err: err})
		return
	}
	if saved == nil {
		return
	}
	queue := playback.SavedQueue{
		Tracks:     make([]Track, 0, len(saved.Tracks)),
		Index:      saved.CurrentIndex,
		RepeatMode: RepeatMode(saved.RepeatMode),
		Shuffle:    saved.Shuffle,
	}
	for _, t := range saved.Tracks {
		track := Track{
			ID:          t.TrackID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
		// The library knows more than what is saved: genre, disc number, year.
		if t.TrackID > 0 && lib != nil {
			if lt, err := lib.TrackByID(t.TrackID); err == nil {
				track = playback.TrackFromPlaylist(playlist.FromLibraryTrack(*lt))
			}
		}
		queue.Tracks = append(queue.Tracks, track)
	}
	c.playback.svc.RestoreQueue(queue)
}

// mark flags something as to be saved. Marks made while a save runs merge
// into one: a burst of queue edits costs one or two writes.
func mark(dirty chan struct{}) {
	select {
	case dirty <- struct{}{}:
	default:
	}
}

// persist saves what is marked, away from the loop: writing a long queue
// takes time the loop must not spend with events waiting.
func (c *Core) persist() {
	defer close(c.persistDone)
	for {
		select {
		case <-c.stop:
			return
		case <-c.queueDirty:
			if err := c.saveQueue(); err != nil {
				c.emit(Failed{Op: errmsg.OpQueueSave, Err: err})
			}
		case <-c.volumeDirty:
			if err := c.saveVolume(); err != nil {
				c.emit(Failed{Op: errmsg.OpVolumeSave, Err: err})
			}
		}
	}
}

// saveQueue saves the queue as it is now, with its position and its modes.
func (c *Core) saveQueue() error {
	pb := c.playback
	tracks := pb.QueueTracks()
	saved := state.QueueState{
		CurrentIndex: pb.QueueCurrentIndex(),
		RepeatMode:   int(pb.RepeatMode()),
		Shuffle:      pb.Shuffle(),
		Tracks:       make([]state.QueueTrack, len(tracks)),
	}
	for i, t := range tracks {
		saved.Tracks[i] = state.QueueTrack{
			TrackID:     t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
	}
	return c.state.SaveQueue(saved)
}

func (c *Core) saveVolume() error {
	return c.state.SaveVolume(c.player.Volume(), c.player.Muted())
}
