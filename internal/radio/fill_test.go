package radio

import (
	"fmt"
	"net/url"
	"slices"
	"testing"
)

// Issue #315: a fill reads radio's memory and leaves it untouched. The core
// records its seeds when it queues its tracks.

func TestRadio_FillRecordsNoSeed(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart()))

	for i := 1; i <= 2; i++ {
		res := r.Fill("Low", nil)

		if res.Err != nil || len(res.Tracks) == 0 {
			t.Fatalf("fill %d = %+v, want tracks and no error", i, res)
		}
		if want := []string{"Codeine"}; !slices.Equal(res.Seeds, want) {
			t.Errorf("fill %d seeds = %v, want %v, the artists of its tracks", i, res.Seeds, want)
		}
	}
}

func TestRadio_FillSkipsTheRecordedSeeds(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart()))

	r.AddRecentSeeds("Codeine")
	res := r.Fill("Low", nil)

	if res.Err != nil || len(res.Tracks) != 0 {
		t.Errorf("fill = %+v, want no track: Codeine is a recent seed", res)
	}
}

// Issue #269: a fill whose similar artists are one at the limit, with many
// tracks in the library, and one with a single track picks that single track.

// fillRuns is how many fills TestRadio_FillPicksTheTrackOfTheArtistUnderTheLimit
// makes: enough for the draw to land on the artist at the limit.
const fillRuns = 100

func TestRadio_FillPicksTheTrackOfTheArtistUnderTheLimit(t *testing.T) {
	codeine := fakeLastfm(oneChart())
	r := newTestRadio(t, func(method string, params url.Values) string {
		if method == "artist.getsimilar" {
			return `<lfm status="ok"><similarartists artist="Low">` +
				`<artist><name>Big</name><match>0.9</match></artist>` +
				`<artist><name>Small</name><match>0.9</match></artist>` +
				`</similarartists></lfm>`
		}
		return codeine(method, params)
	})
	for i := range 19 {
		insertTrack(t, r.db, "Big", fmt.Sprintf("Big %d", i), fmt.Sprintf("/lib/Big/%d.mp3", i), i+1)
	}
	insertTrack(t, r.db, "Small", "Small 1", "/lib/Small/1.mp3", 1)
	r.AddToRecentlyPlayed("/lib/Big/0.mp3", "Big")
	r.AddToRecentlyPlayed("/lib/Big/1.mp3", "Big")

	for i := range fillRuns {
		res := r.Fill("Low", nil)
		r.userCalls()

		if res.Err != nil || len(res.Tracks) != 1 || res.Tracks[0].Path != "/lib/Small/1.mp3" {
			t.Fatalf("fill %d = %+v, want /lib/Small/1.mp3: Big is at the limit", i, res)
		}
	}
}
