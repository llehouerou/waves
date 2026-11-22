package musicbrainz

import (
	"fmt"
	"maps"
	"net/url"
	"strconv"
)

const (
	// browsePageSize is the number of entities a browse request asks for at
	// once: the most MusicBrainz gives.
	browsePageSize = 100
	// maxBrowsePages bounds the requests of one browse call, so 2000
	// entities: some lists cannot be read in full (Various Artists has close
	// to 300,000 release groups), and every request of waves waits for the
	// one rate limiter.
	maxBrowsePages = 20
)

// Browsed is what a browse call read: the entities, and how many MusicBrainz
// has. Fewer entities than the total is a list cut by maxBrowsePages.
type Browsed[T any] struct {
	Items []T
	Total int
}

// browsePage is the response to one page of a browse request: its entities,
// and how many MusicBrainz has.
type browsePage[R any] interface {
	entities() []R
	count() int
}

// browse reads the pages of the browse request of entity with params, from
// the first, until it holds as many entities as MusicBrainz reports, a page
// comes back empty or maxBrowsePages pages are read. It returns the entities
// in the order MusicBrainz gave them, and the total MusicBrainz reported. A
// failed page fails the whole call: it returns no entity. The params of the
// caller are left as they are.
func browse[R any, P browsePage[R]](c *Client, entity string, params url.Values) (read []R, total int, err error) {
	params = maps.Clone(params)
	params.Set("fmt", "json")
	params.Set("limit", strconv.Itoa(browsePageSize))
	for range maxBrowsePages {
		params.Set("offset", strconv.Itoa(len(read)))
		var page P
		if err := c.getJSON(fmt.Sprintf("%s/%s?%s", baseURL, entity, params.Encode()), &page); err != nil {
			return nil, 0, err
		}
		entities := page.entities()
		read = append(read, entities...)
		total = page.count()
		if len(entities) == 0 || len(read) >= total {
			break
		}
	}
	return read, total, nil
}
