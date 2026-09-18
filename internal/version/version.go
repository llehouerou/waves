// Package version holds the build version and the shared outbound User-Agent.
package version

// Version is the build version, set via ldflags at release time.
var Version = "dev"

// UserAgent returns the identifying User-Agent for all outbound HTTP requests.
// MetaBrainz services (MusicBrainz, ListenBrainz) require one.
func UserAgent() string {
	return "waves/" + Version + " (+https://github.com/llehouerou/waves)"
}
