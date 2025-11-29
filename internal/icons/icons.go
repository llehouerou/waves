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
	Folder string
	Audio  string
}

var (
	nerdIcons = Icons{
		Folder: "\uf07b ", // nf-fa-folder
		Audio:  "\uf001 ", // nf-fa-music
	}

	unicodeIcons = Icons{
		Folder: "📁 ",
		Audio:  "🎵 ",
	}

	noneIcons = Icons{
		Folder: "/",
		Audio:  "",
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
