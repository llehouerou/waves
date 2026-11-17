package mbworkflow

import tea "github.com/charmbracelet/bubbletea"

// RequestKind is what a MusicBrainz request asks for.
type RequestKind int

// The kinds of request, with the message that answers each. None is zero: a
// Request with no kind is no request.
const (
	SearchArtists        RequestKind = iota + 1 // artists for a query: ArtistSearchResultMsg
	SearchReleaseGroups                         // release groups for a query: SearchResultMsg
	ArtistReleaseGroups                         // release groups of an artist: SearchResultMsg
	ReleaseGroupReleases                        // releases of a release group: ReleasesResultMsg
	ReleaseDetails                              // details of a release: ReleaseDetailsResultMsg
)

// Request is a MusicBrainz request a popup makes: the result message of its
// command names it, so that the popup tells its own result from the result
// of a request made before, or by a popup closed since.
type Request struct {
	Kind RequestKind
	// Of is what is asked about: the query of a search, or the MusicBrainz
	// ID of the artist, release group or release.
	Of string
}

// Pending is the one request a popup waits for the result of: none at
// first, and none again once its result is taken.
type Pending struct {
	req Request
}

// Ask returns the command that sends req to client, and makes req the
// request waited for, in place of any other.
func (p *Pending) Ask(client Client, req Request) tea.Cmd {
	p.req = req
	return req.cmd(client)
}

// Take reports whether req is the request waited for. If it is, its result
// is taken: the popup waits for nothing until it makes another request.
func (p *Pending) Take(req Request) bool {
	if p.req.Kind == 0 || req != p.req {
		return false
	}
	p.req = Request{}
	return true
}

// cmd is the command that sends the request to client. Its result message
// names the request.
func (r Request) cmd(client Client) tea.Cmd {
	switch r.Kind {
	case SearchArtists:
		return func() tea.Msg {
			artists, err := client.SearchArtists(r.Of)
			return ArtistSearchResultMsg{Request: r, Artists: artists, Err: err}
		}
	case SearchReleaseGroups:
		return func() tea.Msg {
			groups, err := client.SearchReleaseGroups(r.Of)
			return SearchResultMsg{Request: r, ReleaseGroups: groups, Err: err}
		}
	case ArtistReleaseGroups:
		return func() tea.Msg {
			groups, err := client.GetArtistReleaseGroups(r.Of)
			return SearchResultMsg{Request: r, ReleaseGroups: groups, Err: err}
		}
	case ReleaseGroupReleases:
		return func() tea.Msg {
			releases, err := client.GetReleaseGroupReleases(r.Of)
			return ReleasesResultMsg{Request: r, Releases: releases, Err: err}
		}
	case ReleaseDetails:
		return func() tea.Msg {
			details, err := client.GetRelease(r.Of)
			return ReleaseDetailsResultMsg{Request: r, Details: details, Err: err}
		}
	}
	return nil
}
