package listenbrainz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Captured from GET /1/explore/fresh-releases/?days=1&past=true&future=false.
// Third entry has no release_group_primary_type key (deleted server-side when null).
const sampleJSON = `{"payload":{"releases":[
{"artist_credit_name":"다브다","artist_mbids":["69cd5e37-c134-4aa9-a878-42f2f9612b96"],"caa_id":46215607761,"caa_release_mbid":"1d2f9e1a-0ea1-4c6f-9204-a60e065246fb","listen_count":3,"release_date":"2026-09-17","release_group_mbid":"b01dd0f8-00f0-41e7-b9b8-701b96fa40dd","release_group_primary_type":"Album","release_mbid":"1d2f9e1a-0ea1-4c6f-9204-a60e065246fb","release_name":"ON(百)","release_tags":[]},
{"artist_credit_name":"Bryce Dessner","artist_mbids":["20336142-85de-4e11-a1fd-a6c815b4e583"],"caa_id":46210599706,"caa_release_mbid":"e71db4a3-ca96-4ecd-8af6-c1522fdaf318","listen_count":0,"release_date":"2026-09-17","release_group_mbid":"75735e6d-24fa-4dd6-90db-938291433c3e","release_group_primary_type":"Album","release_group_secondary_type":"Soundtrack","release_mbid":"e71db4a3-ca96-4ecd-8af6-c1522fdaf318","release_name":"Monster","release_tags":[]},
{"artist_credit_name":"Sarsur","artist_mbids":["9966fb84-4107-43eb-9745-1e7074a1b781"],"caa_id":null,"caa_release_mbid":null,"listen_count":0,"release_date":"2026-09-17","release_group_mbid":"d59e404c-78c1-45cf-9da1-5a3ce7cbf009","release_group_secondary_type":"Demo","release_mbid":"4974b72d-38e2-4dba-b37a-e07e2a956a8d","release_name":"Molten Rock and Ash","release_tags":[]}
],"count":3}}`

func TestFreshReleases(t *testing.T) {
	var gotQuery, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleJSON))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	releases, err := c.FreshReleases(context.Background())
	if err != nil {
		t.Fatalf("FreshReleases: %v", err)
	}

	if gotQuery != "days=90&future=true&past=true" {
		t.Errorf("query = %q", gotQuery)
	}
	if gotUA == "" {
		t.Error("no User-Agent sent")
	}
	if len(releases) != 3 {
		t.Fatalf("got %d releases, want 3", len(releases))
	}

	first := releases[0]
	if first.ArtistCreditName != "다브다" || first.ReleaseName != "ON(百)" ||
		first.ReleaseGroupMBID != "b01dd0f8-00f0-41e7-b9b8-701b96fa40dd" ||
		first.ReleaseDate != "2026-09-17" || first.PrimaryType != "Album" || first.ListenCount != 3 {
		t.Errorf("first release = %+v", first)
	}
	if releases[1].SecondaryType != "Soundtrack" {
		t.Errorf("secondary type = %q", releases[1].SecondaryType)
	}
	// Missing key is an unknown type, not an error.
	if releases[2].PrimaryType != "" {
		t.Errorf("missing primary type = %q, want empty", releases[2].PrimaryType)
	}
}

func TestFreshReleases_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"days must be between 1 and 90."}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if _, err := c.FreshReleases(context.Background()); err == nil {
		t.Fatal("expected error on HTTP 400")
	}
}
