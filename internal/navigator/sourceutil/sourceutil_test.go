//nolint:goconst // test cases intentionally repeat strings for readability
package sourceutil

import (
	"math"
	"testing"
)

func TestJoinPath(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "single part",
			parts:    []string{"Library"},
			expected: "Library",
		},
		{
			name:     "two parts",
			parts:    []string{"Library", "Artists"},
			expected: "Library > Artists",
		},
		{
			name:     "three parts",
			parts:    []string{"Library", "Artists", "Beatles"},
			expected: "Library > Artists > Beatles",
		},
		{
			name:     "empty parts",
			parts:    []string{},
			expected: "",
		},
		{
			name:     "parts with spaces",
			parts:    []string{"My Music", "Rock Albums", "Greatest Hits"},
			expected: "My Music > Rock Albums > Greatest Hits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinPath(tt.parts...)
			if result != tt.expected {
				t.Errorf("JoinPath(%v) = %q, want %q", tt.parts, result, tt.expected)
			}
		})
	}
}

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		segments []string
		expected string
	}{
		{
			name:     "root only",
			root:     "Library",
			segments: []string{},
			expected: "Library",
		},
		{
			name:     "root with one segment",
			root:     "Library",
			segments: []string{"Artists"},
			expected: "Library > Artists",
		},
		{
			name:     "root with multiple segments",
			root:     "Library",
			segments: []string{"Artists", "Beatles", "Abbey Road"},
			expected: "Library > Artists > Beatles > Abbey Road",
		},
		{
			name:     "empty root with segments",
			root:     "",
			segments: []string{"Artists", "Beatles"},
			expected: " > Artists > Beatles",
		},
		{
			name:     "root with spaces",
			root:     "My Library",
			segments: []string{"Rock Music"},
			expected: "My Library > Rock Music",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPath(tt.root, tt.segments...)
			if result != tt.expected {
				t.Errorf("BuildPath(%q, %v) = %q, want %q", tt.root, tt.segments, result, tt.expected)
			}
		})
	}
}

func TestPathSeparator(t *testing.T) {
	if PathSeparator != " > " {
		t.Errorf("PathSeparator = %q, want %q", PathSeparator, " > ")
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		prefix     string
		expected   []string
		expectedOk bool
	}{
		{
			name:       "valid library artist ID",
			id:         "library:artist:Beatles",
			prefix:     "library",
			expected:   []string{"artist", "Beatles"},
			expectedOk: true,
		},
		{
			name:       "valid library album ID",
			id:         "library:album:123",
			prefix:     "library",
			expected:   []string{"album", "123"},
			expectedOk: true,
		},
		{
			name:       "valid playlist ID",
			id:         "playlist:folder:456",
			prefix:     "playlist",
			expected:   []string{"folder", "456"},
			expectedOk: true,
		},
		{
			name:       "ID with four parts",
			id:         "library:track:123:extra",
			prefix:     "library",
			expected:   []string{"track", "123", "extra"},
			expectedOk: true,
		},
		{
			name:       "wrong prefix",
			id:         "library:artist:Beatles",
			prefix:     "playlist",
			expected:   nil,
			expectedOk: false,
		},
		{
			name:       "too short ID",
			id:         "library",
			prefix:     "library",
			expected:   nil,
			expectedOk: false,
		},
		{
			name:       "empty ID",
			id:         "",
			prefix:     "library",
			expected:   nil,
			expectedOk: false,
		},
		{
			name:       "empty prefix",
			id:         "library:artist:Beatles",
			prefix:     "",
			expected:   nil,
			expectedOk: false,
		},
		{
			name:       "ID with colons in value",
			id:         "library:artist:AC:DC",
			prefix:     "library",
			expected:   []string{"artist", "AC", "DC"},
			expectedOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseID(tt.id, tt.prefix)

			if ok != tt.expectedOk {
				t.Errorf("ParseID(%q, %q) ok = %v, want %v", tt.id, tt.prefix, ok, tt.expectedOk)
			}

			if tt.expectedOk {
				if len(result) != len(tt.expected) {
					t.Errorf("ParseID(%q, %q) = %v, want %v", tt.id, tt.prefix, result, tt.expected)
					return
				}
				for i, v := range tt.expected {
					if result[i] != v {
						t.Errorf("ParseID(%q, %q)[%d] = %q, want %q", tt.id, tt.prefix, i, result[i], v)
					}
				}
			} else if result != nil {
				t.Errorf("ParseID(%q, %q) = %v, want nil", tt.id, tt.prefix, result)
			}
		})
	}
}

func TestFormatID(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		parts    []string
		expected string
	}{
		{
			name:     "library artist",
			prefix:   "library",
			parts:    []string{"artist", "Beatles"},
			expected: "library:artist:Beatles",
		},
		{
			name:     "library album",
			prefix:   "library",
			parts:    []string{"album", "123"},
			expected: "library:album:123",
		},
		{
			name:     "playlist folder",
			prefix:   "playlist",
			parts:    []string{"folder", "456"},
			expected: "playlist:folder:456",
		},
		{
			name:     "prefix only",
			prefix:   "library",
			parts:    []string{},
			expected: "library",
		},
		{
			name:     "single part",
			prefix:   "library",
			parts:    []string{"root"},
			expected: "library:root",
		},
		{
			name:     "many parts",
			prefix:   "nav",
			parts:    []string{"a", "b", "c", "d"},
			expected: "nav:a:b:c:d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatID(tt.prefix, tt.parts...)
			if result != tt.expected {
				t.Errorf("FormatID(%q, %v) = %q, want %q", tt.prefix, tt.parts, result, tt.expected)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expected   int64
		expectedOk bool
	}{
		{"valid positive", "123", 123, true},
		{"valid negative", "-456", -456, true},
		{"valid zero", "0", 0, true},
		{"valid large", "9223372036854775807", math.MaxInt64, true},
		{"valid min", "-9223372036854775808", math.MinInt64, true},
		{"invalid empty", "", 0, false},
		{"invalid letters", "abc", 0, false},
		{"invalid float", "12.34", 0, false},
		{"invalid mixed", "12abc", 0, false},
		{"invalid spaces", " 123", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseInt64(tt.input)

			if ok != tt.expectedOk {
				t.Errorf("ParseInt64(%q) ok = %v, want %v", tt.input, ok, tt.expectedOk)
			}

			if result != tt.expected {
				t.Errorf("ParseInt64(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expected   int
		expectedOk bool
	}{
		{"valid positive", "123", 123, true},
		{"valid negative", "-456", -456, true},
		{"valid zero", "0", 0, true},
		{"invalid empty", "", 0, false},
		{"invalid letters", "abc", 0, false},
		{"invalid float", "12.34", 0, false},
		{"invalid mixed", "12abc", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseInt(tt.input)

			if ok != tt.expectedOk {
				t.Errorf("ParseInt(%q) ok = %v, want %v", tt.input, ok, tt.expectedOk)
			}

			if result != tt.expected {
				t.Errorf("ParseInt(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"positive", 123, "123"},
		{"negative", -456, "-456"},
		{"zero", 0, "0"},
		{"large", math.MaxInt64, "9223372036854775807"},
		{"min", math.MinInt64, "-9223372036854775808"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatInt64(tt.input)
			if result != tt.expected {
				t.Errorf("FormatInt64(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"positive", 123, "123"},
		{"negative", -456, "-456"},
		{"zero", 0, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatInt(tt.input)
			if result != tt.expected {
				t.Errorf("FormatInt(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatAndParseRoundTrip(t *testing.T) {
	// Test that FormatInt64 and ParseInt64 are inverses
	values := []int64{0, 1, -1, 123, -456, math.MaxInt64, math.MinInt64}

	for _, v := range values {
		formatted := FormatInt64(v)
		parsed, ok := ParseInt64(formatted)
		if !ok {
			t.Errorf("ParseInt64(FormatInt64(%d)) failed to parse", v)
			continue
		}
		if parsed != v {
			t.Errorf("ParseInt64(FormatInt64(%d)) = %d, want %d", v, parsed, v)
		}
	}
}

func TestFormatIDAndParseIDRoundTrip(t *testing.T) {
	tests := []struct {
		prefix string
		parts  []string
	}{
		{"library", []string{"artist", "Beatles"}},
		{"playlist", []string{"folder", "123"}},
		{"nav", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		id := FormatID(tt.prefix, tt.parts...)
		parsed, ok := ParseID(id, tt.prefix)

		if !ok {
			t.Errorf("ParseID(FormatID(%q, %v)) failed to parse", tt.prefix, tt.parts)
			continue
		}

		if len(parsed) != len(tt.parts) {
			t.Errorf("ParseID(FormatID(%q, %v)) = %v, want %v", tt.prefix, tt.parts, parsed, tt.parts)
			continue
		}

		for i, v := range tt.parts {
			if parsed[i] != v {
				t.Errorf("ParseID(FormatID(%q, %v))[%d] = %q, want %q", tt.prefix, tt.parts, i, parsed[i], v)
			}
		}
	}
}
