package icons

// Style represents the icon style to use.
type Style string

const (
	StyleNerd    Style = "nerd"
	StyleUnicode Style = "unicode"
	StyleNone    Style = "none"
)

// Icons holds the icon characters for the current style.
type Icons struct {
	Folder    string
	Audio     string
	Artist    string
	Album     string
	Playlist  string
	Shuffle   string
	RepeatAll string
	RepeatOne string
	Favorite  string
}

var (
	nerdIcons = Icons{
		Folder:    "\uf07b ", // nf-fa-folder
		Audio:     "\uf001 ", // nf-fa-music
		Artist:    "\uf007 ", // nf-fa-user
		Album:     "󰀥 ",      // nf-md-album
		Playlist:  "󰲸 ",      // nf-md-playlist_music
		Shuffle:   "󰒟",       // nf-md-shuffle
		RepeatAll: "󰑖",       // nf-md-repeat
		RepeatOne: "󰑘",       // nf-md-repeat_once
		Favorite:  "󰣐",       // nf-md-heart
	}

	unicodeIcons = Icons{
		Folder:    "📁 ",
		Audio:     "🎵 ",
		Artist:    "👤 ",
		Album:     "💿 ",
		Playlist:  "📋 ",
		Shuffle:   "🔀",
		RepeatAll: "🔁",
		RepeatOne: "🔂",
		Favorite:  "♥",
	}

	noneIcons = Icons{
		Folder:    "/",
		Audio:     "",
		Artist:    "",
		Album:     "",
		Playlist:  "",
		Shuffle:   "[S]",
		RepeatAll: "[R]",
		RepeatOne: "[1]",
		Favorite:  "*",
	}

	// current holds the active icon set
	current = noneIcons
)

// Init initializes the icons based on the style.
// Call this once at startup with the config value.
func Init(style string) {
	switch Style(style) {
	case StyleNerd:
		current = nerdIcons
	case StyleUnicode:
		current = unicodeIcons
	case StyleNone:
		current = noneIcons
	default:
		current = noneIcons
	}
}

// Folder returns the folder indicator.
// For "none" style, this is a suffix ("/").
// For other styles, this is a prefix icon.
func Folder() string {
	return current.Folder
}

// IsPrefix returns true if the folder icon should be prepended.
func IsPrefix() bool {
	return current != noneIcons
}

// FormatDir formats a directory name with the appropriate icon.
func FormatDir(name string) string {
	if current == noneIcons {
		return name + current.Folder
	}
	return current.Folder + name
}

// FormatAudio formats an audio file name with the appropriate icon.
func FormatAudio(name string) string {
	if current == noneIcons {
		return name
	}
	return current.Audio + name
}

// FormatArtist formats an artist name with the appropriate icon.
func FormatArtist(name string) string {
	if current == noneIcons {
		return name
	}
	return current.Artist + name
}

// FormatAlbum formats an album name with the appropriate icon.
func FormatAlbum(name string) string {
	if current == noneIcons {
		return name
	}
	return current.Album + name
}

// FormatPlaylist formats a playlist name with the appropriate icon.
func FormatPlaylist(name string) string {
	if current == noneIcons {
		return name
	}
	return current.Playlist + name
}

// Shuffle returns the shuffle icon.
func Shuffle() string {
	return current.Shuffle
}

// RepeatAll returns the repeat all icon.
func RepeatAll() string {
	return current.RepeatAll
}

// RepeatOne returns the repeat one icon.
func RepeatOne() string {
	return current.RepeatOne
}

// Favorite returns the favorite/heart icon.
func Favorite() string {
	return current.Favorite
}
