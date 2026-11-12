package export

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"
)

// writeSource writes a file to export, and returns its path.
func writeSource(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return path
}

// trust is the first two tracks of Low's Trust, to export from src1 and src2.
func trust(src1, src2 string) []Track {
	return []Track{
		{SrcPath: src1, Artist: "Low", Album: "Trust", Title: "Amazing Grace", TrackNum: 1, Extension: ".mp3"},
		{SrcPath: src2, Artist: "Low", Album: "Trust", Title: "Canada", TrackNum: 2, Extension: ".mp3"},
	}
}

func TestRun_ExportsEachTrackToWhereItsNamesSay(t *testing.T) {
	root := t.TempDir()
	batch := Batch{
		Tracks:    trust(writeSource(t, "a.mp3", "amazing grace"), writeSource(t, "b.mp3", "canada")),
		Root:      root,
		Structure: FolderStructureFlat,
	}

	var reported []int
	failed, done, err := batch.Run(context.Background(), func(done int) { reported = append(reported, done) }, nil)

	if err != nil || len(failed) != 0 || done != 2 {
		t.Fatalf("Run = %v failed, %d done, err %v; want none failed, 2 done", failed, done, err)
	}
	for name, want := range map[string]string{
		"Low - Trust/01 - Amazing Grace.mp3": "amazing grace",
		"Low - Trust/02 - Canada.mp3":        "canada",
	} {
		if got, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(got) != want {
			t.Errorf("%s = %q (err %v), want %q", name, got, err, want)
		}
	}
	// Told before each track how many are done.
	if !slices.Equal(reported, []int{0, 1}) {
		t.Errorf("reported %v, want [0 1]", reported)
	}
}

func TestRun_ATrackThatFailsDoesNotStopTheOthers(t *testing.T) {
	root := t.TempDir()
	gone := filepath.Join(t.TempDir(), "gone.mp3")
	batch := Batch{Tracks: trust(gone, writeSource(t, "b.mp3", "canada")), Root: root, Structure: FolderStructureFlat}

	failed, done, err := batch.Run(context.Background(), nil, nil)

	if err != nil || done != 2 {
		t.Fatalf("Run = %d done, err %v; want both tracks reached and no error", done, err)
	}
	if len(failed) != 1 || failed[0].Track.Title != "Amazing Grace" || failed[0].Err == nil {
		t.Errorf("failed = %+v, want Amazing Grace and why", failed)
	}
	if _, err := os.Stat(filepath.Join(root, "Low - Trust", "02 - Canada.mp3")); err != nil {
		t.Errorf("the track after the one that failed was not exported: %v", err)
	}
}

// The core asks before each track whether the device is still there.
func TestRun_StopsWhereItIsToldTo(t *testing.T) {
	root := t.TempDir()
	batch := Batch{
		Tracks:    trust(writeSource(t, "a.mp3", "amazing grace"), writeSource(t, "b.mp3", "canada")),
		Root:      root,
		Structure: FolderStructureFlat,
	}
	unplugged := errors.New("unplugged")
	asked := 0

	_, done, err := batch.Run(context.Background(), nil, func() error {
		asked++
		if asked == 2 {
			return unplugged
		}
		return nil
	})

	if !errors.Is(err, unplugged) || done != 1 {
		t.Errorf("Run = %d done, err %v; want stopped after the first track, told why", done, err)
	}
	if _, err := os.Stat(filepath.Join(root, "Low - Trust", "02 - Canada.mp3")); !os.IsNotExist(err) {
		t.Errorf("the track after the stop was exported: %v", err)
	}
}

// Quitting: the copy it is on is not cut, the next track is not begun.
func TestRun_EndsBeforeTheNextTrackWhenItsContextEnds(t *testing.T) {
	root := t.TempDir()
	batch := Batch{
		Tracks:    trust(writeSource(t, "a.mp3", "amazing grace"), writeSource(t, "b.mp3", "canada")),
		Root:      root,
		Structure: FolderStructureFlat,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ended while the first track is about to be copied.
	failed, done, err := batch.Run(ctx, func(int) { cancel() }, nil)

	if !errors.Is(err, context.Canceled) || done != 1 || len(failed) != 0 {
		t.Errorf("Run = %v failed, %d done, err %v; want the first track done and the export canceled", failed, done, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "Low - Trust", "01 - Amazing Grace.mp3")); err != nil || string(got) != "amazing grace" {
		t.Errorf("the track being copied = %q (err %v), want it whole", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "Low - Trust", "02 - Canada.mp3")); !os.IsNotExist(err) {
		t.Errorf("the next track was exported: %v", err)
	}
}

// needFFmpeg skips a test about the conversion where there is no ffmpeg.
func needFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
}

func TestRun_ConvertsFLACToMP3(t *testing.T) {
	needFFmpeg(t)
	src := filepath.Join(t.TempDir(), "canada.flac")
	if out, err := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "flac", src).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot write a FLAC: %v\n%s", err, out)
	}
	root := t.TempDir()
	batch := Batch{
		Tracks:      []Track{{SrcPath: src, Artist: "Low", Album: "Trust", Title: "Canada", TrackNum: 2, Extension: ".flac"}},
		Root:        root,
		Structure:   FolderStructureHierarchical,
		ConvertFLAC: true,
	}

	failed, _, err := batch.Run(context.Background(), nil, nil)

	if err != nil || len(failed) != 0 {
		t.Fatalf("Run = %+v failed, err %v", failed, err)
	}
	info, err := os.Stat(filepath.Join(root, "Low", "Trust", "02 - Canada.mp3"))
	if err != nil || info.Size() == 0 {
		t.Errorf("the MP3 = %+v (err %v), want it written", info, err)
	}
	if _, err := os.Stat(filepath.Join(root, "Low", "Trust", "02 - Canada.flac")); !os.IsNotExist(err) {
		t.Errorf("the FLAC was exported too: %v", err)
	}
}

// Quitting must not wait for a track to be encoded: ffmpeg is killed, and
// what it wrote is not left behind for a later export to take for the track.
func TestRun_AConversionIsKilledWhenItsContextEnds(t *testing.T) {
	needFFmpeg(t)
	// A FLAC that never comes: ffmpeg waits for it for as long as it lives.
	src := filepath.Join(t.TempDir(), "canada.flac")
	if err := syscall.Mkfifo(src, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	root := t.TempDir()
	batch := Batch{
		Tracks:      []Track{{SrcPath: src, Artist: "Low", Album: "Trust", Title: "Canada", TrackNum: 2, Extension: ".flac"}},
		Root:        root,
		Structure:   FolderStructureHierarchical,
		ConvertFLAC: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		failed []TrackError
		done   int
		err    error
	}
	ended := make(chan result, 1)
	go func() {
		failed, done, err := batch.Run(ctx, nil, nil)
		ended <- result{failed, done, err}
	}()

	// Opening the other end returns once ffmpeg reads the file.
	reading := make(chan *os.File, 1)
	go func() {
		w, _ := os.OpenFile(src, os.O_WRONLY, 0) //nolint:errcheck // a nil file fails the test below
		reading <- w
	}()
	select {
	case w := <-reading:
		if w == nil {
			t.Fatal("cannot open the FLAC ffmpeg reads")
		}
		defer w.Close() // ffmpeg gets its end of file, were it still there
	case <-time.After(5 * time.Second):
		t.Fatal("ffmpeg never read the FLAC")
	}

	cancel()

	select {
	case got := <-ended:
		if !errors.Is(got.err, context.Canceled) || got.done != 0 || len(got.failed) != 0 {
			t.Errorf("Run = %+v failed, %d done, err %v; want the export canceled, and no track failed", got.failed, got.done, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run still waits for ffmpeg after its context ended")
	}
	if _, err := os.Stat(filepath.Join(root, "Low", "Trust", "02 - Canada.mp3")); !os.IsNotExist(err) {
		t.Errorf("a partial MP3 was left: %v", err)
	}
}
