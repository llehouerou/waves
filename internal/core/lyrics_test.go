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

// fakeLrclib is the lrclib of these tests: it has the synced and the plain
// lyrics it is given, by track title, and counts the times it is asked.
type fakeLrclib struct {
	synced map[string]string
	plain  map[string]string
	err    error // what it answers instead, when set
	asked  atomic.Int32
}

func (f *fakeLrclib) Get(_ context.Context, artist, title string, _ time.Duration) (*lrclib.LyricsResult, error) {
	f.asked.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	synced, hasSynced := f.synced[title]
	plain, hasPlain := f.plain[title]
	if !hasSynced && !hasPlain {
		return nil, lrclib.ErrNotFound
	}
	return &lrclib.LyricsResult{ArtistName: artist, TrackName: title, SyncedLyrics: synced, PlainLyrics: plain}, nil
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

// canadaLRC are the synced lyrics lrclib has for canada.
const canadaLRC = "[00:12.50]You can't get away\n[00:20.00]From Canada\n"

// canadaLines are the lines of canadaLRC, as they are shown.
var canadaLines = []string{"You can't get away", "From Canada"}

// A file that gives no timed line is no lyrics: the fetch goes on to lrclib,
// whether the file is next to the track or in the cache.
func TestLyrics_AFileWithNoTimedLineIsNoLyrics(t *testing.T) {
	for _, place := range []string{"next to the track", "in the cache"} {
		for name, content := range map[string]string{
			"empty":      "",
			"plain text": "You can't get away\nFrom Canada\n",
			"tags only":  "[ar:Low]\n[ti:Canada]\n",
		} {
			t.Run(place+"/"+name, func(t *testing.T) {
				lrc := &fakeLrclib{synced: map[string]string{"Canada": canadaLRC}}
				cache := t.TempDir()
				c, _ := newTestCore(t, Options{lrclib: lrc, LyricsCache: cache})
				track := LyricsTrack{FilePath: filepath.Join(t.TempDir(), "02 - Canada.mp3"), Artist: "Low", Title: "Canada"}
				file := filepath.Join(filepath.Dir(track.FilePath), "02 - Canada.lrc")
				if place == "in the cache" {
					file = filepath.Join(cache, "Low", "Canada.lrc")
				}
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}

				lyrics, err := c.Lyrics().Fetch(track)

				if err != nil {
					t.Fatalf("fetch: %v", err)
				}
				if got := lineTexts(lyrics); !slices.Equal(got, canadaLines) || !lyrics.IsSynced() {
					t.Errorf("lyrics = %q (synced %v), want lrclib's %q, synced", got, lyrics.IsSynced(), canadaLines)
				}
				if n := lrc.asked.Load(); n != 1 {
					t.Errorf("lrclib was asked %d times, want once", n)
				}
			})
		}
	}
}

// An empty .lrc next to the track, and nobody else having lyrics, is
// ErrNoLyrics.
func TestLyrics_AnEmptyFileAndNobodyElseIsErrNoLyrics(t *testing.T) {
	c, _ := newTestCore(t, Options{}) // an lrclib with no lyrics
	track := LyricsTrack{FilePath: filepath.Join(t.TempDir(), "02 - Canada.mp3"), Artist: "Low", Title: "Canada"}
	if err := os.WriteFile(filepath.Join(filepath.Dir(track.FilePath), "02 - Canada.lrc"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	lyrics, err := c.Lyrics().Fetch(track)

	if !errors.Is(err, ErrNoLyrics) || lyrics != nil {
		t.Errorf("Fetch = %v, %v; want ErrNoLyrics", lyrics, err)
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

// canadaPlain are the plain lyrics of canada: more than a dozen lines, a blank
// one between two verses, and lines in square brackets that are text.
const canadaPlain = `[Verse 1: Alan]
You can't get away
From Canada
The snow is falling
On the lake
And the lights are out
In the cabin

[Chorus]
Canada, Canada
We drove all night
To the border
And back again
[Outro]
`

// canadaPlainLines are the lines of canadaPlain, as they are shown.
var canadaPlainLines = []string{
	"[Verse 1: Alan]", "You can't get away", "From Canada", "The snow is falling",
	"On the lake", "And the lights are out", "In the cabin", "[Chorus]",
	"Canada, Canada", "We drove all night", "To the border", "And back again", "[Outro]",
}

// fetchesOnceThenFromTheCache fetches canada twice from a core with a cache
// folder on lrc, and checks that both fetches return the lines want, synced
// or not, and that only the first one asked lrclib.
func fetchesOnceThenFromTheCache(t *testing.T, lrc *fakeLrclib, want []string, synced bool) {
	t.Helper()
	c, _ := newTestCore(t, Options{lrclib: lrc, LyricsCache: t.TempDir()})

	for _, from := range []string{"lrclib", "the cache"} {
		lyrics, err := c.Lyrics().Fetch(canada)
		if err != nil {
			t.Fatalf("fetch from %s: %v", from, err)
		}
		if got := lineTexts(lyrics); !slices.Equal(got, want) || lyrics.IsSynced() != synced {
			t.Errorf("lyrics from %s = %q (synced %v), want %q (synced %v)", from, got, lyrics.IsSynced(), want, synced)
		}
	}

	if n := lrc.asked.Load(); n != 1 {
		t.Errorf("lrclib was asked %d times, want once: the second fetch reads the cache", n)
	}
}

func TestLyrics_UnsyncedComeFromLrclibOnceThenFromTheCache(t *testing.T) {
	lrc := &fakeLrclib{plain: map[string]string{"Canada": canadaPlain}}
	fetchesOnceThenFromTheCache(t, lrc, canadaPlainLines, false)
}

// Once kept, unsynced lyrics no longer need lrclib.
func TestLyrics_UnsyncedOnceFetchedOutliveLrclib(t *testing.T) {
	for name, gone := range map[string]*fakeLrclib{
		"lrclib fails":              {err: errors.New("lrclib: 503")},
		"lrclib no longer has them": {},
	} {
		t.Run(name, func(t *testing.T) {
			cache := t.TempDir()
			first, _ := newTestCore(t, Options{lrclib: &fakeLrclib{plain: map[string]string{"Canada": canadaPlain}}, LyricsCache: cache})
			if _, err := first.Lyrics().Fetch(canada); err != nil {
				t.Fatalf("first fetch: %v", err)
			}
			c, _ := newTestCore(t, Options{lrclib: gone, LyricsCache: cache})

			lyrics, err := c.Lyrics().Fetch(canada)

			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if got := lineTexts(lyrics); !slices.Equal(got, canadaPlainLines) || lyrics.IsSynced() {
				t.Errorf("lyrics = %q (synced %v), want %q, unsynced", got, lyrics.IsSynced(), canadaPlainLines)
			}
		})
	}
}

// A line of plain lyrics shaped like an LRC timestamp is text all the same.
func TestLyrics_UnsyncedLineLikeATimestampComesBackFromTheCacheAsText(t *testing.T) {
	lrc := &fakeLrclib{plain: map[string]string{"Canada": "[01:30] is when it breaks\nFrom Canada\n[00:10][00:20]twice\n"}}
	want := []string{"[01:30] is when it breaks", "From Canada", "[00:10][00:20]twice"}
	fetchesOnceThenFromTheCache(t, lrc, want, false)
}

// When lrclib has both, the synced ones are shown and kept.
func TestLyrics_SyncedWinOverPlainAndAreKept(t *testing.T) {
	lrc := &fakeLrclib{
		synced: map[string]string{"Canada": "[00:12.50]You can't get away\n[00:20.00]From Canada\n"},
		plain:  map[string]string{"Canada": "You can't get away\nFrom Canada\n"},
	}
	fetchesOnceThenFromTheCache(t, lrc, []string{"You can't get away", "From Canada"}, true)
}

// Synced lyrics with no timed line are no synced lyrics: the plain ones of the
// same answer are shown, unsynced, and kept.
func TestLyrics_SyncedWithNoTimedLineGiveWayToPlain(t *testing.T) {
	for name, synced := range map[string]string{
		"plain text":        "From Canada\n",
		"blank lines alone": "\n  \n",
	} {
		t.Run(name, func(t *testing.T) {
			lrc := &fakeLrclib{
				synced: map[string]string{"Canada": synced},
				plain:  map[string]string{"Canada": canadaPlain},
			}
			fetchesOnceThenFromTheCache(t, lrc, canadaPlainLines, false)
		})
	}
}

// Blank plain lyrics are no lyrics: nothing is kept, lrclib is asked again.
func TestLyrics_BlankPlainAreErrNoLyricsAndNotKept(t *testing.T) {
	lrc := &fakeLrclib{plain: map[string]string{"Canada": "  \n\n\t\n"}}
	c, _ := newTestCore(t, Options{lrclib: lrc, LyricsCache: t.TempDir()})

	for range 2 {
		if lyrics, err := c.Lyrics().Fetch(canada); !errors.Is(err, ErrNoLyrics) || lyrics != nil {
			t.Fatalf("Fetch = %v, %v; want ErrNoLyrics", lyrics, err)
		}
	}

	if n := lrc.asked.Load(); n != 2 {
		t.Errorf("lrclib was asked %d times, want twice: nothing is kept", n)
	}
}

func TestLyrics_UnsyncedAreFetchedAgainWithoutACacheFolder(t *testing.T) {
	lrc := &fakeLrclib{plain: map[string]string{"Canada": canadaPlain}}
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
