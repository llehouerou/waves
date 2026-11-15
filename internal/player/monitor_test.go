package player

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gopxl/beep/v2/speaker"
	"github.com/stretchr/testify/assert"

	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// These tests play real MP3s with no audio device: openTrack takes the speaker
// as initialised, and nothing pulls samples from it, so a track stays at its
// start, 26 ms from its end, and the monitor preloads at each of its ticks.
// They change the package's speaker globals, so they never run in parallel.

// monitorPlayer returns a player whose monitor runs over the test's tracks,
// stopped when the test ends.
func monitorPlayer(t *testing.T) *Player {
	t.Helper()
	initialized, rate := speakerInitialized, speakerSampleRate
	speakerInitialized, speakerSampleRate = true, 44100
	p := newPlayer()
	t.Cleanup(func() {
		p.Stop()
		if !waitMonitors(0) {
			t.Errorf("%d monitors still running after the test", monitors())
		}
		speakerInitialized, speakerSampleRate = initialized, rate
	})
	return p
}

// track writes a one-frame MP3 in dir and returns its path.
func track(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name+".mp3")
	tagstest.WriteMP3(t, path, tags.Tag{Title: name})
	return path
}

// monitors counts the monitor goroutines of every player.
func monitors() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return bytes.Count(buf[:n], []byte(").monitorLoop("))
}

// waitMonitors waits up to 3 seconds for n monitors to be running.
func waitMonitors(n int) bool {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if monitors() == n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// openFiles counts the descriptors this process holds on path.
func openFiles(t *testing.T, path string) int {
	t.Helper()
	const fds = "/proc/self/fd"
	entries, err := os.ReadDir(fds)
	if err != nil {
		t.Skipf("no /proc/self/fd: %v", err)
	}
	count := 0
	for _, e := range entries {
		target, err := os.Readlink(fds + "/" + e.Name())
		if err == nil && target == path {
			count++
		}
	}
	return count
}

// returnsWhileBlocked runs fn, which must return while the preload function is
// still blocked: Stop and Play never wait for the monitor.
func returnsWhileBlocked(t *testing.T, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s waits for the monitor blocked in the preload function", name)
	}
}

// goroutineID returns the ID of the calling goroutine, to tell monitors apart.
func goroutineID() uint64 {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	buf = bytes.TrimPrefix(buf, []byte("goroutine "))
	id, _ := strconv.ParseUint(string(buf[:bytes.IndexByte(buf, ' ')]), 10, 64)
	return id
}

// preloadGate is a preload function whose first call blocks until the test
// releases it, standing for a slow open of the next track. It returns first
// from that call, and then from every later one.
type preloadGate struct {
	first, then string
	entered     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once

	mu       sync.Mutex
	calls    int
	released bool
	after    []uint64 // the goroutine of each call that began after the release
}

func newPreloadGate(t *testing.T, first, then string) *preloadGate {
	g := &preloadGate{
		first:   first,
		then:    then,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	t.Cleanup(g.Release) // a failed test must not leave a monitor blocked
	return g
}

func (g *preloadGate) fn() string {
	g.mu.Lock()
	g.calls++
	if g.calls == 1 {
		g.mu.Unlock()
		close(g.entered)
		<-g.release
		return g.first
	}
	if g.released {
		g.after = append(g.after, goroutineID())
	}
	g.mu.Unlock()
	return g.then
}

func (g *preloadGate) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-g.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the monitor never called the preload function")
	}
}

// Release lets the blocked call return.
func (g *preloadGate) Release() {
	g.releaseOnce.Do(func() {
		g.mu.Lock()
		g.released = true
		g.mu.Unlock()
		close(g.release)
	})
}

func (g *preloadGate) laterCalls() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls - 1
}

// waitCallsAfterRelease waits up to 5 seconds for n calls after the release,
// and returns how many goroutines made them.
func (g *preloadGate) waitCallsAfterRelease(t *testing.T, n int) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		if len(g.after) >= n {
			callers := map[uint64]bool{}
			for _, id := range g.after {
				callers[id] = true
			}
			g.mu.Unlock()
			return len(callers)
		}
		g.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fewer than %d preload calls after the release", n)
	return 0
}

// stopDuringPreload plays a track, stops the player while its monitor is
// blocked in the preload function, then lets the call return the next track.
func stopDuringPreload(t *testing.T) (p *Player, g *preloadGate, next string) {
	t.Helper()
	dir := t.TempDir()
	current, next := track(t, dir, "current"), track(t, dir, "next")
	p = monitorPlayer(t)
	g = newPreloadGate(t, next, "")
	p.SetPreloadFunc(g.fn)

	if err := p.Play(current); err != nil {
		t.Fatalf("play: %v", err)
	}
	g.waitEntered(t)
	returnsWhileBlocked(t, "Stop", p.Stop)
	g.Release()
	return p, g, next
}

// playDuringPreload plays a track, gives the player another one while the
// monitor of the first is blocked in the preload function, then lets the call
// return preloaded. Later calls return no track.
func playDuringPreload(t *testing.T, preloaded func(second string) string) (p *Player, g *preloadGate, second string) {
	t.Helper()
	dir := t.TempDir()
	first, second := track(t, dir, "first"), track(t, dir, "second")
	p = monitorPlayer(t)
	g = newPreloadGate(t, preloaded(second), "")
	p.SetPreloadFunc(g.fn)

	if err := p.Play(first); err != nil {
		t.Fatalf("play: %v", err)
	}
	g.waitEntered(t)
	var err error
	returnsWhileBlocked(t, "Play", func() { err = p.Play(second) })
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	g.Release()
	return p, g, second
}

func TestStopEndsTheMonitorBlockedInThePreload(t *testing.T) {
	_, g, _ := stopDuringPreload(t)

	assert.True(t, waitMonitors(0), "the monitor still runs after Stop")
	time.Sleep(1200 * time.Millisecond) // two ticks
	assert.Zero(t, g.laterCalls(), "the preload function was called after Stop")
}

func TestPreloadCompletingAfterStopIsDiscarded(t *testing.T) {
	p, _, next := stopDuringPreload(t)

	waitMonitors(0) // the monitor returns once it has discarded its preload
	assert.Nil(t, p.next, "the preload landed on a stopped player")
	assert.Zero(t, openFiles(t, next), "the track the stale preload opened is still open")
}

func TestPlayDuringPreloadLeavesOneMonitor(t *testing.T) {
	_, g, _ := playDuringPreload(t, func(string) string { return "" })

	assert.Equal(t, 1, g.waitCallsAfterRelease(t, 4),
		"the monitor of the replaced track still calls the preload function")
	assert.True(t, waitMonitors(1), "%d monitors run for one Play", monitors())
}

// The stale preload opens the very track now playing: the run it was
// started for is what decides, not the path.
func TestPreloadCompletingAfterANewTrackIsDiscarded(t *testing.T) {
	p, _, second := playDuringPreload(t, func(second string) string { return second })

	waitMonitors(1) // the stale monitor returns once it has discarded its preload
	speaker.Lock()
	next, hasNext := p.next, p.gapless.HasNext()
	speaker.Unlock()
	assert.Nil(t, next, "the stale preload landed on the new run")
	assert.False(t, hasNext, "the new run is followed by the stale preload")
	assert.Equal(t, 1, openFiles(t, second),
		"the track the stale preload opened is still open next to the playing one")
}

func TestPreloadDuringItsPlaybackIsKept(t *testing.T) {
	dir := t.TempDir()
	current, next := track(t, dir, "current"), track(t, dir, "next")
	p := monitorPlayer(t)
	p.SetPreloadFunc(func() string { return next })

	if err := p.Play(current); err != nil {
		t.Fatalf("play: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	var preloaded *trackState
	var hasNext bool
	for preloaded == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		speaker.Lock()
		preloaded, hasNext = p.next, p.gapless.HasNext()
		speaker.Unlock()
	}
	if preloaded == nil {
		t.Fatal("the next track was never preloaded")
	}
	assert.Equal(t, next, preloaded.trackInfo.Path)
	assert.True(t, hasNext, "the gapless streamer has no next track")
}

// preloaded waits up to 5 seconds for a track to be preloaded, failing the
// test if one ever lands from stale. It returns the preloaded track's path and
// whether the gapless streamer has that track as its next streamer.
func preloaded(t *testing.T, p *Player, stale string) (path string, gaplessNext bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		speaker.Lock()
		next := p.next
		if next != nil {
			p.gapless.mu.Lock()
			gaplessNext = p.gapless.next == next.resampled
			p.gapless.mu.Unlock()
		}
		speaker.Unlock()
		if next != nil {
			if next.trackInfo.Path == stale {
				t.Fatal("the preload dropped while in flight landed")
			}
			return next.trackInfo.Path, gaplessNext
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no track was ever preloaded")
	return "", false
}

func TestClearPreloadDropsThePreloadInFlight(t *testing.T) {
	dir := t.TempDir()
	current := track(t, dir, "current")
	stale, fresh := track(t, dir, "stale"), track(t, dir, "fresh")
	p := monitorPlayer(t)
	g := newPreloadGate(t, stale, fresh)
	p.SetPreloadFunc(g.fn)

	if err := p.Play(current); err != nil {
		t.Fatalf("play: %v", err)
	}
	g.waitEntered(t)
	returnsWhileBlocked(t, "ClearPreload", p.ClearPreload)
	g.Release()

	path, gaplessNext := preloaded(t, p, stale)
	assert.Equal(t, fresh, path, "the monitor did not preload again after the drop")
	assert.True(t, gaplessNext, "the gapless streamer is not followed by the preloaded track")
	assert.Zero(t, openFiles(t, stale), "the track the dropped preload opened is still open")
	assert.True(t, waitMonitors(1), "%d monitors run for one Play", monitors())
}

func TestClearPreloadClearsAPreloadLanded(t *testing.T) {
	dir := t.TempDir()
	current, next := track(t, dir, "current"), track(t, dir, "next")
	p := monitorPlayer(t)
	var once sync.Once
	p.SetPreloadFunc(func() string { // the next track once, then none
		path := ""
		once.Do(func() { path = next })
		return path
	})

	if err := p.Play(current); err != nil {
		t.Fatalf("play: %v", err)
	}
	preloaded(t, p, "")
	p.ClearPreload()

	speaker.Lock()
	cleared, hasNext := p.next, p.gapless.HasNext()
	speaker.Unlock()
	assert.Nil(t, cleared, "the preloaded track is still the next one")
	assert.False(t, hasNext, "the gapless streamer still has a next track")
}
