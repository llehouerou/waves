//go:build windows

// Package stderr provides a no-op implementation for Windows.
// Windows audio libraries don't produce the same stderr noise as ALSA.
package stderr

import "os"

// Start is a no-op on Windows.
func Start() error {
	return nil
}

// WriteOriginal writes to stderr.
func WriteOriginal(msg string) {
	_, _ = os.Stderr.WriteString(msg)
}

// Stop is a no-op on Windows.
func Stop() {}
