package albumart

import (
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// probeTimeout bounds the wait for a terminal that never answers the query.
const probeTimeout = 100 * time.Millisecond

// graphicsQuery asks the terminal what it can do, in one round trip:
//   - a Kitty graphics query for a 1x1 RGB image, answered with ";OK" only by
//     terminals that implement the protocol;
//   - a Primary Device Attributes request, whose reply lists "4" when sixel is
//     supported and terminates the whole exchange with 'c'.
//
// Terminals answer in order, so the DA1 reply also tells us the Kitty query has
// been answered (or ignored).
const graphicsQuery = "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\\x1b[c"

// Detect returns the best available ImageProtocol for the current terminal.
// It never returns nil except for an explicit "none" override: terminals with no
// graphics protocol fall back to unicode half blocks.
//
// Detection asks the terminal itself. Environment variables are useless here:
// a terminal launched from another one inherits its variables (alacritty
// started from ghostty keeps TERM_PROGRAM=ghostty), and TERM says nothing about
// sixel support.
//
// The WAVES_IMAGE_PROTOCOL environment variable can override detection:
//   - "kitty": force Kitty protocol
//   - "sixel": force Sixel protocol
//   - "blocks": force the unicode half-block renderer
//   - "none": disable image display
func Detect() ImageProtocol {
	switch os.Getenv("WAVES_IMAGE_PROTOCOL") {
	case "kitty":
		return &KittyProtocol{}
	case "sixel":
		return NewSixelProtocol()
	case "blocks":
		return NewBlockProtocol()
	case "none":
		return nil
	}

	kitty, sixel := probeGraphics(os.Stdin, os.Stdout, probeTimeout)
	switch {
	case kitty:
		return &KittyProtocol{}
	case sixel:
		return NewSixelProtocol()
	default:
		return NewBlockProtocol()
	}
}

// probeGraphics asks the terminal which graphics protocols it supports. It
// degrades to (false, false) on a non-TTY, an error, or a terminal that never
// answers within the timeout — the block renderer then handles the art.
func probeGraphics(in *os.File, out io.Writer, timeout time.Duration) (kitty, sixel bool) {
	fd := in.Fd()
	if !term.IsTerminal(fd) {
		return false, false
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return false, false
	}
	defer term.Restore(fd, state) //nolint:errcheck // nothing useful to do if restore fails

	if _, err := io.WriteString(out, graphicsQuery); err != nil {
		return false, false
	}

	return parseProbeReply(readProbeReply(newDeadlineReader(fd, timeout)))
}

// readProbeReply reads one byte at a time up to the DA1 terminator 'c', so
// keystrokes typed during startup stay in the input buffer.
func readProbeReply(r io.Reader) string {
	var sb strings.Builder
	buf := make([]byte, 1)
	for sb.Len() < 256 {
		n, err := r.Read(buf)
		if n > 0 {
			sb.WriteByte(buf[0])
			if buf[0] == 'c' {
				break
			}
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}

// parseProbeReply reads the terminal's answer to graphicsQuery: a Kitty
// graphics response (ESC _G ... ; OK ESC \) and a DA1 reply
// (ESC [ ? p1 ; p2 ; ... c) listing attribute 4 for sixel.
func parseProbeReply(reply string) (kitty, sixel bool) {
	return strings.Contains(reply, "\x1b_G") && strings.Contains(reply, ";OK"),
		parseDA1Sixel(reply)
}

func parseDA1Sixel(reply string) bool {
	start := strings.Index(reply, "\x1b[?")
	end := strings.LastIndex(reply, "c")
	if start < 0 || end < start {
		return false
	}

	return slices.Contains(strings.Split(reply[start+len("\x1b[?"):end], ";"), "4")
}
