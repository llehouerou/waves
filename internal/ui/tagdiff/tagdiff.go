// Package tagdiff is the rule of the tag previews of the import and retag
// popups: a line of a preview compares a tag of the files an import or a
// retag writes with the value it writes on each of them. A popup gives the
// files it writes, the ones that have a track: a file with no track is left
// as it is, and counts in no line.
package tagdiff

import "github.com/llehouerou/waves/internal/core"

// File is a file an import or a retag writes, and the track it tags it as.
type File struct {
	Tags  *core.FileTags
	Track *core.MBTrack
}

// Values are the values of a tag on files, an empty one counted as a value
// among the others: the value every file holds, or how many different ones
// they hold.
type Values struct {
	Value    string // the value every file holds; "" when none has one, or when they differ
	Distinct int    // how many different values the files hold; 0 for no file
}

// Several reports whether the files hold different values.
func (v Values) Several() bool {
	return v.Distinct > 1
}

// Line is a tag across the files an import or a retag writes.
type Line struct {
	Held    Values // what the files hold
	Written Values // what is written on them
	Changed bool   // a file holds another value than the one written on it
}

// Compare is the line of the tag held reads from a file, of which written
// gives the value written on each file. The values are compared whole, an
// empty one included: a file with no value for a tag written with one
// changes, as does a file with a value for a tag written empty.
func Compare(files []File, held func(*core.FileTags) string, written func(File) string) Line {
	holds := make([]string, len(files))
	writes := make([]string, len(files))
	changed := false
	for i, f := range files {
		holds[i], writes[i] = held(f.Tags), written(f)
		if holds[i] != writes[i] {
			changed = true
		}
	}
	return Line{Held: summarize(holds), Written: summarize(writes), Changed: changed}
}

// Kept is the line of a tag each file keeps as it is: it never changes.
func Kept(files []File, held func(*core.FileTags) string) Line {
	holds := make([]string, len(files))
	for i, f := range files {
		holds[i] = held(f.Tags)
	}
	values := summarize(holds)
	return Line{Held: values, Written: values}
}

// Every writes value on every file: a value of the release or of its group.
func Every(value string) func(File) string {
	return func(File) string { return value }
}

// Track writes on each file the value of its track value gives.
func Track(value func(*core.MBTrack) string) func(File) string {
	return func(f File) string { return value(f.Track) }
}

func summarize(values []string) Values {
	distinct := make(map[string]bool)
	for _, v := range values {
		distinct[v] = true
	}
	if len(distinct) != 1 {
		return Values{Distinct: len(distinct)}
	}
	return Values{Value: values[0], Distinct: 1}
}
