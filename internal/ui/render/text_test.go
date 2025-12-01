package render

import (
	"strings"
	"testing"
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
