package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// trustTitles are the tracks of the album newExportCore puts in the library.
var trustTitles = []string{"Amazing Grace", "Canada", "Candy Girl"}

// newExportCore builds a core whose library holds Low's Trust, three tagged
// files in one library source, and returns the IDs of its tracks in order.
func newExportCore(t *testing.T, opts Options) (c *Core, ids []int64) {
	t.Helper()
	source := t.TempDir()
	paths := make([]string, len(trustTitles))
	for i, title := range trustTitles {
		paths[i] = filepath.Join(source, "Low", "Trust", title+".mp3")
		tagstest.WriteMP3(t, paths[i], FileTags{Artist: "Low", AlbumArtist: "Low", Album: "Trust", Title: title, TrackNumber: i + 1})
	}
	opts.LibrarySources = []string{source}
	c, _ = newTestCore(t, opts)
	if err := c.library.lib.AddTracks(paths); err != nil {
		t.Fatalf("add tracks: %v", err)
	}
	ids, err := c.Library().AlbumTrackIDs("Low", "Trust")
	if err != nil || len(ids) != len(trustTitles) {
		t.Fatalf("album track IDs = %v (err %v), want %d", ids, err, len(trustTitles))
	}
	return c, ids
}

// folderTarget saves an export target that is the folder dir, flat.
func folderTarget(t *testing.T, c *Core, dir string) int64 {
	t.Helper()
	id, err := c.Exports().CreateTarget(ExportTarget{Name: "Backup", Subfolder: dir, FolderStructure: ExportFlat})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	return id
}

// exported reports whether track is under root, whole, where the flat layout
// puts it.
func exported(root string, track *ExportTrack, name string) bool {
	want, err := os.ReadFile(track.SrcPath)
	if err != nil {
		return false
	}
	got, err := os.ReadFile(filepath.Join(root, "Low - Trust", name))
	return err == nil && bytes.Equal(got, want)
}

func TestExport_RunsToItsEndWithNoClientDrivingIt(t *testing.T) {
	c, ids := newExportCore(t, Options{})
	dest := t.TempDir()
	ex := c.Exports()
	tracks := ex.Tracks(ids)
	if len(tracks) != 3 {
		t.Fatalf("tracks = %+v, want the three of the album", tracks)
	}

	if _, err := ex.Start(ExportRequest{TargetID: folderTarget(t, c, dest), Tracks: tracks}); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Not an event is read, nothing more is asked: the core exports on its own.
	names := []string{"01 - Amazing Grace.mp3", "02 - Canada.mp3", "03 - Candy Girl.mp3"}
	eventually(t, "every track is under the target, whole", func() bool {
		for i, name := range names {
			if !exported(dest, &tracks[i], name) {
				return false
			}
		}
		return true
	})
}

func TestExport_ReportsWhereItIsThenHowItEnded(t *testing.T) {
	c, ids := newExportCore(t, Options{})
	ex := c.Exports()

	id, err := ex.Start(ExportRequest{TargetID: folderTarget(t, c, t.TempDir()), Tracks: ex.Tracks(ids)})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// What a client labels the export with, before its first track.
	progress := nextEvent[ExportProgressed](t, c)
	if want := (ExportProgressed{ID: id, Target: "Backup", Artist: "Low", Album: "Trust", Done: 0, Total: 3}); progress != want {
		t.Errorf("first ExportProgressed = %+v, want %+v", progress, want)
	}

	finished := nextEvent[ExportFinished](t, c)
	if finished.ID != id || finished.Target != "Backup" || finished.Artist != "Low" || finished.Album != "Trust" {
		t.Errorf("ExportFinished = %+v, want export %d of Low - Trust to Backup", finished, id)
	}
	if finished.Done != 3 || finished.Total != 3 || len(finished.Failed) != 0 || finished.Err != nil {
		t.Errorf("ExportFinished = %+v, want the three tracks done and no failure", finished)
	}
}

// A file gone since the library last saw it: the others are exported, and the
// end of the export says which one was not.
func TestExport_ATrackThatFailsDoesNotStopTheOthers(t *testing.T) {
	c, ids := newExportCore(t, Options{})
	dest := t.TempDir()
	ex := c.Exports()
	tracks := ex.Tracks(ids)
	canada, err := os.ReadFile(tracks[1].SrcPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tracks[1].SrcPath); err != nil {
		t.Fatal(err)
	}

	if _, err := ex.Start(ExportRequest{TargetID: folderTarget(t, c, dest), Tracks: tracks}); err != nil {
		t.Fatalf("start: %v", err)
	}

	finished := nextEvent[ExportFinished](t, c)
	if finished.Done != 3 || finished.Err != nil {
		t.Errorf("ExportFinished = %+v, want the three tracks reached and the export not cut short", finished)
	}
	if len(finished.Failed) != 1 || finished.Failed[0].Track.Title != "Canada" || finished.Failed[0].Err == nil {
		t.Errorf("failed = %+v, want Canada and why", finished.Failed)
	}
	if !exported(dest, &tracks[0], "01 - Amazing Grace.mp3") || !exported(dest, &tracks[2], "03 - Candy Girl.mp3") {
		t.Error("the tracks around the one that failed were not exported")
	}
	if err := os.WriteFile(tracks[1].SrcPath, canada, 0o600); err != nil { // exported compares with the source
		t.Fatal(err)
	}
	if exported(dest, &tracks[1], "02 - Canada.mp3") {
		t.Error("the track that failed is under the target")
	}
}

func TestExport_RefusesWhatItCannotStart(t *testing.T) {
	c, ids := newExportCore(t, Options{})
	ex := c.Exports()
	target := folderTarget(t, c, t.TempDir())

	if _, err := ex.Start(ExportRequest{TargetID: target}); err == nil {
		t.Error("an export of no track was accepted")
	}
	if _, err := ex.Start(ExportRequest{TargetID: target + 1, Tracks: ex.Tracks(ids)}); err == nil {
		t.Error("an export to a target that does not exist was accepted")
	}
	expectNoEvent[ExportProgressed](t, c)
}

// stick is the removable devices of a machine with one USB stick plugged in,
// mounted at mount.
func stick(mount string) func() ([]ExportVolume, error) {
	return func() ([]ExportVolume, error) {
		return []ExportVolume{{Label: "STICK", UUID: "1234-ABCD", MountPath: mount, Device: "/dev/sdb1"}}, nil
	}
}

// stickTarget saves an export target that is the Music folder of the stick.
func stickTarget(t *testing.T, c *Core) int64 {
	t.Helper()
	id, err := c.Exports().CreateTarget(ExportTarget{
		Name:            "Stick",
		DeviceUUID:      "1234-ABCD",
		DeviceLabel:     "STICK",
		Subfolder:       "/Music",
		FolderStructure: ExportFlat,
	})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	return id
}

func TestExport_ToADeviceGoesUnderWhereItIsMounted(t *testing.T) {
	mount := t.TempDir()
	c, ids := newExportCore(t, Options{volumes: stick(mount)})
	ex := c.Exports()
	tracks := ex.Tracks(ids)

	if _, err := ex.Start(ExportRequest{TargetID: stickTarget(t, c), Tracks: tracks}); err != nil {
		t.Fatalf("start: %v", err)
	}

	if finished := nextEvent[ExportFinished](t, c); finished.Done != 3 || len(finished.Failed) != 0 || finished.Err != nil {
		t.Fatalf("ExportFinished = %+v, want the three tracks exported", finished)
	}
	if !exported(filepath.Join(mount, "Music"), &tracks[0], "01 - Amazing Grace.mp3") {
		t.Error("the first track is not in the target's folder of the device")
	}
}

func TestExport_RefusesATargetWhoseDeviceIsNotConnected(t *testing.T) {
	c, ids := newExportCore(t, Options{}) // no device plugged in
	ex := c.Exports()

	_, err := ex.Start(ExportRequest{TargetID: stickTarget(t, c), Tracks: ex.Tracks(ids)})

	if !errors.Is(err, ErrTargetNotConnected) {
		t.Errorf("Start = %v, want ErrTargetNotConnected", err)
	}
	expectNoEvent[ExportProgressed](t, c)
}

// The stick is pulled out while the second track is about to be written:
// nothing more is written where it was mounted, which may still be a folder.
func TestExport_StopsWhenItsDeviceIsUnplugged(t *testing.T) {
	mount := t.TempDir()
	plugged := stick(mount)
	var asked atomic.Int32
	c, ids := newExportCore(t, Options{volumes: func() ([]ExportVolume, error) {
		// There for Start and for the first track, gone from then on.
		if asked.Add(1) > 2 {
			return nil, nil
		}
		return plugged()
	}})
	ex := c.Exports()
	tracks := ex.Tracks(ids)

	if _, err := ex.Start(ExportRequest{TargetID: stickTarget(t, c), Tracks: tracks}); err != nil {
		t.Fatalf("start: %v", err)
	}

	finished := nextEvent[ExportFinished](t, c)
	if !errors.Is(finished.Err, ErrTargetNotConnected) || finished.Done != 1 || finished.Total != 3 || len(finished.Failed) != 0 {
		t.Errorf("ExportFinished = %+v, want it ended after one track of three by ErrTargetNotConnected", finished)
	}
	music := filepath.Join(mount, "Music")
	if !exported(music, &tracks[0], "01 - Amazing Grace.mp3") {
		t.Error("the track exported before the device left is not there")
	}
	if _, err := os.Stat(filepath.Join(music, "Low - Trust", "02 - Canada.mp3")); !os.IsNotExist(err) {
		t.Errorf("a track was written where the device was mounted: %v", err)
	}
}

func TestExportTracks_AreNamedAsTheirAlbumAndCountItsDiscs(t *testing.T) {
	mgr := openTestState(t)
	insert := func(path, artist, albumArtist string, disc, track int) int64 {
		t.Helper()
		res, err := mgr.DB().Exec(`
			INSERT INTO library_tracks
				(path, mtime, artist, album_artist, album, title, disc_number, track_number, year, added_at, updated_at)
			VALUES (?, 1000, ?, ?, 'Box', 'Song', ?, ?, 2024, 1000, 1000)
		`, path, artist, albumArtist, disc, track)
		if err != nil {
			t.Fatalf("insert track: %v", err)
		}
		id, _ := res.LastInsertId() //nolint:errcheck // SQLite always has it
		return id
	}
	first := insert("/lib/box/1-01.flac", "Guest", "Low", 1, 1)
	second := insert("/lib/box/2-03.flac", "Low", "Low", 2, 3)
	other := insert("/lib/other/01.mp3", "Codeine", "Codeine", 1, 1) // another artist's album of the same name
	c, _ := newTestCore(t, Options{State: mgr})

	got := c.Exports().Tracks([]int64{first, second, other + 100, other})

	want := []ExportTrack{
		{ID: first, SrcPath: "/lib/box/1-01.flac", Artist: "Low", Album: "Box", Title: "Song", TrackNum: 1, DiscNum: 1, DiscTotal: 2, Extension: ".flac"},
		{ID: second, SrcPath: "/lib/box/2-03.flac", Artist: "Low", Album: "Box", Title: "Song", TrackNum: 3, DiscNum: 2, DiscTotal: 2, Extension: ".flac"},
		{ID: other, SrcPath: "/lib/other/01.mp3", Artist: "Codeine", Album: "Box", Title: "Song", TrackNum: 1, DiscNum: 1, DiscTotal: 1, Extension: ".mp3"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Tracks =\n%+v\nwant\n%+v", got, want)
	}
}

// targetNames are the names of the saved targets, as Targets orders them.
func targetNames(t *testing.T, c *Core) []string {
	t.Helper()
	targets, err := c.Exports().Targets()
	if err != nil {
		t.Fatalf("targets: %v", err)
	}
	names := make([]string, len(targets))
	for i := range targets {
		names[i] = targets[i].Name
	}
	return names
}

func TestExportTargets_AreSavedRenamedAndDeleted(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	ex := c.Exports()
	id := stickTarget(t, c)
	folderTarget(t, c, "/home/me/backup")
	if got := targetNames(t, c); !slices.Equal(got, []string{"Backup", "Stick"}) {
		t.Fatalf("targets = %v, want Backup and Stick", got)
	}

	if err := ex.RenameTarget(id, "Car"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	targets, err := ex.Targets()
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets = %+v (err %v), want two", targets, err)
	}
	// Renamed, and otherwise what it was.
	want := ExportTarget{ID: id, Name: "Car", DeviceUUID: "1234-ABCD", DeviceLabel: "STICK", Subfolder: "/Music", FolderStructure: ExportFlat}
	want.CreatedAt = targets[1].CreatedAt
	if targets[1] != want {
		t.Errorf("renamed target = %+v, want %+v", targets[1], want)
	}
	if err := ex.RenameTarget(id+100, "Nowhere"); err == nil {
		t.Error("renaming a target that does not exist was accepted")
	}

	if err := ex.DeleteTarget(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := targetNames(t, c); !slices.Equal(got, []string{"Backup"}) {
		t.Errorf("targets = %v, want Backup alone", got)
	}
}

func TestExportVolumes_AreTheDevicesMountedNow(t *testing.T) {
	c, _ := newTestCore(t, Options{volumes: stick("/run/media/me/STICK")})

	volumes, err := c.Exports().Volumes()

	if err != nil || len(volumes) != 1 || volumes[0].UUID != "1234-ABCD" || volumes[0].MountPath != "/run/media/me/STICK" {
		t.Errorf("volumes = %+v (err %v), want the stick", volumes, err)
	}
}

// slowTrack is a track whose file is a FIFO: copying it lasts for as long as
// the test holds its other end (feed).
func slowTrack(t *testing.T, title string, num int) ExportTrack {
	t.Helper()
	path := filepath.Join(t.TempDir(), title+".mp3")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	return ExportTrack{SrcPath: path, Artist: "Low", Album: "Trust", Title: title, TrackNum: num, Extension: ".mp3"}
}

// feed waits for an export to be copying a slow track, and returns the end
// the test writes the file at: the copy lasts until it is closed.
func feed(t *testing.T, track *ExportTrack) *os.File {
	t.Helper()
	opened := make(chan *os.File, 1)
	go func() {
		// Opening this end returns once the export has opened the other.
		w, _ := os.OpenFile(track.SrcPath, os.O_WRONLY, 0) //nolint:errcheck // a nil file fails the test below
		opened <- w
	}()
	select {
	case w := <-opened:
		if w == nil {
			t.Fatal("cannot open the slow track")
		}
		// Registered after the core's: a failed test must not leave Close
		// waiting for a copy nobody ends.
		t.Cleanup(func() { w.Close() })
		return w
	case <-time.After(5 * time.Second):
		t.Fatal("no export copies the slow track")
		return nil
	}
}

// endCopy writes the content of a slow track and ends it.
func endCopy(t *testing.T, w *os.File, content string) {
	t.Helper()
	if _, err := w.WriteString(content); err != nil {
		t.Fatalf("write the slow track: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close the slow track: %v", err)
	}
}

// waitExport returns the end of export id, skipping the end of others.
func waitExport(t *testing.T, c *Core, id int64) ExportFinished {
	t.Helper()
	for {
		if e := nextEvent[ExportFinished](t, c); e.ID == id {
			return e
		}
	}
}

func TestExport_SeveralRunAtOnce(t *testing.T) {
	c, ids := newExportCore(t, Options{})
	ex := c.Exports()
	slow := slowTrack(t, "Canada", 2)
	slowDest, dest := t.TempDir(), t.TempDir()

	first, err := ex.Start(ExportRequest{TargetID: folderTarget(t, c, slowDest), Tracks: []ExportTrack{slow}})
	if err != nil {
		t.Fatalf("start the first export: %v", err)
	}
	w := feed(t, &slow) // the first export is on its copy, and stays there

	tracks := ex.Tracks(ids)
	second, err := ex.Start(ExportRequest{TargetID: folderTarget(t, c, dest), Tracks: tracks})
	if err != nil {
		t.Fatalf("start a second export while the first runs: %v", err)
	}
	if second == first {
		t.Fatalf("both exports are export %d", first)
	}

	// The second one ends while the first is still copying.
	if finished := waitExport(t, c, second); finished.Done != 3 || len(finished.Failed) != 0 || finished.Err != nil {
		t.Errorf("second ExportFinished = %+v, want its three tracks exported", finished)
	}
	if !exported(dest, &tracks[2], "03 - Candy Girl.mp3") {
		t.Error("the second export did not write its last track")
	}

	endCopy(t, w, "canada")
	if finished := waitExport(t, c, first); finished.Done != 1 || len(finished.Failed) != 0 || finished.Err != nil {
		t.Errorf("first ExportFinished = %+v, want its track exported", finished)
	}
	if got, err := os.ReadFile(filepath.Join(slowDest, "Low - Trust", "02 - Canada.mp3")); err != nil || string(got) != "canada" {
		t.Errorf("the first export wrote %q (err %v), want its track whole", got, err)
	}
}

// Close ends an export after the copy it is on, and waits for it: the file
// being written is whole, the next one is not begun, nothing is reported.
func TestClose_EndsAnExportAfterTheCopyItIsOn(t *testing.T) {
	c, ids := newExportCore(t, Options{})
	ex := c.Exports()
	inLibrary := ex.Tracks(ids)
	slow := slowTrack(t, "Canada", 2)
	dest := t.TempDir()

	_, err := ex.Start(ExportRequest{TargetID: folderTarget(t, c, dest), Tracks: []ExportTrack{inLibrary[0], slow, inLibrary[2]}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	w := feed(t, &slow) // the export is copying its second track

	closed := make(chan struct{})
	go func() {
		c.Close()
		close(closed)
	}()
	eventually(t, "Close has begun", c.stopping)
	select {
	case <-closed:
		t.Fatal("Close did not wait for the copy in flight")
	case <-time.After(quiet):
	}

	endCopy(t, w, "canada")
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close still waits after the copy ended")
	}

	if got, err := os.ReadFile(filepath.Join(dest, "Low - Trust", "02 - Canada.mp3")); err != nil || string(got) != "canada" {
		t.Errorf("the track being copied = %q (err %v), want it whole", got, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Low - Trust", "03 - Candy Girl.mp3")); !os.IsNotExist(err) {
		t.Errorf("the track after it was exported: %v", err)
	}
	// Cut short by Close is not an export that ended.
	expectStreamEndsWithout[ExportFinished](t, c)
}
