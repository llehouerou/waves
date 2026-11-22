package radio

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/state"
)

// Issue #242: radio remembers what Last.fm answered about an artist, an empty
// list or a refusal included, and asks again only for what failed.

const (
	similarMethod   = "artist.getsimilar"
	topTracksMethod = "artist.gettoptracks"
)

// similarXML is the answer of artist.getsimilar naming names.
func similarXML(names ...string) string {
	var b strings.Builder
	b.WriteString(`<lfm status="ok"><similarartists artist="Low">`)
	for _, name := range names {
		fmt.Fprintf(&b, `<artist><name>%s</name><match>0.9</match></artist>`, name)
	}
	b.WriteString(`</similarartists></lfm>`)
	return b.String()
}

// topTracksXML is the answer of artist.gettoptracks naming titles.
func topTracksXML(titles ...string) string {
	var b strings.Builder
	b.WriteString(`<lfm status="ok"><toptracks artist="Codeine">`)
	for i, title := range titles {
		fmt.Fprintf(&b, `<track rank="%d"><name>%s</name><playcount>100</playcount></track>`, i+1, title)
	}
	b.WriteString(`</toptracks></lfm>`)
	return b.String()
}

// codeineSimilar answers that Codeine is the artist similar to any other.
func codeineSimilar(string) string { return similarXML("Codeine") }

// refusedTopTracks refuses the top tracks of any artist.
func refusedTopTracks(string) string { return lastfmtest.Error(6) }

// answering is a Last.fm with an empty user chart that answers the similar
// artists and the top tracks of an artist with similar and topTracks.
func answering(similar, topTracks func(artist string) string) func(string, url.Values) string {
	user := fakeLastfm(oneChart())
	return func(method string, params url.Values) string {
		switch method {
		case similarMethod:
			return similar(params.Get("artist"))
		case topTracksMethod:
			return topTracks(params.Get("artist"))
		}
		return user(method, params)
	}
}

// played puts a track by each of artists in the library and in the tracks
// radio played, in order.
func (r *testRadio) played(t *testing.T, artists ...string) {
	t.Helper()
	for _, artist := range artists {
		path := "/lib/" + artist + "/1.mp3"
		insertTrack(t, r.db, artist, artist+" 1", path, 1)
		r.AddToRecentlyPlayed(path, artist)
	}
}

// fillAsking fills from seed and returns the artists Last.fm was asked
// method about meanwhile, in order.
func (r *testRadio) fillAsking(seed, method string) (res FillResult, asked []string) {
	res = r.Fill(seed, nil)
	for {
		select {
		case call := <-r.calls:
			if call.Method == method {
				asked = append(asked, call.Params.Get("artist"))
			}
		default:
			return res, asked
		}
	}
}

// expectTracks fails the test unless res holds tracks and no error.
func expectTracks(t *testing.T, fill int, res FillResult) {
	t.Helper()
	if res.Err != nil || len(res.Tracks) == 0 {
		t.Fatalf("fill %d = %+v, want tracks and no error", fill, res)
	}
}

// expectAsked fails the test unless asked is want.
func expectAsked(t *testing.T, fill int, method string, asked, want []string) {
	t.Helper()
	if !slices.Equal(asked, want) {
		t.Errorf("fill %d asked %s about %v, want %v", fill, method, asked, want)
	}
}

func TestRadio_TopTracksLastfmHasNoneForAreNotAskedAgain(t *testing.T) {
	answers := []struct{ name, answer string }{
		{"refused", lastfmtest.Error(6)},
		{"empty", topTracksXML()},
	}
	for _, a := range answers {
		t.Run(a.name, func(t *testing.T) {
			r := newTestRadio(t, answering(codeineSimilar, func(string) string { return a.answer }))

			for fill := 1; fill <= 3; fill++ {
				res, asked := r.fillAsking("Low", topTracksMethod)
				expectTracks(t, fill, res)
				var want []string
				if fill == 1 {
					want = []string{"Codeine"}
				}
				expectAsked(t, fill, topTracksMethod, asked, want)
			}
		})
	}
}

func TestRadio_SeedsLastfmHasNoSimilarArtistForAreNotAskedAgain(t *testing.T) {
	r := newTestRadio(t, answering(func(string) string { return similarXML() }, refusedTopTracks))
	r.played(t, "Galaxie 500", "Bedhead")

	for fill := 1; fill <= 3; fill++ {
		res, asked := r.fillAsking("Low", similarMethod)
		if res.Err != nil || len(res.Tracks) != 0 {
			t.Fatalf("fill %d = %+v, want no track and no error", fill, res)
		}
		var want []string
		if fill == 1 {
			want = []string{"Low", "Bedhead", "Galaxie 500"} // the seed, then the artists played, last first
		}
		expectAsked(t, fill, similarMethod, asked, want)
	}
}

func TestRadio_SeedLastfmRefusesFillsFromTheArtistsPlayed(t *testing.T) {
	r := newTestRadio(t, answering(func(artist string) string {
		if artist == "Low" {
			return lastfmtest.Error(6)
		}
		return similarXML("Codeine")
	}, refusedTopTracks))
	r.played(t, "Galaxie 500")

	for fill := 1; fill <= 3; fill++ {
		res, asked := r.fillAsking("Low", similarMethod)
		expectTracks(t, fill, res)
		var want []string
		if fill == 1 {
			want = []string{"Low", "Galaxie 500"}
		}
		expectAsked(t, fill, similarMethod, asked, want)
	}
}

// failures are Last.fm's answers that say nothing about the artist: what
// they were asked about is asked again by the next fill.
var failures = []struct{ name, answer string }{
	{"unreachable", lastfmtest.Unreachable},
	{"timeout", lastfmtest.Stalled},
	{"try again later", lastfmtest.Error(11)},
}

func TestRadio_SimilarArtistsThatFailAreAskedAgain(t *testing.T) {
	for _, f := range failures {
		t.Run(f.name, func(t *testing.T) {
			r := newTestRadio(t, answering(func(string) string { return f.answer }, refusedTopTracks))

			for fill := 1; fill <= 2; fill++ {
				res, asked := r.fillAsking("Low", similarMethod)
				if res.Err == nil {
					t.Errorf("fill %d = %+v, want the error", fill, res)
				}
				// Go's transport sends a request again when the connection
				// it reused is cut.
				expectAsked(t, fill, similarMethod, slices.Compact(asked), []string{"Low"})
			}
		})
	}
}

func TestRadio_TopTracksThatFailAreAskedAgain(t *testing.T) {
	for _, f := range failures {
		t.Run(f.name, func(t *testing.T) {
			r := newTestRadio(t, answering(codeineSimilar, func(string) string { return f.answer }))

			for fill := 1; fill <= 2; fill++ {
				res, asked := r.fillAsking("Low", topTracksMethod)
				expectTracks(t, fill, res)
				expectAsked(t, fill, topTracksMethod, slices.Compact(asked), []string{"Codeine"}) // a cut request may be sent again
			}
		})
	}
}

// ageEmptyAnswers makes the empty answers radio remembers older than its TTL.
func (r *testRadio) ageEmptyAnswers(t *testing.T) {
	t.Helper()
	aged := time.Now().AddDate(0, 0, -r.config.CacheTTLDays-1).Unix()
	for _, age := range []string{
		`UPDATE lastfm_no_similar_artists SET fetched_at = ?`,
		`UPDATE lastfm_no_artist_top_tracks SET fetched_at = ?`,
	} {
		if _, err := r.db.Exec(age, aged); err != nil {
			t.Fatalf("%s: %v", age, err)
		}
	}
}

func TestRadio_EmptyAnswerOlderThanTheTTLIsAskedAgain(t *testing.T) {
	t.Run("similar artists", func(t *testing.T) {
		r := newTestRadio(t, answering(func(string) string { return similarXML() }, refusedTopTracks))
		r.fillAsking("Low", similarMethod)
		r.ageEmptyAnswers(t)

		_, asked := r.fillAsking("Low", similarMethod)
		expectAsked(t, 2, similarMethod, asked, []string{"Low"})
	})
	t.Run("top tracks", func(t *testing.T) {
		r := newTestRadio(t, answering(codeineSimilar, refusedTopTracks))
		r.fillAsking("Low", topTracksMethod)
		r.ageEmptyAnswers(t)

		res, asked := r.fillAsking("Low", topTracksMethod)
		expectTracks(t, 2, res)
		expectAsked(t, 2, topTracksMethod, asked, []string{"Codeine"})
	})
}

func TestRadio_DatabaseOfThePreviousSchemaRemembersEmptyAnswers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waves.db")
	st, err := state.OpenAt(path)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	// The tables of the empty answers are what the previous schema lacks.
	if _, err := st.DB().Exec(`
		DROP TABLE IF EXISTS lastfm_no_similar_artists;
		DROP TABLE IF EXISTS lastfm_no_artist_top_tracks;
	`); err != nil {
		t.Fatalf("drop the tables of the empty answers: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close state: %v", err)
	}

	r := openTestRadio(t, path, answering(func(artist string) string {
		if artist == "Low" {
			return similarXML("Codeine")
		}
		return similarXML()
	}, func(string) string { return topTracksXML() }))

	for fill := 1; fill <= 2; fill++ {
		res, asked := r.fillAsking("Low", topTracksMethod)
		expectTracks(t, fill, res)
		var want []string
		if fill == 1 {
			want = []string{"Codeine"}
		}
		expectAsked(t, fill, topTracksMethod, asked, want)

		_, asked = r.fillAsking("Galaxie 500", similarMethod)
		want = nil
		if fill == 1 {
			want = []string{"Galaxie 500"}
		}
		expectAsked(t, fill, similarMethod, asked, want)
	}
}
