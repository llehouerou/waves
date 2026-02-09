package library

import (
	"regexp"
	"strings"
)

var (
	punctuationRe   = regexp.MustCompile(`[^\w\s]`)
	multipleSpaceRe = regexp.MustCompile(`\s+`)
)

// NormalizeTitle normalizes a title for comparison by:
// - Converting to lowercase
// - Replacing punctuation with spaces
// - Normalizing whitespace
func NormalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = punctuationRe.ReplaceAllString(s, " ")
	s = multipleSpaceRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}
