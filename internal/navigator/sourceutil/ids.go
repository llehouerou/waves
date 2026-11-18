package sourceutil

import (
	"strconv"
	"strings"
)

// An ID is its parts joined with ":". Each part is escaped first, so that it
// may hold any character: "%" is written "%25", ":" "%3A", a line feed
// "%0A" and a carriage return "%0D". ParseID gives back exactly the parts
// FormatID was given, and an ID is a single line.
var (
	escaper   = strings.NewReplacer("%", "%25", ":", "%3A", "\n", "%0A", "\r", "%0D")
	unescaper = strings.NewReplacer("%25", "%", "%3A", ":", "%0A", "\n", "%0D", "\r")
)

// ParseID splits a node ID built by FormatID into its parts and validates
// the prefix. Returns the parts after the prefix (type and data) and true if
// valid.
// Example: "library:artist:AC%3ADC" with prefix "library" returns ["artist", "AC:DC"], true
func ParseID(id, prefix string) ([]string, bool) {
	parts := strings.Split(id, ":")
	if len(parts) < 2 || unescaper.Replace(parts[0]) != prefix {
		return nil, false
	}
	parts = parts[1:]
	for i, p := range parts {
		parts[i] = unescaper.Replace(p)
	}
	return parts, true
}

// FormatID builds a node ID from prefix and parts, which may hold any
// character.
// Example: FormatID("library", "artist", "AC:DC") returns "library:artist:AC%3ADC"
func FormatID(prefix string, parts ...string) string {
	all := make([]string, 0, 1+len(parts))
	all = append(all, escaper.Replace(prefix))
	for _, p := range parts {
		all = append(all, escaper.Replace(p))
	}
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
