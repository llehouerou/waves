package albumart

import (
	"os"
	"strings"
)

// Detect returns the best available ImageProtocol for the current terminal,
// or nil if no image protocol is supported.
//
// The WAVES_IMAGE_PROTOCOL environment variable can override detection:
//   - "kitty": force Kitty protocol
//   - "sixel": force Sixel protocol
//   - "none": disable image display
func Detect() ImageProtocol {
	if override := os.Getenv("WAVES_IMAGE_PROTOCOL"); override != "" {
		switch override {
		case "kitty":
			return &KittyProtocol{}
		case "sixel":
			return NewSixelProtocol()
		case "none":
			return nil
		}
	}

	if IsKittySupported() {
		return &KittyProtocol{}
	}

	if IsSixelSupported() {
		return NewSixelProtocol()
	}

	return nil
}

// IsKittySupported checks if the terminal supports Kitty graphics protocol.
func IsKittySupported() bool {
	// Contour sets CONTOUR_PROFILE but doesn't support Kitty protocol.
	// Check early because parent terminal env vars (e.g. GHOSTTY_RESOURCES_DIR)
	// can leak into Contour when launched from a Kitty-capable terminal.
	if os.Getenv("CONTOUR_PROFILE") != "" {
		return false
	}

	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("TERM") == "xterm-kitty" {
		return true
	}
	if os.Getenv("TERM_PROGRAM") == "WezTerm" {
		return true
	}
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}
	if version := os.Getenv("KONSOLE_VERSION"); version != "" {
		if len(version) >= 4 && version[:4] >= "2204" {
			return true
		}
	}
	return strings.Contains(os.Getenv("TERM"), "kitty")
}

// IsSixelSupported checks if the terminal supports Sixel graphics.
func IsSixelSupported() bool {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")

	// foot (Wayland terminal)
	if term == "foot" || term == "foot-extra" {
		return true
	}

	// VS Code integrated terminal
	if termProgram == "vscode" {
		return true
	}

	// mintty (Windows terminal)
	if termProgram == "mintty" {
		return true
	}

	// iTerm2
	if termProgram == "iTerm.app" {
		return true
	}

	// Contour terminal
	if termProgram == "contour" || os.Getenv("CONTOUR_PROFILE") != "" {
		return true
	}

	// xterm with VT340 support (common sixel-capable config)
	if term == "xterm" || strings.HasPrefix(term, "xterm-") {
		// xterm supports sixel when built with --enable-sixel-graphics
		// We can't easily detect this, but TERM=xterm is a reasonable hint
		// when no other protocol matched above.
		// Note: IsKittySupported() is checked first, so xterm-kitty won't reach here.
		return true
	}

	return false
}
