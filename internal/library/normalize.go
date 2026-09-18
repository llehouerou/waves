package library

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var foldAccents = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

var remasterSuffixes = []string{
	" (remastered)",
	" (remaster)",
	" - remastered",
	" [remastered]",
}

// NormalizeTitle normalizes an artist name or album/track title for comparison:
// lowercase, remaster suffixes trimmed, accents folded, punctuation dropped,
// separators collapsed to single spaces. Unicode-aware: non-latin scripts are
// preserved as-is.
//
// ponytail: ø/ð/ß/æ are single codepoints NFD cannot decompose, so they stay.
// Add a mapping table if those collide in practice.
func NormalizeTitle(s string) string {
	s = strings.ToLower(s)

	for _, suffix := range remasterSuffixes {
		s = strings.TrimSuffix(s, suffix)
	}

	if folded, _, err := transform.String(foldAccents, s); err == nil {
		s = folded
	}

	var b strings.Builder
	lastWasSpace := true // start true to trim leading spaces

	for _, r := range s {
		switch {
		case r == 'ʼ': // modifier letter apostrophe: punctuation, not a letter
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastWasSpace = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if !lastWasSpace {
				b.WriteRune(' ')
				lastWasSpace = true
			}
		}
	}

	return strings.TrimSpace(b.String())
}
