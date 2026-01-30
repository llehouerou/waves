// Package lyrics provides lyrics parsing and sourcing.
package lyrics

import (
	"bufio"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Line represents a single timestamped lyric line.
type Line struct {
	Time time.Duration
	Text string
}

// Lyrics contains parsed lyrics with optional metadata.
type Lyrics struct {
	Lines  []Line
	Title  string
	Artist string
	Album  string
}

// LineAt returns the index of the lyric line at the given playback position.
// Returns -1 if no line is active yet or if lyrics are unsynced.
func (l *Lyrics) LineAt(pos time.Duration) int {
	if len(l.Lines) == 0 || !l.IsSynced() {
		return -1
	}

	// Find the last line that starts at or before pos
	idx := -1
	for i, line := range l.Lines {
		if line.Time <= pos {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// Regular expressions for parsing LRC format
var (
	// Matches timestamps like [00:12.34] or [00:12:34] or [00:12]
	timestampRe = regexp.MustCompile(`\[(\d+):(\d+)(?:[.:](\d+))?\]`)

	// Matches metadata tags like [ar:Artist Name]
	metadataRe = regexp.MustCompile(`^\[([a-z]+):(.+)\]$`)
)

// ParseLRC parses LRC format lyrics from a reader.
func ParseLRC(r io.Reader) (*Lyrics, error) {
	lyrics := &Lyrics{}
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as metadata
		if meta := metadataRe.FindStringSubmatch(line); meta != nil {
			tag := strings.ToLower(meta[1])
			value := strings.TrimSpace(meta[2])
			switch tag {
			case "ar":
				lyrics.Artist = value
			case "ti":
				lyrics.Title = value
			case "al":
				lyrics.Album = value
			}
			continue
		}

		// Try to parse as timestamped lyric line
		// LRC can have multiple timestamps for the same text: [00:12.34][00:45.67]Text
		matches := timestampRe.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}

		// Extract the text after all timestamps
		lastMatch := matches[len(matches)-1]
		text := strings.TrimSpace(line[lastMatch[1]:])

		// Parse each timestamp and create a Line
		for _, match := range matches {
			ts, err := parseTimestamp(line[match[0]:match[1]])
			if err != nil {
				continue
			}
			lyrics.Lines = append(lyrics.Lines, Line{
				Time: ts,
				Text: text,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Sort lines by timestamp
	sort.Slice(lyrics.Lines, func(i, j int) bool {
		return lyrics.Lines[i].Time < lyrics.Lines[j].Time
	})

	return lyrics, nil
}

// parseTimestamp parses a timestamp like [00:12.34] into a Duration.
func parseTimestamp(s string) (time.Duration, error) {
	matches := timestampRe.FindStringSubmatch(s)
	if matches == nil {
		return 0, nil
	}

	minutes, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, err
	}

	var millis int
	if matches[3] != "" {
		millis, err = strconv.Atoi(matches[3])
		if err != nil {
			return 0, err
		}
		// Handle both .xx (centiseconds) and .xxx (milliseconds)
		if len(matches[3]) == 2 {
			millis *= 10 // Convert centiseconds to milliseconds
		}
	}

	return time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(millis)*time.Millisecond, nil
}
