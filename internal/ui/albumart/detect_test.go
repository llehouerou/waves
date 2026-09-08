package albumart

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The probe asks the terminal directly: a Kitty graphics query answered with
// "OK", then a Primary Device Attributes request whose parameter list contains
// "4" for sixel. Environment variables cannot be trusted: launching alacritty
// from ghostty leaves TERM_PROGRAM=ghostty in its environment (issue #30).
func TestParseProbeReply(t *testing.T) {
	tests := []struct {
		name      string
		reply     string
		wantKitty bool
		wantSixel bool
	}{
		{
			name:      "kitty",
			reply:     "\x1b_Gi=31;OK\x1b\\\x1b[?62;c",
			wantKitty: true,
		},
		{
			name:      "kitty refuses the query",
			reply:     "\x1b_Gi=31;ENOTSUPPORTED:no graphics\x1b\\\x1b[?62;c",
			wantKitty: false,
		},
		{
			name:      "foot",
			reply:     "\x1b[?62;4;22c",
			wantSixel: true,
		},
		{
			name:      "xterm with sixel",
			reply:     "\x1b[?63;1;2;4;6;9;15;22c",
			wantSixel: true,
		},
		{
			name:  "alacritty",
			reply: "\x1b[?6c",
		},
		{
			name:  "4 inside a longer param",
			reply: "\x1b[?62;14;40;422c",
		},
		{name: "empty"},
		{name: "garbage", reply: "not a reply"},
		{name: "truncated", reply: "\x1b[?62;4"},
		{name: "no leading csi", reply: "62;4;22c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kitty, sixel := parseProbeReply(tt.reply)
			if kitty != tt.wantKitty || sixel != tt.wantSixel {
				t.Errorf("parseProbeReply(%q) = (kitty=%v, sixel=%v), want (%v, %v)",
					tt.reply, kitty, sixel, tt.wantKitty, tt.wantSixel)
			}
		})
	}
}

// The reply ends at the DA1 terminator 'c'; anything after it belongs to the
// user's keystrokes and must not be swallowed.
func TestReadProbeReply(t *testing.T) {
	r := strings.NewReader("\x1b[?62;4;22cqrest")

	got := readProbeReply(r)

	if got != "\x1b[?62;4;22c" {
		t.Fatalf("readProbeReply = %q", got)
	}
	if left, _ := r.ReadByte(); left != 'q' {
		t.Errorf("reader advanced past the reply, next byte = %q", left)
	}
}

func TestReadProbeReply_EOFBeforeTerminator(t *testing.T) {
	if got := readProbeReply(strings.NewReader("\x1b[?62;4")); got != "\x1b[?62;4" {
		t.Errorf("readProbeReply = %q, want the partial reply", got)
	}
}

// A pipe is not a terminal: the probe must give up at once rather than block
// for its timeout or leave the terminal in raw mode.
func TestProbeGraphics_SkipsNonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	start := time.Now()
	kitty, sixel := probeGraphics(r, w, time.Second)

	if kitty || sixel {
		t.Errorf("probeGraphics on a non-TTY = (%v, %v), want (false, false)", kitty, sixel)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("probe took %v on a non-TTY, expected an immediate skip", elapsed)
	}
}

func typeName(p ImageProtocol) string {
	return fmt.Sprintf("%T", p)
}

func clearTerminalEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"WAVES_IMAGE_PROTOCOL", "TERM", "TERM_PROGRAM", "KITTY_WINDOW_ID",
		"GHOSTTY_RESOURCES_DIR", "KONSOLE_VERSION", "CONTOUR_PROFILE",
	} {
		t.Setenv(key, "")
	}
}

func TestDetect_Overrides(t *testing.T) {
	tests := []struct {
		override string
		want     string
	}{
		{"kitty", "*albumart.KittyProtocol"},
		{"sixel", "*albumart.SixelProtocol"},
		{"blocks", "*albumart.BlockProtocol"},
	}

	for _, tt := range tests {
		t.Run(tt.override, func(t *testing.T) {
			clearTerminalEnv(t)
			t.Setenv("WAVES_IMAGE_PROTOCOL", tt.override)

			if got := typeName(Detect()); got != tt.want {
				t.Errorf("Detect() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDetect_NoneDisablesArt(t *testing.T) {
	clearTerminalEnv(t)
	t.Setenv("WAVES_IMAGE_PROTOCOL", "none")

	if p := Detect(); p != nil {
		t.Errorf("Detect() = %s, want nil", typeName(p))
	}
}

// Terminal environment variables leak into child terminals, so they must not
// select a protocol: only the probe does. Under `go test` nothing answers it,
// so every terminal-looking environment falls back to blocks.
func TestDetect_IgnoresInheritedTerminalEnv(t *testing.T) {
	for _, env := range [][2]string{
		{"TERM", "xterm-kitty"},
		{"TERM_PROGRAM", "ghostty"},
		{"KITTY_WINDOW_ID", "1"},
		{"GHOSTTY_RESOURCES_DIR", "/usr/share/ghostty"},
		{"TERM", "foot"},
		{"TERM_PROGRAM", "iTerm.app"},
		{"TERM", "xterm-256color"},
	} {
		t.Run(env[0]+"="+env[1], func(t *testing.T) {
			clearTerminalEnv(t)
			t.Setenv(env[0], env[1])

			if got := typeName(Detect()); got != "*albumart.BlockProtocol" {
				t.Errorf("Detect() = %s, want *albumart.BlockProtocol", got)
			}
		})
	}
}
