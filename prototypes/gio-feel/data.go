package main

import (
	"database/sql"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"gioui.org/op/paint"
	"golang.org/x/image/draw"
	_ "modernc.org/sqlite"
)

type track struct {
	artist, album, title string
	num, year, albumIdx  int
}

type album struct {
	artist, title string
	year          int
	dir           string
}

// loadLibrary reads the real waves library read-only, or fakes 50k tracks.
func loadLibrary() (tracks []track, albums []album, source string) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".local/share/waves/waves.db")
	if _, err := os.Stat(path); err == nil {
		t, a, err := readDB(path)
		if err == nil && len(t) > 0 {
			return t, a, path
		}
		fmt.Fprintln(os.Stderr, "prototype: library unreadable, using synthetic data:", err)
	}
	t, a := synthetic()
	return t, a, "synthetic"
}

func readDB(path string) ([]track, []album, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	var albums []album
	index := map[string]int{}
	rows, err := db.Query(`SELECT album_artist, album, COALESCE(MAX(year), 0), MIN(path)
		FROM library_tracks GROUP BY album_artist, album
		ORDER BY album_artist COLLATE NOCASE, album COLLATE NOCASE`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var a album
		var p string
		if err := rows.Scan(&a.artist, &a.title, &a.year, &p); err != nil {
			return nil, nil, err
		}
		a.dir = filepath.Dir(p)
		index[a.artist+"\x00"+a.title] = len(albums)
		albums = append(albums, a)
	}
	rows.Close()

	var tracks []track
	rows, err = db.Query(`SELECT album_artist, artist, album, title, COALESCE(track_number, 0), COALESCE(year, 0)
		FROM library_tracks
		ORDER BY album_artist COLLATE NOCASE, album COLLATE NOCASE, disc_number, track_number`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t track
		var albumArtist string
		if err := rows.Scan(&albumArtist, &t.artist, &t.album, &t.title, &t.num, &t.year); err != nil {
			return nil, nil, err
		}
		t.albumIdx = index[albumArtist+"\x00"+t.album]
		tracks = append(tracks, t)
	}
	return tracks, albums, rows.Err()
}

func synthetic() ([]track, []album) {
	var tracks []track
	var albums []album
	for a := range 4000 {
		albums = append(albums, album{artist: fmt.Sprintf("Artist %d", a/4), title: fmt.Sprintf("Album %d", a), year: 1970 + a%55})
		for n := 1; n <= 12; n++ {
			tracks = append(tracks, track{artist: albums[a].artist, album: albums[a].title, title: fmt.Sprintf("Track %d of album %d", n, a), num: n, year: albums[a].year, albumIdx: a})
		}
	}
	return tracks, albums
}

var coverNames = []string{"cover.jpg", "cover.jpeg", "cover.png", "folder.jpg", "folder.jpeg", "folder.png"}

var discDir = regexp.MustCompile(`(?i)^(cd|disc|disk)\s*\d+`)

// coverCache decodes album covers on worker goroutines, most recent request
// first, so fast scrolling never waits behind covers already off screen.
type coverCache struct {
	mu      sync.Mutex
	cond    *sync.Cond
	entries map[int]*cover
	stack   []coverReq
	notify  func()
}

type coverReq struct {
	idx, px int
	dir     string
}

type cover struct {
	img  paint.ImageOp
	ok   bool // a cover was found and decoded
	done bool
}

func newCoverCache(workers int, notify func()) *coverCache {
	c := &coverCache{entries: map[int]*cover{}, notify: notify}
	c.cond = sync.NewCond(&c.mu)
	for range workers {
		go c.work()
	}
	return c
}

// get returns a copy of album idx's cover, queueing its decode at px pixels.
func (c *coverCache) get(idx, px int, dir string) cover {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[idx]; ok {
		return *e
	}
	c.entries[idx] = &cover{}
	c.stack = append(c.stack, coverReq{idx: idx, px: px, dir: dir})
	if len(c.stack) > 96 { // forget requests scrolled past long ago; re-asked if seen again
		delete(c.entries, c.stack[0].idx)
		c.stack = c.stack[1:]
	}
	c.cond.Signal()
	return cover{}
}

func (c *coverCache) work() {
	for {
		c.mu.Lock()
		for len(c.stack) == 0 {
			c.cond.Wait()
		}
		req := c.stack[len(c.stack)-1]
		c.stack = c.stack[:len(c.stack)-1]
		c.mu.Unlock()

		img, ok := decodeCover(req.dir, req.px)
		c.mu.Lock()
		if e, live := c.entries[req.idx]; live {
			e.done, e.ok = true, ok
			if ok {
				e.img = paint.NewImageOp(img)
				e.img.Filter = paint.FilterLinear
			}
		}
		c.mu.Unlock()
		c.notify()
	}
}

func decodeCover(dir string, px int) (image.Image, bool) {
	if dir == "" {
		return nil, false
	}
	dirs := []string{dir}
	if discDir.MatchString(filepath.Base(dir)) {
		dirs = append(dirs, filepath.Dir(dir))
	}
	for _, d := range dirs {
		for _, name := range coverNames {
			f, err := os.Open(filepath.Join(d, name))
			if err != nil {
				continue
			}
			src, _, err := image.Decode(f)
			f.Close()
			if err != nil {
				continue
			}
			return squareThumb(src, px), true
		}
	}
	return nil, false
}

// squareThumb center-crops src to a square and scales it to px.
func squareThumb(src image.Image, px int) image.Image {
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(b.Min).Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))
	dst := image.NewRGBA(image.Rect(0, 0, px, px))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)
	return dst
}
