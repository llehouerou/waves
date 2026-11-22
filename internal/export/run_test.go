package export

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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

// files are the paths of the files under dir, relative to it, with "/".
func files(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		found = append(found, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return found
}

// An album artist and an album tagged ".." name folders under the root, not
// the folders above it.
func TestRun_NeverWritesAboveItsRoot(t *testing.T) {
	top := t.TempDir()
	root := filepath.Join(top, "mnt", "usb", "Music")
	src := writeSource(t, "x.mp3", "x")
	batch := Batch{
		Tracks:    []Track{{SrcPath: src, Artist: "..", Album: "..", Title: "x", TrackNum: 1, Extension: ".mp3"}},
		Root:      root,
		Structure: FolderStructureHierarchical,
	}

	failed, done, err := batch.Run(context.Background(), nil, nil)

	if err != nil || len(failed) != 0 || done != 1 {
		t.Fatalf("Run = %v failed, %d done, err %v; want none failed, 1 done", failed, done, err)
	}
	want := []string{"mnt/usb/Music/_./_./01 - x.mp3"}
	if got := files(t, top); !slices.Equal(got, want) {
		t.Errorf("files written = %q, want %q", got, want)
	}
	if got, err := os.ReadFile(filepath.Join(root, "_.", "_.", "01 - x.mp3")); err != nil || string(got) != "x" {
		t.Errorf("exported file = %q (err %v), want the source as it is", got, err)
	}
}

// Whatever its tags, a track is exported under a name that is not hidden, in
// every layout.
func TestRun_WritesNoHiddenName(t *testing.T) {
	names := []string{".", "..", "...", ".38 Special", ".hack", "./..", "../x"}
	for _, structure := range []FolderStructure{FolderStructureHierarchical, FolderStructureFlat, FolderStructureSingle} {
		t.Run(string(structure), func(t *testing.T) {
			// Some levels below the folder walked: a name that climbs out of
			// the root still lands where the walk sees it.
			top := t.TempDir()
			root := filepath.Join(top, "mnt", "usb", "Music")
			tracks := make([]Track, 0, 3*len(names))
			for i, name := range names {
				src := writeSource(t, "src.mp3", name)
				tracks = append(tracks,
					Track{SrcPath: src, Artist: name, Album: "A", Title: "t", TrackNum: i + 1, Extension: ".mp3"},
					Track{SrcPath: src, Artist: "A", Album: name, Title: "t", TrackNum: i + 1, Extension: ".mp3"},
					Track{SrcPath: src, Artist: name, Album: name, Title: name, TrackNum: i + 1, Extension: ".mp3"},
				)
			}
			batch := Batch{Tracks: tracks, Root: root, Structure: structure}

			failed, done, err := batch.Run(context.Background(), nil, nil)

			if err != nil || len(failed) != 0 || done != len(tracks) {
				t.Fatalf("Run = %v failed, %d done, err %v; want none failed, %d done", failed, done, err, len(tracks))
			}
			err = filepath.WalkDir(top, func(path string, d fs.DirEntry, err error) error {
				if err == nil && path != top && strings.HasPrefix(d.Name(), ".") {
					t.Errorf("hidden name written: %s", path)
				}
				return err
			})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			for _, file := range files(t, top) {
				if !strings.HasPrefix(file, "mnt/usb/Music/") {
					t.Errorf("file written outside the root: %s", file)
				}
			}
		})
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
// what it wrote is not left behind, under the track's name or another.
func TestRun_AConversionIsKilledWhenItsContextEnds(t *testing.T) {
	flac, err := os.ReadFile(writeFLAC(t))
	if err != nil {
		t.Fatalf("read the FLAC: %v", err)
	}
	src := makeFIFO(t, "canada.flac")
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ended := startRun(ctx, canadaFLAC(src, root))
	// Half the FLAC: ffmpeg writes its MP3, and waits for the rest.
	w := openFIFOWriter(t, src)
	feedFIFO(t, w, flac[:len(flac)/2])
	waitWriting(t, root, 0)

	cancel()

	select {
	case got := <-ended:
		if !errors.Is(got.err, context.Canceled) || got.done != 0 || len(got.failed) != 0 {
			t.Errorf("Run = %+v failed, %d done, err %v; want the export canceled, and no track failed", got.failed, got.done, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run still waits for ffmpeg after its context ended")
	}
	if got := files(t, root); len(got) != 0 {
		t.Errorf("files left = %q, want none: a partial MP3 is not left", got)
	}
}

// makeFIFO makes a file that is a FIFO: a track whose file it is lasts for as
// long as the test holds its other end (open).
func makeFIFO(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	return path
}

// openFIFOWriter waits for an export to read the FIFO at path, and returns the end the
// test writes it at: the export reads it until it is closed.
func openFIFOWriter(t *testing.T, path string) *os.File {
	t.Helper()
	opened := make(chan *os.File, 1)
	go func() {
		// Opening this end returns once the export has opened the other.
		w, _ := os.OpenFile(path, os.O_WRONLY, 0) //nolint:errcheck // a nil file fails the test below
		opened <- w
	}()
	select {
	case w := <-opened:
		if w == nil {
			t.Fatal("cannot open the FIFO")
		}
		t.Cleanup(func() { w.Close() }) // an export that still reads it gets its end of file
		return w
	case <-time.After(5 * time.Second):
		t.Fatal("no export reads the FIFO")
		return nil
	}
}

// feedFIFO writes data at w, the end of a FIFO an export reads.
func feedFIFO(t *testing.T, w *os.File, data []byte) {
	t.Helper()
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write the FIFO: %v", err)
	}
}

// waitWriting waits until a file under dir holds at least size bytes: the
// export writes it.
func waitWriting(t *testing.T, dir string, size int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, name := range files(t, dir) {
			if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.Size() >= size {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no file of %d bytes written under %s: %q", size, dir, files(t, dir))
}

type runResult struct {
	failed []TrackError
	done   int
	err    error
}

// startRun runs batch in the background, and returns where its result comes.
func startRun(ctx context.Context, batch Batch) <-chan runResult {
	ended := make(chan runResult, 1)
	go func() {
		failed, done, err := batch.Run(ctx, nil, nil)
		ended <- runResult{failed, done, err}
	}()
	return ended
}

// waitDone waits for the result of a batch started, and fails the test unless
// its track was done.
func waitDone(t *testing.T, ended <-chan runResult) {
	t.Helper()
	select {
	case got := <-ended:
		if got.err != nil || len(got.failed) != 0 || got.done != 1 {
			t.Errorf("Run = %+v failed, %d done, err %v; want none failed, 1 done", got.failed, got.done, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run never ended")
	}
}

// noHiddenName fails the test on a name under dir that starts with a dot.
func noHiddenName(t *testing.T, dir string) {
	t.Helper()
	for _, name := range files(t, dir) {
		for part := range strings.SplitSeq(name, "/") {
			if strings.HasPrefix(part, ".") {
				t.Errorf("hidden name written: %s", name)
			}
		}
	}
}

// A copy cut short must leave nothing a later export takes for the track:
// the file gets its name once it is whole.
func TestRun_ACopyIsNamedOnceWhole(t *testing.T) {
	src := makeFIFO(t, "canada.mp3")
	root := t.TempDir()
	batch := Batch{Tracks: trust(src, src)[1:], Root: root, Structure: FolderStructureFlat}
	final := filepath.Join(root, "Low - Trust", "02 - Canada.mp3")

	ended := startRun(context.Background(), batch)
	w := openFIFOWriter(t, src)
	feedFIFO(t, w, []byte("can"))
	waitWriting(t, root, 3)

	if _, err := os.Stat(final); !os.IsNotExist(err) {
		t.Errorf("the track has its final name while it is copied: %v", err)
	}
	noHiddenName(t, root)

	feedFIFO(t, w, []byte("ada"))
	w.Close()
	waitDone(t, ended)

	if got, err := os.ReadFile(final); err != nil || string(got) != "canada" {
		t.Errorf("exported file = %q (err %v), want the whole source", got, err)
	}
	if got, want := files(t, root), []string{"Low - Trust/02 - Canada.mp3"}; !slices.Equal(got, want) {
		t.Errorf("files written = %q, want %q", got, want)
	}
}

// writeFLAC writes a FLAC of ten seconds of noise, and returns its path.
func writeFLAC(t *testing.T) string {
	t.Helper()
	needFFmpeg(t)
	src := filepath.Join(t.TempDir(), "canada.flac")
	out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "anoisesrc=duration=10", "-c:a", "flac", src).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg cannot write a FLAC: %v\n%s", err, out)
	}
	return src
}

// validMP3 fails the test unless path is an MP3 ffmpeg decodes whole.
func validMP3(t *testing.T, path string) {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=format_name", "-of", "default=nw=1:nk=1", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "mp3" {
		t.Errorf("%s is not an MP3: %v\n%s", path, err, out)
	}
	out, err = exec.Command("ffmpeg", "-v", "error", "-i", path, "-f", "null", "-").CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Errorf("%s does not decode: %v\n%s", path, err, out)
	}
}

// canadaFLAC is the second track of Low's Trust, a FLAC at src converted to
// an MP3 under root.
func canadaFLAC(src, root string) Batch {
	return Batch{
		Tracks:      []Track{{SrcPath: src, Artist: "Low", Album: "Trust", Title: "Canada", TrackNum: 2, Extension: ".flac"}},
		Root:        root,
		Structure:   FolderStructureHierarchical,
		ConvertFLAC: true,
	}
}

func TestRun_AConversionIsNamedOnceWhole(t *testing.T) {
	flac, err := os.ReadFile(writeFLAC(t))
	if err != nil {
		t.Fatalf("read the FLAC: %v", err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skipf("ffprobe not available: %v", err)
	}
	src := makeFIFO(t, "canada.flac")
	root := t.TempDir()
	final := filepath.Join(root, "Low", "Trust", "02 - Canada.mp3")

	ended := startRun(context.Background(), canadaFLAC(src, root))
	w := openFIFOWriter(t, src)
	feedFIFO(t, w, flac[:len(flac)/2])
	waitWriting(t, root, 0)

	if _, err := os.Stat(final); !os.IsNotExist(err) {
		t.Errorf("the track has its final name while it is converted: %v", err)
	}
	noHiddenName(t, root)

	feedFIFO(t, w, flac[len(flac)/2:])
	w.Close()
	waitDone(t, ended)

	validMP3(t, final)
	if got, want := files(t, root), []string{"Low/Trust/02 - Canada.mp3"}; !slices.Equal(got, want) {
		t.Errorf("files written = %q, want %q", got, want)
	}
}

// What a write cut short leaves (waves killed, the device pulled out) is
// written again by the next export of the track, and not left beside it.
func TestRun_WritesAgainWhatAWriteCutShortLeft(t *testing.T) {
	t.Run("copy", func(t *testing.T) {
		root := t.TempDir()
		folder := filepath.Join(root, "Low - Trust")
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "02 - Canada.mp3.part"), []byte("can"), 0o600); err != nil {
			t.Fatal(err)
		}
		src := writeSource(t, "canada.mp3", "canada")
		batch := Batch{Tracks: trust(src, src)[1:], Root: root, Structure: FolderStructureFlat}

		waitDone(t, startRun(context.Background(), batch))

		if got, err := os.ReadFile(filepath.Join(folder, "02 - Canada.mp3")); err != nil || string(got) != "canada" {
			t.Errorf("exported file = %q (err %v), want the whole source", got, err)
		}
		if got, want := files(t, root), []string{"Low - Trust/02 - Canada.mp3"}; !slices.Equal(got, want) {
			t.Errorf("files written = %q, want %q", got, want)
		}
	})

	t.Run("conversion", func(t *testing.T) {
		src := writeFLAC(t)
		if _, err := exec.LookPath("ffprobe"); err != nil {
			t.Skipf("ffprobe not available: %v", err)
		}
		root := t.TempDir()
		folder := filepath.Join(root, "Low", "Trust")
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "02 - Canada.mp3.part"), []byte("half an mp3"), 0o600); err != nil {
			t.Fatal(err)
		}

		waitDone(t, startRun(context.Background(), canadaFLAC(src, root)))

		validMP3(t, filepath.Join(folder, "02 - Canada.mp3"))
		if got, want := files(t, root), []string{"Low/Trust/02 - Canada.mp3"}; !slices.Equal(got, want) {
			t.Errorf("files written = %q, want %q", got, want)
		}
	})
}

// A copy that fails once its destination is open leaves no file, under the
// final name or another.
func TestRun_ACopyThatFailsLeavesNoFile(t *testing.T) {
	src := t.TempDir() // a directory: opens, but cannot be read as a file
	root := t.TempDir()
	batch := Batch{Tracks: trust(src, src)[1:], Root: root, Structure: FolderStructureFlat}

	failed, done, err := batch.Run(context.Background(), nil, nil)

	if err != nil || done != 1 || len(failed) != 1 {
		t.Fatalf("Run = %+v failed, %d done, err %v; want the track failed", failed, done, err)
	}
	if got := files(t, root); len(got) != 0 {
		t.Errorf("files left = %q, want none", got)
	}
}

// A file already under the final name of a track is left as it is, whatever
// it holds, and the track counts as done.
func TestRun_LeavesAFileAlreadyThere(t *testing.T) {
	for _, convert := range []bool{false, true} {
		t.Run(fmt.Sprintf("convert=%v", convert), func(t *testing.T) {
			root := t.TempDir()
			final := filepath.Join(root, "Low", "Trust", "02 - Canada.mp3")
			if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(final, []byte("cut short"), 0o600); err != nil {
				t.Fatal(err)
			}
			batch := canadaFLAC(writeSource(t, "canada.flac", "not even a FLAC"), root)
			batch.ConvertFLAC = convert
			if !convert {
				batch.Tracks[0].Extension = ".mp3"
			}

			waitDone(t, startRun(context.Background(), batch))

			if got, err := os.ReadFile(final); err != nil || string(got) != "cut short" {
				t.Errorf("file already there = %q (err %v), want it untouched", got, err)
			}
			if got, want := files(t, root), []string{"Low/Trust/02 - Canada.mp3"}; !slices.Equal(got, want) {
				t.Errorf("files = %q, want %q", got, want)
			}
		})
	}
}

// Two exports that reach the same file at once write it one after the other:
// it ends whole, and neither fails the track.
func TestRun_TwoExportsOfTheSameTrackAtOnce(t *testing.T) {
	root := t.TempDir()
	slow := makeFIFO(t, "canada.mp3")
	first := Batch{Tracks: trust(slow, slow)[1:], Root: root, Structure: FolderStructureFlat}
	fast := writeSource(t, "canada.mp3", "canada")
	second := Batch{Tracks: trust(fast, fast)[1:], Root: root, Structure: FolderStructureFlat}

	firstEnded := startRun(context.Background(), first)
	w := openFIFOWriter(t, slow)
	feedFIFO(t, w, []byte("can"))
	waitWriting(t, root, 3)

	secondEnded := startRun(context.Background(), second)
	// Time for the second one to write the file while the first one does, or
	// to wait for it.
	time.Sleep(200 * time.Millisecond)

	feedFIFO(t, w, []byte("ada"))
	w.Close()
	waitDone(t, firstEnded)
	waitDone(t, secondEnded)

	if got, err := os.ReadFile(filepath.Join(root, "Low - Trust", "02 - Canada.mp3")); err != nil || string(got) != "canada" {
		t.Errorf("exported file = %q (err %v), want it whole", got, err)
	}
	if got, want := files(t, root), []string{"Low - Trust/02 - Canada.mp3"}; !slices.Equal(got, want) {
		t.Errorf("files written = %q, want %q", got, want)
	}
}
