// Package export provides functionality for exporting music to external devices.
package export

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Volume represents a mounted removable device.
type Volume struct {
	Label     string
	UUID      string
	MountPath string
	Device    string
}

// String returns a display string for the volume.
func (v Volume) String() string {
	if v.Label != "" {
		return fmt.Sprintf("%s (%s)", v.Label, v.MountPath)
	}
	return v.MountPath
}

// removableMediaPrefixes are paths where removable media is typically mounted.
var removableMediaPrefixes = []string{
	"/media/",
	"/mnt/",
	"/run/media/",
}

// DetectVolumes scans for mounted removable media.
func DetectVolumes() ([]Volume, error) {
	mounts, err := parseMounts()
	if err != nil {
		return nil, err
	}

	volumes := make([]Volume, 0, len(mounts))
	for dev, mountPath := range mounts {
		uuid := lookupUUID(dev)
		label := lookupLabel(dev)
		if uuid == "" {
			continue // Skip if we can't identify the device
		}

		volumes = append(volumes, Volume{
			Label:     label,
			UUID:      uuid,
			MountPath: mountPath,
			Device:    dev,
		})
	}

	return volumes, nil
}

// parseMounts reads /proc/mounts and returns device->mountPath for removable media.
func parseMounts() (map[string]string, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("open /proc/mounts: %w", err)
	}
	defer f.Close()

	mounts := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		dev, path, ok := parseMountLine(scanner.Text())
		if ok {
			mounts[dev] = path
		}
	}
	return mounts, scanner.Err()
}

// parseMountLine parses a line from /proc/mounts.
// Returns device, mountPath, and whether this is a removable media mount.
func parseMountLine(line string) (device, mountPath string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}

	device = fields[0]
	mountPath = unescapeMountPath(fields[1])

	// Only include removable media paths
	for _, prefix := range removableMediaPrefixes {
		if strings.HasPrefix(mountPath, prefix) {
			return device, mountPath, true
		}
	}
	return "", "", false
}

// unescapeMountPath handles octal escapes in mount paths (e.g., \040 for space).
func unescapeMountPath(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			// Try to parse octal escape
			if oct := s[i+1 : i+4]; isOctal(oct) {
				var val byte
				for _, c := range oct {
					val = val*8 + byte(c-'0')
				}
				result.WriteByte(val)
				i += 3
				continue
			}
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

func isOctal(s string) bool {
	for _, c := range s {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}

// lookupUUID finds the UUID for a device by checking /dev/disk/by-uuid symlinks.
func lookupUUID(device string) string {
	return lookupDiskSymlink("/dev/disk/by-uuid", device)
}

// lookupLabel finds the label for a device by checking /dev/disk/by-label symlinks.
func lookupLabel(device string) string {
	return lookupDiskSymlink("/dev/disk/by-label", device)
}

func lookupDiskSymlink(dir, device string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		link := filepath.Join(dir, entry.Name())
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		// Resolve relative symlink
		resolved := filepath.Join(dir, target)
		resolved, err = filepath.EvalSymlinks(resolved)
		if err != nil {
			continue
		}
		if resolved == device {
			return entry.Name()
		}
	}
	return ""
}
