package render

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "no truncation needed",
			input:    "hello",
			maxWidth: 10,
			want:     "hello",
		},
		{
			name:     "exact fit",
			input:    "hello",
			maxWidth: 5,
			want:     "hello",
		},
		{
			name:     "truncation with ellipsis",
			input:    "hello world",
			maxWidth: 8,
			want:     "hello...",
		},
		{
			name:     "very short max width",
			input:    "hello",
			maxWidth: 3,
			want:     "...",
		},
		{
			name:     "empty string",
			input:    "",
			maxWidth: 10,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestTruncateEllipsis(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "no truncation needed",
			input:    "hello",
			maxWidth: 10,
			want:     "hello",
		},
		{
			name:     "truncation with single ellipsis",
			input:    "hello world",
			maxWidth: 8,
			want:     "hello w…",
		},
		{
			name:     "empty string",
			input:    "",
			maxWidth: 10,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateEllipsis(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("TruncateEllipsis(%q, %d) = %q, want %q", tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestPad(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "padding needed",
			input: "hello",
			width: 10,
			want:  "hello     ",
		},
		{
			name:  "exact width",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "already wider",
			input: "hello world",
			width: 5,
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			width: 5,
			want:  "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Pad(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("Pad(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestTruncateAndPad(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		wantLen  int
		contains string
	}{
		{
			name:     "truncate and pad",
			input:    "hello world",
			width:    8,
			wantLen:  8,
			contains: "...",
		},
		{
			name:     "just pad",
			input:    "hi",
			width:    8,
			wantLen:  8,
			contains: "hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateAndPad(tt.input, tt.width)
			if len(got) != tt.wantLen {
				t.Errorf("TruncateAndPad(%q, %d) length = %d, want %d", tt.input, tt.width, len(got), tt.wantLen)
			}
			if !strings.Contains(got, tt.contains) {
				t.Errorf("TruncateAndPad(%q, %d) = %q, should contain %q", tt.input, tt.width, got, tt.contains)
			}
		})
	}
}

func TestRow(t *testing.T) {
	tests := []struct {
		name    string
		left    string
		right   string
		width   int
		wantLen int
	}{
		{
			name:    "basic row",
			left:    "left",
			right:   "right",
			width:   20,
			wantLen: 20,
		},
		{
			name:    "tight fit",
			left:    "left",
			right:   "right",
			width:   10,
			wantLen: 10, // minimum gap of 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Row(tt.left, tt.right, tt.width)
			if len(got) < tt.wantLen {
				t.Errorf("Row(%q, %q, %d) length = %d, want >= %d", tt.left, tt.right, tt.width, len(got), tt.wantLen)
			}
			if !strings.HasPrefix(got, tt.left) {
				t.Errorf("Row(%q, %q, %d) should start with %q", tt.left, tt.right, tt.width, tt.left)
			}
			if !strings.HasSuffix(got, tt.right) {
				t.Errorf("Row(%q, %q, %d) should end with %q", tt.left, tt.right, tt.width, tt.right)
			}
		})
	}
}

func TestSeparator(t *testing.T) {
	got := Separator(10)
	want := "──────────"
	if got != want {
		t.Errorf("Separator(10) = %q, want %q", got, want)
	}
}

func TestEmptyLine(t *testing.T) {
	got := EmptyLine(5)
	want := "     "
	if got != want {
		t.Errorf("EmptyLine(5) = %q, want %q", got, want)
	}
}

// A panel with no room left draws no line, rather than panicking (issue #179).
func TestSeparatorAndEmptyLine_NoWidthIsEmpty(t *testing.T) {
	for _, width := range []int{0, -1, -5} {
		if got := Separator(width); got != "" {
			t.Errorf("Separator(%d) = %q, want empty", width, got)
		}
		if got := EmptyLine(width); got != "" {
			t.Errorf("EmptyLine(%d) = %q, want empty", width, got)
		}
	}
}

// Cut ends a text too wide for its room with "..." where it fits, between two
// characters, and is never wider than the room, however small (issue #203).
func TestCut(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"ASCII that fits", "hello", 5, "hello"},
		{"Cyrillic that fits", "Группа крови", 12, "Группа крови"},
		{"ASCII", "hello world", 8, "hello..."},
		{"Cyrillic", "Группа крови", 9, "Группа..."},
		{"accented Latin", "Ágætis byrjun", 6, "Ágæ..."},
		{"CJK filling the room", "坂本龍一", 7, "坂本..."},
		{"CJK one column short", "坂本龍一", 6, "坂..."},
		{"room for the dots only", "Группа", 3, "..."},
		{"no room for the dots", "Группа", 2, "Гр"},
		{"one column", "Группа", 1, "Г"},
		{"wide character in one column", "坂本", 1, ""},
		{"no room", "Группа", 0, ""},
		{"negative room", "Группа", -4, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Cut(tt.input, tt.width); got != tt.want {
				t.Errorf("Cut(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

// CutLeft keeps the end of a text too wide for its room and begins it with
// "..." where it fits, between two characters, and is never wider than the
// room, however small (issue #313).
func TestCutLeft(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"ASCII that fits", "hello", 5, "hello"},
		{"Cyrillic that fits", "Группа крови", 20, "Группа крови"},
		{"CJK that fits", "音楽\\坂本龍一", 13, "音楽\\坂本龍一"},
		{"ASCII", "hello world", 8, "...world"},
		{"Cyrillic", "Группа крови", 9, "... крови"},
		{"accented Latin", "Ágætis byrjun", 6, "...jun"},
		{"CJK filling the room", "坂本龍一", 7, "...龍一"},
		{"CJK one column short", "坂本龍一", 6, "...一"},
		{"room for the dots only", "Группа", 3, "..."},
		{"no room for the dots", "Группа", 2, "па"},
		{"one column", "Группа", 1, "а"},
		{"wide character in one column", "坂本", 1, ""},
		{"no room", "Группа", 0, ""},
		{"negative room", "Группа", -4, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CutLeft(tt.input, tt.width); got != tt.want {
				t.Errorf("CutLeft(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

// widthCases are names of every script and symbol the views show, each
// narrower than 18 columns by the measure of lipgloss (issue #317).
var widthCases = []string{
	"Kino",
	"Кино",
	"Группа крови",
	"Ελληνικά",
	"Café Müller",
	"─│",
	"日本語",
	"bob❤️",
	"fr🇫🇷",
}

// Pad and TruncateAndPad fill a text to exactly the width asked, and Truncate
// and TruncateEllipsis leave a text that fits whole, by the measure of
// lipgloss, which lays out the frame, whatever the script (issue #317).
func TestTextHelpers_MeasureAsLipgloss(t *testing.T) {
	for _, s := range widthCases {
		t.Run(s, func(t *testing.T) {
			if got := lipgloss.Width(Pad(s, 18)); got != 18 {
				t.Errorf("Pad(%q, 18) is %d columns wide, want 18", s, got)
			}
			if got := lipgloss.Width(TruncateAndPad(s, 20)); got != 20 {
				t.Errorf("TruncateAndPad(%q, 20) is %d columns wide, want 20", s, got)
			}
			if got := lipgloss.Width(TruncateAndPadEllipsis(s, 20)); got != 20 {
				t.Errorf("TruncateAndPadEllipsis(%q, 20) is %d columns wide, want 20", s, got)
			}
			if got := Truncate(s, 20); got != s {
				t.Errorf("Truncate(%q, 20) = %q, want it unchanged", s, got)
			}
			if got := TruncateEllipsis(s, 20); got != s {
				t.Errorf("TruncateEllipsis(%q, 20) = %q, want it unchanged", s, got)
			}
		})
	}
}

// A text wider than the width is cut to exactly the width when its characters
// are one column wide, and to the width or one column less when a wide
// character straddles the cut (issue #317).
func TestTruncate_CutsToTheWidthByTheMeasureOfLipgloss(t *testing.T) {
	cuts := []struct {
		name string
		cut  func(string, int) string
		tail string
	}{
		{"Truncate", Truncate, "..."},
		{"TruncateEllipsis", TruncateEllipsis, "…"},
	}
	for _, c := range cuts {
		t.Run(c.name+" Cyrillic", func(t *testing.T) {
			got := c.cut("Группа крови на рукаве", 20)
			if w := lipgloss.Width(got); w != 20 {
				t.Errorf("%s is %d columns wide, want 20: %q", c.name, w, got)
			}
			if !utf8.ValidString(got) {
				t.Errorf("%s is not valid UTF-8: %q", c.name, got)
			}
			if !strings.HasSuffix(got, c.tail) {
				t.Errorf("%s = %q, want it to end with %q", c.name, got, c.tail)
			}
		})
		t.Run(c.name+" CJK", func(t *testing.T) {
			got := c.cut("坂本龍一の音楽と映画", 10)
			if w := lipgloss.Width(got); w != 10 && w != 9 {
				t.Errorf("%s is %d columns wide, want 9 or 10: %q", c.name, w, got)
			}
			if !strings.HasSuffix(got, c.tail) {
				t.Errorf("%s = %q, want it to end with %q", c.name, got, c.tail)
			}
		})
	}
}

// sgr matches a whole ANSI colour code.
var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// An ANSI escape code counts for no column and is never cut (issue #317).
func TestTextHelpers_KeepEscapeCodesWhole(t *testing.T) {
	styled := "\x1b[31mabc\x1b[0m"
	if got := lipgloss.Width(Pad(styled, 10)); got != 10 {
		t.Errorf("Pad(%q, 10) is %d columns wide, want 10", styled, got)
	}
	if got := lipgloss.Width(TruncateAndPad(styled, 10)); got != 10 {
		t.Errorf("TruncateAndPad(%q, 10) is %d columns wide, want 10", styled, got)
	}

	long := "\x1b[31mhello\x1b[0m \x1b[1mworld\x1b[0m"
	for _, cut := range []string{Truncate(long, 8), TruncateEllipsis(long, 8), TruncateAndPad(long, 8)} {
		if w := lipgloss.Width(cut); w != 8 {
			t.Errorf("%q is %d columns wide, want 8", cut, w)
		}
		if rest := sgr.ReplaceAllString(cut, ""); strings.Contains(rest, "\x1b") {
			t.Errorf("%q has an escape code cut in two", cut)
		}
		if !strings.HasPrefix(cut, "\x1b[31mhello") {
			t.Errorf("%q lost its first colour code", cut)
		}
	}
}

// Truncate still answers "..." when asked for fewer columns than the dots
// take, and a text that fits whole is returned unchanged (issue #317).
func TestTruncate_UnderThreeColumns(t *testing.T) {
	if got := Truncate("Группа", 2); got != "..." {
		t.Errorf(`Truncate("Группа", 2) = %q, want "..."`, got)
	}
	if got := Truncate("ab", 2); got != "ab" {
		t.Errorf(`Truncate("ab", 2) = %q, want "ab"`, got)
	}
	if got := TruncateEllipsis("Группа", 1); got != "…" {
		t.Errorf(`TruncateEllipsis("Группа", 1) = %q, want "…"`, got)
	}
	if got := TruncateEllipsis("Группа", 0); got != "" {
		t.Errorf(`TruncateEllipsis("Группа", 0) = %q, want ""`, got)
	}
}
