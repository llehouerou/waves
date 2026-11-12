package core

import (
	"context"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/slskd"
)

// Search starts a search of the Soulseek network and returns at once:
// SearchProgressed follows it and SearchFinished ends it, with its responses
// or its failure. One search runs at a time: a new one replaces the one
// running, which is no longer reported.
func (d *Downloads) Search(query string) error {
	if d.client == nil {
		return downloads.ErrNoSlskd
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.searchMu.Lock()
	if d.cancelSearch != nil {
		d.cancelSearch()
	}
	d.cancelSearch = cancel
	d.searchMu.Unlock()

	go func() {
		// slskd is polled twice per tick: every half second.
		responses, err := d.client.RunSearch(ctx, query, d.core.tick/2, func(p slskd.SearchProgress) {
			if ctx.Err() != nil {
				return
			}
			d.core.emit(SearchProgressed{
				Query:     query,
				Phase:     p.Phase,
				State:     p.State,
				Responses: p.Responses,
				Elapsed:   p.Elapsed,
			})
		})
		if ctx.Err() != nil || d.core.stopping() {
			return // replaced by another search, or cut short by Close
		}
		d.core.emit(SearchFinished{Query: query, Responses: responses, Err: err})
	}()
	return nil
}
