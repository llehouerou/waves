//go:build unix

package albumart

import (
	"os"

	"golang.org/x/sys/unix"
)

// getCellSize returns the terminal cell dimensions in pixels
// by querying TIOCGWINSZ. Falls back to defaults if unavailable.
func getCellSize() (cellW, cellH int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return 8, 16
	}
	return int(ws.Xpixel) / int(ws.Col), int(ws.Ypixel) / int(ws.Row)
}
