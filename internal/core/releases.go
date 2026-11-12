package core

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/listenbrainz"
	"github.com/llehouerou/waves/internal/releases"
)

// Release is a new release by an artist of the library, or by a discovery: an
// artist the library has none of, similar to several it has.
type Release = releases.Release

// ReleasesStatus is where the work that keeps the new releases fresh is.
type ReleasesStatus struct {
	Refreshing  bool  // the list is being downloaded again
	Err         error // why the last refresh failed; nil once one works
	Discoveries bool  // there is a Last.fm API key: the list has discoveries too
}

// freshReleasesAPI is the part of listenbrainz.Client the core calls.
type freshReleasesAPI interface {
	FreshReleases(ctx context.Context) ([]listenbrainz.Release, error)
}

// Releases is the new releases domain of the core: the releases ListenBrainz
// lists, cached, matched against the library and against the artists Last.fm
// finds similar to several of the library's.
//
// Two pieces of background work keep the list fresh, each one at a time: the
// refresh, which downloads the list again, and the warm-up, which asks
// Last.fm for the artists similar to those of the library. A client only
// asks for them (Refresh).
type Releases struct {
	cache   *releases.Cache // nil without a database
	db      *sql.DB
	lb      freshReleasesAPI
	ttlDays int // how long the similar artists of an artist stay fresh
	core    *Core

	refreshing atomic.Bool
	warming    atomic.Bool

	mu  sync.Mutex
	err error // why the last refresh failed
}

// Status reports where the work that keeps the list fresh is.
func (r *Releases) Status() ReleasesStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ReleasesStatus{
		Refreshing:  r.refreshing.Load(),
		Err:         r.err,
		Discoveries: r.core.lastfm.client != nil,
	}
}

// List returns the new releases, by date then artist, each one saying
// whether the library already has the album and whether it is being
// downloaded.
//
// It reads the whole cache: a client calls it off its UI goroutine.
func (r *Releases) List() ([]Release, error) {
	list, err := r.cache.Matched()
	if err != nil {
		return nil, err
	}

	// A mark that can't be read is not worth failing the list for.
	downloading, _ := r.core.downloads.mgr.ActiveReleaseGroups() //nolint:errcheck // see above

	// shortcut: one query per artist of the library with a new release
	// (~150 on a real library). A single query over all albums if it shows.
	lib := r.core.library.lib
	albums := make(map[string]map[string]struct{})
	for i := range list {
		rel := &list[i]
		rel.Downloading = downloading[rel.ReleaseGroupMBID]
		if !rel.InLibrary {
			continue
		}
		of, ok := albums[rel.ArtistCreditName]
		if !ok {
			of = lib.AlbumsForArtistNormalized(rel.ArtistCreditName)
			albums[rel.ArtistCreditName] = of
		}
		_, rel.Owned = of[library.NormalizeTitle(rel.ReleaseName)]
	}
	return list, nil
}

// Refresh brings the list up to date and returns at once. The core starts
// what does not run already:
//   - the refresh, when the releases it has cached are a day old, or whatever
//     their age when force is set; ReleasesRefreshed ends it;
//   - the warm-up, when there is a Last.fm API key; WarmupFinished ends it
//     when it changed what the discoveries come from.
func (r *Releases) Refresh(force bool) {
	if r.cache == nil {
		return
	}
	r.warm()
	if !force {
		// An age that can't be read is no reason to download: the list
		// would not be readable either.
		if stale, err := r.cache.IsStale(); err != nil || !stale {
			return
		}
	}
	if !r.refreshing.CompareAndSwap(false, true) {
		return
	}
	r.core.work.Go(func() {
		ctx, cancel := r.core.untilStopped()
		defer cancel()
		payload, err := r.lb.FreshReleases(ctx)
		if r.core.stopping() {
			return // cut short by Close: not a refresh that failed
		}
		if err == nil {
			err = r.cache.Replace(payload)
		}

		r.mu.Lock()
		r.err = err
		r.mu.Unlock()
		r.refreshing.Store(false)
		r.core.emit(ReleasesRefreshed{Err: err})
	})
}

// warm asks Last.fm for the artists similar to those of the library that have
// none fresh: the discoveries, and radio, come from them.
func (r *Releases) warm() {
	client := r.core.lastfm.client
	if client == nil || !r.warming.CompareAndSwap(false, true) {
		return
	}
	r.core.work.Go(func() {
		params := releases.WarmupParams{DB: r.db, Client: client, TTLDays: r.ttlDays}
		// An artist it failed for has nothing written: the next warm-up
		// takes it again, as it does all of them when this one failed.
		ctx, cancel := r.core.untilStopped()
		defer cancel()
		progress, _ := releases.Warmup(ctx, params) //nolint:errcheck // see above
		r.warming.Store(false)
		if progress.Current > 0 && !r.core.stopping() {
			r.core.emit(WarmupFinished{})
		}
	})
}
