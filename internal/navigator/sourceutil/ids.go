package sourceutil

import (
	"strconv"
	"strings"
)

// ParseID splits a node ID into parts and validates the prefix.
// Returns the parts after the prefix (type and data) and true if valid.
// Example: "library:artist:Beatles" with prefix "library" returns ["artist", "Beatles"], true
func ParseID(id, prefix string) ([]string, bool) {
	parts := strings.SplitN(id, ":", 4)
	if len(parts) < 2 || parts[0] != prefix {
		return nil, false
	}
	return parts[1:], true
}

// FormatID builds a node ID from prefix and parts.
// Example: FormatID("library", "artist", "Beatles") returns "library:artist:Beatles"
func FormatID(prefix string, parts ...string) string {
	all := make([]string, 0, 1+len(parts))
	all = append(all, prefix)
	all = append(all, parts...)
	return strings.Join(all, ":")
}

// ParseInt64 parses a string to int64, returning 0 and false on error.
func ParseInt64(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// ParseInt parses a string to int, returning 0 and false on error.
func ParseInt(s string) (int, bool) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

// FormatInt64 formats an int64 as a string.
func FormatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}

// FormatInt formats an int as a string.
func FormatInt(v int) string {
	return strconv.Itoa(v)
}
