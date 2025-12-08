package sourceutil

import "strings"

// PathSeparator is the standard separator for display paths.
const PathSeparator = " > "

// JoinPath joins path segments with the standard separator.
func JoinPath(parts ...string) string {
	return strings.Join(parts, PathSeparator)
}

// BuildPath builds a display path from root and additional segments.
func BuildPath(root string, segments ...string) string {
	if len(segments) == 0 {
		return root
	}
	return root + PathSeparator + strings.Join(segments, PathSeparator)
}
