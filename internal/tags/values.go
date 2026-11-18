package tags

import (
	"slices"
	"strconv"
	"strings"

	"go.senan.xyz/taglib"
)

// valueSeparator joins the values of a tag that holds several.
const valueSeparator = "; "

// JoinValues joins the values of a tag into one string: in order, joined by
// valueSeparator, with the empty values and the repeated ones dropped. Every
// reader of every format gives its text tags through it, and every writer of
// a value made of several joins them with it.
func JoinValues(values []string) string {
	if len(values) == 1 {
		return values[0]
	}
	kept := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" && !slices.Contains(kept, v) {
			kept = append(kept, v)
		}
	}
	return strings.Join(kept, valueSeparator)
}

// SplitValues takes apart a string JoinValues joined into its values: none
// for an empty string. A string joined another way is one value.
func SplitValues(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, valueSeparator)
}

// firstValue returns the first of the values of a tag, or an empty string: a
// number keeps its first value.
func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// joinIfSeveral replaces *field with the values joined when the tag holds more
// than one. dhowden/tag, the first reader of a file, reads a tag with one value
// but loses the others: the reader of the format, which lists them, takes over.
func joinIfSeveral(field *string, values []string) {
	if len(values) > 1 {
		*field = JoinValues(values)
	}
}

// tagValues holds the values of each tag of a file, in file order: what
// taglib reads, or the Vorbis comments of a FLAC file.
type tagValues map[string][]string

// values returns the values of the first of the given keys the file holds.
func (t tagValues) values(keys ...string) []string {
	for _, key := range keys {
		if values := t[key]; len(values) > 0 {
			return values
		}
	}
	return nil
}

// get returns the values of the first of the given keys the file holds,
// joined, or an empty string.
func (t tagValues) get(keys ...string) string {
	return JoinValues(t.values(keys...))
}

// first returns the first value of a key.
func (t tagValues) first(key string) string {
	return firstValue(t[key])
}

// getInt returns the first value as an integer, or 0 if not found or invalid.
func (t tagValues) getInt(key string) int {
	n, _ := strconv.Atoi(t.first(key))
	return n
}

// parseNumberPair parses a track/disc number that may be "N" or "N/M" format.
func (t tagValues) parseNumberPair(key string) (num, total int) {
	s := t.first(key)
	if s == "" {
		return 0, 0
	}
	if idx := strings.Index(s, "/"); idx > 0 {
		num, _ = strconv.Atoi(s[:idx])
		total, _ = strconv.Atoi(s[idx+1:])
		return num, total
	}
	num, _ = strconv.Atoi(s)
	return num, 0
}

// joinBasicIfSeveral replaces each basic tag dhowden/tag read with its values
// joined, when the file holds several. values gives the values of a basic tag
// from the property name taglib reads it by.
func (t *Tag) joinBasicIfSeveral(values func(name string) []string) {
	joinIfSeveral(&t.Title, values(taglib.Title))
	joinIfSeveral(&t.Artist, values(taglib.Artist))
	joinIfSeveral(&t.AlbumArtist, values(taglib.AlbumArtist))
	joinIfSeveral(&t.Album, values(taglib.Album))
	joinIfSeveral(&t.Genre, values(taglib.Genre))
}

// basicValues gives the values of a basic tag, for joinBasicIfSeveral.
func (t tagValues) basicValues(name string) []string {
	return t.values(name)
}
