// Package slskdtest serves a fake slskd to the tests of the packages that talk
// to one.
package slskdtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/llehouerou/waves/internal/slskd"
)

// Server is a fake slskd: it serves the transfers and the search it was
// given, and records what it was asked to queue, to drop and to search.
type Server struct {
	URL string

	mu        sync.Mutex
	down      bool
	transfers []slskd.DownloadFile
	refuse    bool
	queuedFor string
	queued    []slskd.File
	dropped   []string

	search    slskd.SearchRequest
	responses []slskd.SearchResponse
	searches  []string
}

// New starts a fake slskd holding transfers, closed when the test ends. Its
// search is in progress, with no response.
func New(t testing.TB, transfers ...slskd.DownloadFile) *Server {
	t.Helper()
	s := &Server{transfers: transfers, search: slskd.SearchRequest{State: string(slskd.SearchStateInProgress)}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/transfers/downloads", s.listTransfers)
	mux.HandleFunc("POST /api/v0/transfers/downloads/{user}", s.queue)
	mux.HandleFunc("DELETE /api/v0/transfers/downloads/{user}/{id}", s.drop)
	mux.HandleFunc("POST /api/v0/searches", s.startSearch)
	mux.HandleFunc("GET /api/v0/searches/{id}", s.searchStatus)
	mux.HandleFunc("GET /api/v0/searches/{id}/responses", s.searchResponses)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		down := s.down
		s.mu.Unlock()
		if down {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// Client returns a client of the fake.
func (s *Server) Client() *slskd.Client {
	return slskd.NewClient(s.URL, "key")
}

// Down makes the fake fail every request, or answer again.
func (s *Server) Down(down bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.down = down
}

// SetSearch sets where every search is: its slskd state, how many users it
// says have answered, and the responses it returns when asked for them.
func (s *Server) SetSearch(state string, responseCount int, responses ...slskd.SearchResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.search.State, s.search.ResponseCount, s.responses = state, responseCount, responses
}

// Searches returns the queries the fake was asked to search, in order.
func (s *Server) Searches() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.searches
}

// SetTransfers replaces the transfers the fake reports.
func (s *Server) SetTransfers(transfers ...slskd.DownloadFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transfers = transfers
}

// Refuse makes the fake refuse, or accept again, what it is asked to queue.
func (s *Server) Refuse(refuse bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refuse = refuse
}

// Queued returns the last files the fake accepted to queue, and for which
// user: "" and nil when it queued nothing.
func (s *Server) Queued() (user string, files []slskd.File) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queuedFor, s.queued
}

// Dropped returns the transfers the fake was asked to drop, each as
// "user/id?query".
func (s *Server) Dropped() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

func (s *Server) listTransfers(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var resp []slskd.DownloadsResponse // one per user, in first-seen order
	for _, tr := range s.transfers {
		i := len(resp) - 1
		if i < 0 || resp[i].Username != tr.Username {
			resp = append(resp, slskd.DownloadsResponse{Username: tr.Username, Directories: []slskd.DownloadDirectory{{}}})
			i++
		}
		resp[i].Directories[0].Files = append(resp[i].Directories[0].Files, tr)
	}
	_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck // a test server: the client's decode fails the test
}

func (s *Server) queue(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refuse {
		http.Error(w, "refused", http.StatusInternalServerError)
		return
	}
	s.queuedFor = r.PathValue("user")
	_ = json.NewDecoder(r.Body).Decode(&s.queued) //nolint:errcheck // a test server: a bad body leaves nothing queued
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) drop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropped = append(s.dropped, r.PathValue("user")+"/"+r.PathValue("id")+"?"+r.URL.RawQuery)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startSearch(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body struct {
		SearchText string `json:"searchText"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // a test server: a bad body is an empty query
	s.searches = append(s.searches, body.SearchText)
	_ = json.NewEncoder(w).Encode(slskd.SearchRequest{ID: "search", SearchText: body.SearchText}) //nolint:errcheck // as in listTransfers
}

func (s *Server) searchStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = json.NewEncoder(w).Encode(s.search) //nolint:errcheck // as in listTransfers
}

func (s *Server) searchResponses(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	responses := s.responses
	if responses == nil {
		responses = []slskd.SearchResponse{}
	}
	_ = json.NewEncoder(w).Encode(responses) //nolint:errcheck // as in listTransfers
}
