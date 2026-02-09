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
	Folder       string
	Audio        string
	Artist       string
	Album        string
	Playlist     string
	Shuffle      string
	RepeatAll    string
	RepeatOne    string
	Radio        string
	Favorite     string
	Play         string
	Pause        string
	VolumeHigh   string
	VolumeMedium string
	VolumeLow    string
	VolumeOff    string
	VolumeMute   string
	InLibrary    string
}

var (
	nerdIcons = Icons{
		Folder:       "\uf07b ", // nf-fa-folder
		Audio:        "\uf001 ", // nf-fa-music
		Artist:       "\uf007 ", // nf-fa-user
		Album:        "󰀥 ",      // nf-md-album
		Playlist:     "󰲸 ",      // nf-md-playlist_music
		Shuffle:      "󰒟",       // nf-md-shuffle
		RepeatAll:    "󰑖",       // nf-md-repeat
		RepeatOne:    "󰑘",       // nf-md-repeat_once
		Radio:        "󰐹",       // nf-md-radio
		Favorite:     "󰣐",       // nf-md-heart
		Play:         "󰐊",       // nf-md-play
		Pause:        "󰏤",       // nf-md-pause
		VolumeHigh:   "󰕾",       // nf-md-volume_high
		VolumeMedium: "󰖀",       // nf-md-volume_medium
		VolumeLow:    "󰕿",       // nf-md-volume_low
		VolumeOff:    "󰝟",       // nf-md-volume_off
		VolumeMute:   "󰖁",       // nf-md-volume_mute
		InLibrary:    "󰄬",       // nf-md-check
	}

	unicodeIcons = Icons{
		Folder:       "📁 ",
		Audio:        "🎵 ",
		Artist:       "👤 ",
		Album:        "💿 ",
		Playlist:     "📋 ",
		Shuffle:      "🔀",
		RepeatAll:    "🔁",
		RepeatOne:    "🔂",
		Radio:        "📻",
		Favorite:     "♥",
		Play:         "▶",
		Pause:        "⏸",
		VolumeHigh:   "🔊",
		VolumeMedium: "🔉",
		VolumeLow:    "🔈",
		VolumeOff:    "🔇",
		VolumeMute:   "🔇",
		InLibrary:    "✓",
	}

	noneIcons = Icons{
		Folder:       "/",
		Audio:        "",
		Artist:       "",
		Album:        "",
		Playlist:     "",
		Shuffle:      "[S]",
		RepeatAll:    "[R]",
		RepeatOne:    "[1]",
		Radio:        "[~]",
		Favorite:     "*",
		Play:         "[>]",
		Pause:        "[||]",
		VolumeHigh:   "[H]",
		VolumeMedium: "[M]",
		VolumeLow:    "[L]",
		VolumeOff:    "[0]",
		VolumeMute:   "[X]",
		InLibrary:    "*",
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

// Radio returns the radio icon.
func Radio() string {
	return current.Radio
}

// Play returns the play icon.
func Play() string {
	return current.Play
}

// Pause returns the pause icon.
func Pause() string {
	return current.Pause
}

// VolumeIcon returns the appropriate volume icon based on level (0.0-1.0).
func VolumeIcon(level float64, muted bool) string {
	if muted {
		return current.VolumeMute
	}
	switch {
	case level <= 0:
		return current.VolumeOff
	case level <= 0.33:
		return current.VolumeLow
	case level <= 0.66:
		return current.VolumeMedium
	default:
		return current.VolumeHigh
	}
}

// InLibrary returns the "in library" check icon.
func InLibrary() string {
	return current.InLibrary
}
