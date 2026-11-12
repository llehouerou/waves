package core

import "github.com/llehouerou/waves/internal/musicbrainz"

// The MusicBrainz data a client handles, so that it imports the core only.
type (
	MBArtist         = musicbrainz.Artist
	MBReleaseGroup   = musicbrainz.ReleaseGroup
	MBRelease        = musicbrainz.Release
	MBReleaseDetails = musicbrainz.ReleaseDetails
)

// musicbrainzAPI is the part of musicbrainz.Client a client calls as it is.
type musicbrainzAPI interface {
	SearchArtists(query string) ([]MBArtist, error)
	SearchReleaseGroups(query string) ([]MBReleaseGroup, error)
	SearchReleaseGroupsByArtistAlbum(artist, album string) ([]MBReleaseGroup, error)
	GetArtistReleaseGroups(artistID string) ([]MBReleaseGroup, error)
	GetReleaseGroupReleases(releaseGroupID string) ([]MBRelease, error)
	GetRelease(mbid string) (*MBReleaseDetails, error)
	GetCoverArt(releaseMBID string) ([]byte, error)
}

// MusicBrainz is the MusicBrainz catalogue: artists, release groups,
// releases and their cover art. Every request of every client goes through
// it, so through the one rate limiter MusicBrainz asks for.
//
// Its methods wait for MusicBrainz to answer: a client calls them off its UI
// goroutine.
type MusicBrainz struct {
	musicbrainzAPI
}
