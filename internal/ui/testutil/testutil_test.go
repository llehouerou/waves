package testutil

import (
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no ansi codes",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "with color codes",
			input: "\x1b[31mred\x1b[0m text",
			want:  "red text",
		},
		{
			name:  "with multiple codes",
			input: "\x1b[1;32mbold green\x1b[0m",
			want:  "bold green",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "multiple spaces",
			input: "hello    world",
			want:  "hello world",
		},
		{
			name:  "tabs and newlines",
			input: "hello\t\nworld",
			want:  "hello world",
		},
		{
			name:  "leading and trailing",
			input: "  hello  ",
			want:  "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestContainsLine(t *testing.T) {
	output := "line one\nline two\nline three"

	if !ContainsLine(output, "two") {
		t.Error("should find 'two' in output")
	}
	if ContainsLine(output, "four") {
		t.Error("should not find 'four' in output")
	}
}

func TestFindLine(t *testing.T) {
	output := "first line\nsecond line\nthird line"

	got := FindLine(output, "second")
	if got != "second line" {
		t.Errorf("FindLine() = %q, want %q", got, "second line")
	}

	got = FindLine(output, "missing")
	if got != "" {
		t.Errorf("FindLine() for missing = %q, want empty", got)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "three lines",
			input: "one\ntwo\nthree",
			want:  3,
		},
		{
			name:  "with empty lines",
			input: "one\n\nthree\n",
			want:  2,
		},
		{
			name:  "empty input",
			input: "",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountLines(tt.input)
			if got != tt.want {
				t.Errorf("CountLines(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	input := "one\ntwo\nthree\n\n"
	got := SplitLines(input)

	if len(got) != 3 {
		t.Errorf("SplitLines() returned %d lines, want 3", len(got))
	}
	if got[0] != "one" || got[1] != "two" || got[2] != "three" {
		t.Errorf("SplitLines() = %v, want [one two three]", got)
	}
}

func TestAssertContains(t *testing.T) {
	output := "hello world"

	if msg := AssertContains(output, "world"); msg != "" {
		t.Errorf("AssertContains should pass: %s", msg)
	}

	if msg := AssertContains(output, "missing"); msg == "" {
		t.Error("AssertContains should fail for missing substring")
	}
}

func TestAssertNotContains(t *testing.T) {
	output := "hello world"

	if msg := AssertNotContains(output, "missing"); msg != "" {
		t.Errorf("AssertNotContains should pass: %s", msg)
	}

	if msg := AssertNotContains(output, "world"); msg == "" {
		t.Error("AssertNotContains should fail for present substring")
	}
}
