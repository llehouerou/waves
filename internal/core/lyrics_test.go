package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/lrclib"
)

// fakeLrclib is the lrclib of these tests: it has the synced lyrics it is
// given, by track title, and counts the times it is asked.
type fakeLrclib struct {
	synced map[string]string
	err    error // what it answers instead, when set
	asked  atomic.Int32
}

func (f *fakeLrclib) Get(_ context.Context, artist, title string, _ time.Duration) (*lrclib.LyricsResult, error) {
	f.asked.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	text, ok := f.synced[title]
	if !ok {
		return nil, lrclib.ErrNotFound
	}
	return &lrclib.LyricsResult{ArtistName: artist, TrackName: title, SyncedLyrics: text}, nil
}

// lineTexts are the lines of lyrics, without their times.
func lineTexts(lyrics *TrackLyrics) []string {
	texts := make([]string, len(lyrics.Lines))
	for i, line := range lyrics.Lines {
		texts[i] = line.Text
	}
	return texts
}

// A .lrc file next to the track is the user's own: nothing else is asked.
func TestLyrics_TheFileNextToTheTrackWins(t *testing.T) {
	lrc := &fakeLrclib{synced: map[string]string{"Canada": "[00:01.00]from lrclib"}}
	c, _ := newTestCore(t, Options{lrclib: lrc})
	track := filepath.Join(t.TempDir(), "02 - Canada.mp3")
	if err := os.WriteFile(filepath.Join(filepath.Dir(track), "02 - Canada.lrc"), []byte("[00:12.50]You can't get away\n[00:20.00]From Canada\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	lyrics, err := c.Lyrics().Fetch(LyricsTrack{FilePath: track, Artist: "Low", Title: "Canada"})

	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(lyrics.Lines) != 2 || lyrics.Lines[0].Text != "You can't get away" || lyrics.Lines[0].Time != 12500*time.Millisecond {
		t.Errorf("lyrics = %v, want the two lines of the file, timed", lyrics.Lines)
	}
	if n := lrc.asked.Load(); n != 0 {
		t.Errorf("lrclib was asked %d times, want it left alone", n)
	}
}

// canada is a track with no .lrc file next to it.
var canada = LyricsTrack{FilePath: "/music/Low/Trust/02 - Canada.mp3", Artist: "Low", Title: "Canada", Duration: 224 * time.Second}

func TestLyrics_ComeFromLrclibOnceThenFromTheCache(t *testing.T) {
	lrc := &fakeLrclib{synced: map[string]string{"Canada": "[00:12.50]You can't get away\n[00:20.00]From Canada\n"}}
	c, _ := newTestCore(t, Options{lrclib: lrc, LyricsCache: t.TempDir()})
	want := []string{"You can't get away", "From Canada"}

	for _, from := range []string{"lrclib", "the cache"} {
		lyrics, err := c.Lyrics().Fetch(canada)
		if err != nil {
			t.Fatalf("fetch from %s: %v", from, err)
		}
		if got := lineTexts(lyrics); !slices.Equal(got, want) || !lyrics.IsSynced() {
			t.Errorf("lyrics from %s = %v (synced %v), want %v, synced", from, got, lyrics.IsSynced(), want)
		}
	}

	if n := lrc.asked.Load(); n != 1 {
		t.Errorf("lrclib was asked %d times, want once: the second fetch reads the cache", n)
	}
}

func TestLyrics_AreFetchedAgainWithoutACacheFolder(t *testing.T) {
	lrc := &fakeLrclib{synced: map[string]string{"Canada": "[00:12.50]You can't get away\n"}}
	c, _ := newTestCore(t, Options{lrclib: lrc}) // no LyricsCache

	for range 2 {
		if _, err := c.Lyrics().Fetch(canada); err != nil {
			t.Fatalf("fetch: %v", err)
		}
	}

	if n := lrc.asked.Load(); n != 2 {
		t.Errorf("lrclib was asked %d times, want twice: nothing is kept", n)
	}
}

func TestLyrics_NobodyHavingThemIsErrNoLyrics(t *testing.T) {
	c, _ := newTestCore(t, Options{}) // an lrclib with no lyrics

	lyrics, err := c.Lyrics().Fetch(canada)

	if !errors.Is(err, ErrNoLyrics) || lyrics != nil {
		t.Errorf("Fetch = %v, %v; want ErrNoLyrics", lyrics, err)
	}
}

// Not the same as having none: the user may try again.
func TestLyrics_AFailureOfLrclibIsTold(t *testing.T) {
	down := errors.New("lrclib: 503")
	c, _ := newTestCore(t, Options{lrclib: &fakeLrclib{err: down}})

	_, err := c.Lyrics().Fetch(canada)

	if !errors.Is(err, down) || errors.Is(err, ErrNoLyrics) {
		t.Errorf("Fetch = %v, want lrclib's failure", err)
	}
}
