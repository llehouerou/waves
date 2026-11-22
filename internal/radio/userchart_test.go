package radio

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/state"
)

// These tests swap http.DefaultTransport (lastfmtest.Serve): none is parallel.

// testBound is the shortened bound of the Last.fm clients these tests build.
const testBound = 100 * time.Millisecond

// chartTrack is a track of the user's chart on the fake Last.fm.
type chartTrack struct {
	artist, name string
	playcount    int
}

// chartPage is the body of a page of the user's chart.
func chartPage(page, totalPages int, tracks ...chartTrack) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<lfm status="ok"><toptracks user="lowfan" page="%d" perPage="1000" totalPages="%d" total="%d">`,
		page, totalPages, totalPages*len(tracks))
	for i, t := range tracks {
		fmt.Fprintf(&b, `<track rank="%d"><name>%s</name><playcount>%d</playcount><artist><name>%s</name></artist></track>`,
			i+1, t.name, t.playcount, t.artist)
	}
	b.WriteString(`</toptracks></lfm>`)
	return b.String()
}

// fakeLastfm answers that Codeine is the artist similar to any other, knows
// no top track, names each session's user after its key, and answers the
// user's chart with chart.
func fakeLastfm(chart func(user string, page int) string) func(string, url.Values) string {
	return func(method string, params url.Values) string {
		switch method {
		case "artist.getsimilar":
			return `<lfm status="ok"><similarartists artist="Low">` +
				`<artist><name>Codeine</name><match>0.9</match></artist>` +
				`</similarartists></lfm>`
		case "user.getinfo":
			return `<lfm status="ok"><user><name>` + params.Get("sk") + `-user</name></user></lfm>`
		case "user.gettoptracks":
			page, _ := strconv.Atoi(params.Get("page"))
			return chart(params.Get("user"), page)
		}
		return ""
	}
}

// oneChart answers a chart of one page holding tracks, for any user.
func oneChart(tracks ...chartTrack) func(string, int) string {
	return func(string, int) string { return chartPage(1, 1, tracks...) }
}

// testRadio is a radio on a library holding three tracks by Codeine, "Codeine
// 1" to "Codeine 3", and a Last.fm client linked with the session "first".
type testRadio struct {
	*Radio
	client *lastfm.Client
	db     *sql.DB
	calls  <-chan lastfmtest.Call
	ids    map[string]int64 // library track IDs by title
}

func newTestRadio(t *testing.T, respond func(string, url.Values) string) *testRadio {
	t.Helper()
	calls := lastfmtest.Serve(t, respond)
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "waves.db"))
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	db := st.DB()

	ids := make(map[string]int64)
	for i := 1; i <= 3; i++ {
		title := "Codeine " + strconv.Itoa(i)
		ids[title] = insertTrack(t, db, "Codeine", title, "/lib/Codeine/"+strconv.Itoa(i)+".mp3", i)
	}

	client := lastfm.NewBounded("key", "secret", testBound)
	client.SetSessionKey("first")
	r := New(db, client, library.New(db), (&config.Config{}).GetRadioConfig())
	return &testRadio{Radio: r, client: client, db: db, calls: calls, ids: ids}
}

// insertTrack adds to the library in db the track number of artist titled
// title at path, and returns its ID.
func insertTrack(t *testing.T, db *sql.DB, artist, title, path string, number int) int64 {
	t.Helper()
	res, err := db.Exec(`
		INSERT INTO library_tracks
			(path, mtime, artist, album_artist, album, title, disc_number, track_number, year, added_at, updated_at)
		VALUES (?, 1000, ?, ?, 'Album', ?, 1, ?, 2024, 1000, 1000)
	`, path, artist, artist, title, number)
	if err != nil {
		t.Fatalf("insert track: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// fill fills from seed and returns the user.* requests Last.fm received
// meanwhile.
func (r *testRadio) fill(t *testing.T, seed string) []lastfmtest.Call {
	t.Helper()
	res := r.Fill(seed, nil)
	if res.Err != nil || len(res.Tracks) == 0 {
		t.Fatalf("fill = %+v, want tracks and no error", res)
	}
	return r.userCalls()
}

// userCalls drains the calls Last.fm received and returns the user.* ones.
func (r *testRadio) userCalls() []lastfmtest.Call {
	var user []lastfmtest.Call
	for {
		select {
		case call := <-r.calls:
			if strings.HasPrefix(call.Method, "user.") {
				user = append(user, call)
			}
		default:
			return user
		}
	}
}

// candidates builds the candidates of a fill picking Codeine and returns them
// by title.
func (r *testRadio) candidates(favorites map[int64]bool) map[string]Candidate {
	codeine := MatchedArtist{LastfmArtist: lastfm.SimilarArtist{Name: "Codeine", MatchScore: 0.9}, LocalArtist: "Codeine"}
	pool := r.buildCandidatePool([]MatchedArtist{codeine}, nil, favorites, r.userPlays())
	byTitle := make(map[string]Candidate, len(pool))
	for i := range pool {
		byTitle[pool[i].LibraryTrack.Title] = pool[i]
	}
	return byTitle
}

// expectBoosted checks which of Codeine's tracks are boosted as scrobbled by
// the user: each is UserBoost times as likely as "Codeine 3", never boosted.
func expectBoosted(t *testing.T, cands map[string]Candidate, boost float64, titles ...string) {
	t.Helper()
	base := cands["Codeine 3"]
	if base.UserScrobbled || base.Score == 0 {
		t.Fatalf("Codeine 3 = %+v, want a candidate not scrobbled", base)
	}
	for _, title := range []string{"Codeine 1", "Codeine 2"} {
		c := cands[title]
		want, wantScore := false, base.Score
		for _, boosted := range titles {
			if boosted == title {
				want, wantScore = true, base.Score*boost
			}
		}
		if c.UserScrobbled != want {
			t.Errorf("%s scrobbled = %v, want %v", title, c.UserScrobbled, want)
		}
		if diff := c.Score - wantScore; diff > 1e-12 || diff < -1e-12 {
			t.Errorf("%s score = %v, want %v", title, c.Score, wantScore)
		}
	}
}

// methods returns the methods of calls.
func methods(calls []lastfmtest.Call) []string {
	m := make([]string, len(calls))
	for i, c := range calls {
		m[i] = c.Method
	}
	return m
}

// expectNoUserCall fails the test when calls holds a user.* request.
func expectNoUserCall(t *testing.T, calls []lastfmtest.Call) {
	t.Helper()
	if len(calls) > 0 {
		t.Errorf("Last.fm received %v, want no user.* request", methods(calls))
	}
}

// pages returns the pages of the chart that calls asked for, in order.
func pages(calls []lastfmtest.Call) []string {
	var p []string
	for _, c := range calls {
		if c.Method == "user.gettoptracks" {
			p = append(p, c.Params.Get("page"))
		}
	}
	return p
}

func TestRadio_TrackInTheUserChartIsBoosted(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart(chartTrack{"Codeine", "Codeine 1", 12})))

	cands := r.candidates(nil)

	expectBoosted(t, cands, r.config.UserBoost, "Codeine 1")
	if got := cands["Codeine 1"].UserPlaycount; got != 12 {
		t.Errorf("user playcount = %d, want 12", got)
	}
}

func TestRadio_FavoriteInTheUserChartGetsTheFavoriteBoostInstead(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart(chartTrack{"Codeine", "Codeine 1", 12})))

	cands := r.candidates(map[int64]bool{r.ids["Codeine 1"]: true})

	expectBoosted(t, cands, r.config.FavoriteBoost, "Codeine 1")
}

func TestRadio_FillFavorsTheTrackTheUserScrobbled(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart(chartTrack{"Codeine", "Codeine 2", 12})))
	r.config.UserBoost = 1e9 // any other track: a chance in a billion

	res := r.Fill("Low", nil)

	if res.Err != nil || len(res.Tracks) != 1 || res.Tracks[0].Title != "Codeine 2" {
		t.Errorf("fill = %+v, want Codeine 2, the track the user scrobbled", res)
	}
}

func TestRadio_UserChartMatchesWhatNormalizeTitleIgnores(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart(
		chartTrack{"CODEÏNE", "codéine 1 (Remastered)", 12},
		chartTrack{"Low", "Codeine 2", 40}, // same title, another artist
	)))

	expectBoosted(t, r.candidates(nil), r.config.UserBoost, "Codeine 1")
}

func TestRadio_UserChartIsReadToItsLastPage(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(func(user string, page int) string {
		if page == 3 {
			return chartPage(3, 3, chartTrack{"Codeine", "Codeine 2", 1})
		}
		return chartPage(page, 3, chartTrack{"Shellac", "Prayer to God " + strconv.Itoa(page), 9})
	}))

	cands := r.candidates(nil)
	calls := r.userCalls()

	expectBoosted(t, cands, r.config.UserBoost, "Codeine 2")
	if got := pages(calls); !slices.Equal(got, []string{"1", "2", "3"}) {
		t.Errorf("pages asked = %v, want 1, 2, 3", got)
	}
	for _, c := range calls {
		if c.Method != "user.gettoptracks" {
			continue
		}
		if p := c.Params; p.Get("user") != "first-user" || p.Get("period") != "overall" || p.Get("limit") != "1000" {
			t.Errorf("user.gettoptracks params = %v, want user first-user, period overall, limit 1000", p)
		}
	}
}

func TestRadio_UserChartThatNeverEndsIsReadFor30Pages(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(func(user string, page int) string {
		if page == 30 {
			return chartPage(30, 100000, chartTrack{"Codeine", "Codeine 1", 1})
		}
		return chartPage(page, 100000, chartTrack{"Shellac", "Prayer to God " + strconv.Itoa(page), 9})
	}))

	if got := len(pages(r.fill(t, "Low"))); got != 30 {
		t.Errorf("pages asked = %d, want 30", got)
	}
	expectBoosted(t, r.candidates(nil), r.config.UserBoost, "Codeine 1")
	expectNoUserCall(t, r.userCalls())
}

func TestRadio_FreshUserChartIsNotAskedAgain(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart(chartTrack{"Codeine", "Codeine 1", 12})))

	if got := methods(r.fill(t, "Low")); !slices.Equal(got, []string{"user.getinfo", "user.gettoptracks"}) {
		t.Fatalf("first fill sent %v, want user.getinfo then user.gettoptracks", got)
	}
	expectNoUserCall(t, r.fill(t, "Low"))
	expectNoUserCall(t, r.fill(t, "Galaxie 500"))
	expectBoosted(t, r.candidates(nil), r.config.UserBoost, "Codeine 1")
}

func TestRadio_UserChartOlderThanTheTTLIsReadAgain(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart(chartTrack{"Codeine", "Codeine 1", 12})))
	r.fill(t, "Low")

	aged := time.Now().AddDate(0, 0, -r.config.CacheTTLDays-1).Unix()
	if _, err := r.db.Exec(`UPDATE lastfm_user_chart SET fetched_at = ?`, aged); err != nil {
		t.Fatalf("age the chart: %v", err)
	}

	if got := pages(r.fill(t, "Low")); !slices.Equal(got, []string{"1"}) {
		t.Errorf("pages asked = %v, want the chart read again", got)
	}
}

func TestRadio_EmptyUserChartIsRemembered(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart()))

	if got := len(pages(r.fill(t, "Low"))); got != 1 {
		t.Fatalf("pages asked = %d, want 1", got)
	}
	expectNoUserCall(t, r.fill(t, "Low"))
}

func TestRadio_UserChartLastfmRefusesIsRememberedAsEmpty(t *testing.T) {
	for _, code := range []int{27, 6} {
		t.Run("error "+strconv.Itoa(code), func(t *testing.T) {
			r := newTestRadio(t, fakeLastfm(func(string, int) string { return lastfmtest.Error(code) }))

			if got := len(pages(r.fill(t, "Low"))); got != 1 {
				t.Fatalf("pages asked = %d, want 1", got)
			}
			expectNoUserCall(t, r.fill(t, "Low"))
			expectBoosted(t, r.candidates(nil), r.config.UserBoost)
		})
	}
}

func TestRadio_UserChartThatFailsIsAskedAgainFromItsFirstPage(t *testing.T) {
	failures := []struct{ name, answer string }{
		{"timeout", lastfmtest.Stalled},
		{"unreachable", lastfmtest.Unreachable},
		{"try again later", lastfmtest.Error(11)},
	}
	for _, f := range failures {
		t.Run(f.name, func(t *testing.T) {
			var failing atomic.Bool
			failing.Store(true)
			r := newTestRadio(t, fakeLastfm(func(user string, page int) string {
				switch {
				case page == 2 && failing.Load():
					return f.answer
				case page == 2:
					return chartPage(2, 2, chartTrack{"Codeine", "Codeine 2", 1})
				}
				return chartPage(1, 2, chartTrack{"Codeine", "Codeine 1", 12})
			}))

			// Go's transport sends a request again when the connection it
			// reused is cut: a page cut may be asked twice in a row.
			if got := slices.Compact(pages(r.fill(t, "Low"))); !slices.Equal(got, []string{"1", "2"}) {
				t.Fatalf("pages asked = %v, want 1, 2", got)
			}
			failing.Store(false)
			if got := pages(r.fill(t, "Low")); !slices.Equal(got, []string{"1", "2"}) {
				t.Errorf("pages asked by the next fill = %v, want 1, 2", got)
			}
			expectBoosted(t, r.candidates(nil), r.config.UserBoost, "Codeine 1", "Codeine 2")
		})
	}
}

func TestRadio_UserChartThatFailsBoostsNothing(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(func(user string, page int) string {
		if page == 2 {
			return lastfmtest.Unreachable
		}
		return chartPage(1, 2, chartTrack{"Codeine", "Codeine 1", 12})
	}))

	expectBoosted(t, r.candidates(nil), r.config.UserBoost)
}

func TestRadio_NoAccountNoUserChart(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart(chartTrack{"Codeine", "Codeine 1", 12})))
	r.client.SetSessionKey("")

	expectNoUserCall(t, r.fill(t, "Low"))
	expectBoosted(t, r.candidates(nil), r.config.UserBoost)
}

func TestRadio_UserChartBelongsToTheAccountItWasReadFor(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(func(user string, page int) string {
		if user == "second-user" {
			return chartPage(1, 1, chartTrack{"Codeine", "Codeine 2", 3})
		}
		return chartPage(1, 1, chartTrack{"Codeine", "Codeine 1", 12})
	}))
	r.fill(t, "Low")

	r.client.SetSessionKey("second") // what Lastfm.CompleteLink does
	calls := r.fill(t, "Low")

	if got := pages(calls); len(got) != 1 || calls[len(calls)-1].Params.Get("user") != "second-user" {
		t.Errorf("fill after another account was linked sent %v, want the second account's chart read", methods(calls))
	}
	expectBoosted(t, r.candidates(nil), r.config.UserBoost, "Codeine 2")

	r.client.SetSessionKey("") // what Lastfm.Unlink does
	expectBoosted(t, r.candidates(nil), r.config.UserBoost)
}
